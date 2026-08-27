package security

import (
	"fmt"
	"sync"
	"testing"
)

func TestVaultRoundTrip(t *testing.T) {
	vault, err := NewVault(t.TempDir())
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if _, err := vault.Get("telegram:bot:token"); err == nil {
		t.Fatal("expected Get on an empty vault to fail")
	}
	if err := vault.Put("telegram:bot:token", "secret-token"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	value, err := vault.Get("telegram:bot:token")
	if err != nil || value != "secret-token" {
		t.Fatalf("Get() = %q, %v", value, err)
	}
	metadata, err := vault.List()
	if err != nil || len(metadata) != 1 || metadata[0].Name != "telegram:bot:token" {
		t.Fatalf("List() = %#v, %v", metadata, err)
	}
	if err := vault.Delete("telegram:bot:token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := vault.Get("telegram:bot:token"); err == nil {
		t.Fatal("expected Get after Delete to fail")
	}
}

// TestVaultConcurrentOperations pins the concurrency contract the chat
// services rely on: their validation-loop goroutines read tokens while
// request goroutines save or remove secrets. Every method must be safe for
// concurrent use, and the read-modify-write cycles of Put and Delete must
// never lose one another's updates.
func TestVaultConcurrentOperations(t *testing.T) {
	vault, err := NewVault(t.TempDir())
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	const writers = 8
	const iterations = 40

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			key := fmt.Sprintf("telegram:bot-%d:token", w)
			for i := 0; i < iterations; i++ {
				if err := vault.Put(key, fmt.Sprintf("value-%d-%d", w, i)); err != nil {
					t.Errorf("writer %d: Put() error = %v", w, err)
					return
				}
			}
		}(w)
	}
	// A concurrent deleter racing the writers on its own key.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if err := vault.Put("doomed:key", "x"); err != nil {
				t.Errorf("deleter: Put() error = %v", err)
				return
			}
			if err := vault.Delete("doomed:key"); err != nil {
				t.Errorf("deleter: Delete() error = %v", err)
				return
			}
		}
	}()
	// Concurrent readers exercising Get and List.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_, _ = vault.Get(fmt.Sprintf("telegram:bot-%d:token", i%writers))
			if _, err := vault.List(); err != nil {
				t.Errorf("reader: List() error = %v", err)
				return
			}
		}
	}()
	wg.Wait()

	// Lost updates surface here: every writer's key must have survived with
	// its final value and the deleted key must stay gone.
	for w := 0; w < writers; w++ {
		key := fmt.Sprintf("telegram:bot-%d:token", w)
		value, err := vault.Get(key)
		want := fmt.Sprintf("value-%d-%d", w, iterations-1)
		if err != nil || value != want {
			t.Fatalf("key %q = %q, %v; want %q (a concurrent update was lost)", key, value, err, want)
		}
	}
	if _, err := vault.Get("doomed:key"); err == nil {
		t.Fatal("deleted key resurrected after concurrent writes")
	}
}
