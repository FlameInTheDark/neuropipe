package variables

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func newService(t *testing.T) (*Service, *persistence.Store) {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := New(store)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service, store
}

func declare(t *testing.T, service *Service, name string, dataType domain.DataType, defaultValue any) domain.GlobalVariable {
	t.Helper()
	variable, err := service.Create(context.Background(), domain.GlobalVariable{
		Name:         name,
		Description:  "test variable",
		DataType:     dataType,
		DefaultValue: defaultValue,
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", name, err)
	}
	return variable
}

func TestReadReturnsDeclaredDefault(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "visits", domain.DataNumber, float64(5))
	value, err := service.Read("visits")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if value != float64(5) {
		t.Fatalf("Read() = %#v, want default", value)
	}
}

func TestReadUnknownVariableFails(t *testing.T) {
	service, _ := newService(t)
	if _, err := service.Read("missing"); err == nil {
		t.Fatal("Read() accepted an unknown variable")
	}
}

func TestSetValidatesDeclaredType(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "count", domain.DataNumber, float64(0))
	if err := service.Set("count", "not-a-number"); err == nil {
		t.Fatal("Set() accepted a text value for a number variable")
	}
	if err := service.Set("count", float64(42)); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, err := service.Read("count")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if value != float64(42) {
		t.Fatalf("Read() = %#v, want 42", value)
	}
}

func TestSetAcceptsIntForNumberVariable(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "count", domain.DataNumber, float64(0))
	if err := service.Set("count", 42); err != nil {
		t.Fatalf("Set(int) error = %v, want accepted (widen to float)", err)
	}
	value, err := service.Read("count")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if value != 42 {
		t.Fatalf("Read() = %#v (%T), want 42 (int)", value, value)
	}
}

func TestIncrementIsAtomicUnderConcurrentWriters(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "counter", domain.DataNumber, float64(0))
	const writers = 16
	const perWriter = 64
	var wait sync.WaitGroup
	for worker := 0; worker < writers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < perWriter; index++ {
				if _, err := service.Increment("counter", 1); err != nil {
					t.Errorf("Increment() error = %v", err)
					return
				}
			}
		}()
	}
	wait.Wait()
	value, err := service.Read("counter")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got, ok := value.(float64); !ok || got != float64(writers*perWriter) {
		t.Fatalf("Read() = %#v, want %d", value, writers*perWriter)
	}
}

func TestIncrementRejectsNonNumberVariable(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "label", domain.DataText, "hello")
	if _, err := service.Increment("label", 1); err == nil {
		t.Fatal("Increment() accepted a text variable")
	}
}

func TestAppendIsAtomicAcrossConcurrentWriters(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "items", domain.DataList, []any{})
	const writers = 8
	const perWriter = 32
	var wait sync.WaitGroup
	for worker := 0; worker < writers; worker++ {
		wait.Add(1)
		go func(offset int) {
			defer wait.Done()
			for index := 0; index < perWriter; index++ {
				if _, err := service.Append("items", offset+index); err != nil {
					t.Errorf("Append() error = %v", err)
					return
				}
			}
		}(worker * perWriter)
	}
	wait.Wait()
	value, err := service.Read("items")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	list, ok := value.([]any)
	if !ok || len(list) != writers*perWriter {
		t.Fatalf("Read() list length = %d, want %d", len(list), writers*perWriter)
	}
}

func TestAppendRejectsNonListVariable(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "text", domain.DataText, "hello")
	if _, err := service.Append("text", "x"); err == nil {
		t.Fatal("Append() accepted a text variable")
	}
}

func TestCreateRejectsInvalidNameAndDuplicate(t *testing.T) {
	service, _ := newService(t)
	if _, err := service.Create(context.Background(), domain.GlobalVariable{Name: "1bad", DataType: domain.DataText}); err == nil {
		t.Fatal("Create() accepted an invalid name")
	}
	declare(t, service, "count", domain.DataNumber, float64(0))
	if _, err := service.Create(context.Background(), domain.GlobalVariable{Name: "count", DataType: domain.DataNumber}); err == nil {
		t.Fatal("Create() accepted a duplicate name")
	}
}

func TestUpdateMetadataForbidsRenameAndTypeChange(t *testing.T) {
	service, _ := newService(t)
	variable := declare(t, service, "visits", domain.DataNumber, float64(0))
	variable.Description = "Updated"
	if _, err := service.Update(context.Background(), variable); err != nil {
		t.Fatalf("Update(description) error = %v", err)
	}
	variable.Name = "renamed"
	if _, err := service.Update(context.Background(), variable); err == nil {
		t.Fatal("Update() accepted a rename")
	}
	variable.Name = "visits"
	variable.DataType = domain.DataText
	if _, err := service.Update(context.Background(), variable); err == nil {
		t.Fatal("Update() accepted a type change")
	}
}

func TestDeleteBlocksReferencedVariable(t *testing.T) {
	service, store := newService(t)
	definition := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{{ID: "start", Type: "trigger:button"}, {ID: "read", Type: "data:get_global_variable", Data: map[string]any{"config": map[string]any{"name": "shared"}}}}}
	pipeline, err := store.CreatePipeline(context.Background(), "", "uses-variable", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	variable := declare(t, service, "shared", domain.DataText, "hello")
	if err := service.Delete(context.Background(), variable.ID); err == nil {
		t.Fatal("Delete() accepted a referenced variable")
	}
	if err := store.DeletePipeline(context.Background(), pipeline.ID); err != nil {
		t.Fatalf("DeletePipeline() error = %v", err)
	}
	if err := service.Delete(context.Background(), variable.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestValuesPeeredAcrossServiceRestarts(t *testing.T) {
	storeDir := t.TempDir()
	storeFirst, err := persistence.New(storeDir)
	if err != nil {
		t.Fatalf("New store first run error = %v", err)
	}
	first, err := New(storeFirst)
	if err != nil {
		t.Fatalf("New service first run error = %v", err)
	}
	declare(t, first, "count", domain.DataNumber, float64(0))
	if _, err := first.Increment("count", 7); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	first.Start(context.Background())
	// Persist the written value without depending on the ticker timing.
	if err := first.flush(context.Background()); err != nil {
		t.Fatalf("flush() error = %v", err)
	}
	first.Stop()
	_ = storeFirst.Close()

	storeSecond, err := persistence.New(storeDir)
	if err != nil {
		t.Fatalf("New store second run error = %v", err)
	}
	second, err := New(storeSecond)
	if err != nil {
		t.Fatalf("New service second run error = %v", err)
	}
	value, err := second.Read("count")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if value != float64(7) {
		t.Fatalf("Read() = %#v, want persisted 7", value)
	}
	_ = storeSecond.Close()
}

// TestConcurrentReadWriteSafety exercises the mutex-guarded read path against
// concurrent writes from other goroutines. A data race would trip the runtime
// under `go test -race`.
func TestConcurrentReadWriteSafety(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "shared", domain.DataText, "initial")
	var stop atomic.Bool
	var reads atomic.Int64
	var readersReady sync.WaitGroup
	var readers sync.WaitGroup
	readersReady.Add(4)
	for reader := 0; reader < 4; reader++ {
		readers.Add(1)
		go func(id int) {
			defer readers.Done()
			if _, err := service.Read("shared"); err != nil {
				t.Errorf("Read() error = %v", err)
				readersReady.Done()
				return
			}
			reads.Add(1)
			readersReady.Done()
			for !stop.Load() {
				if _, err := service.Read("shared"); err != nil {
					t.Errorf("Read() error = %v", err)
					return
				}
				reads.Add(1)
			}
		}(reader)
	}
	readersReady.Wait()
	var writers sync.WaitGroup
	for writer := 0; writer < 2; writer++ {
		writers.Add(1)
		go func(id int) {
			defer writers.Done()
			for index := 0; index < 200; index++ {
				if err := service.Set("shared", fmt.Sprintf("value-%d", index)); err != nil {
					t.Errorf("Set() error = %v", err)
					return
				}
			}
		}(writer)
	}
	writers.Wait()
	stop.Store(true)
	readers.Wait()
	if reads.Load() == 0 {
		t.Fatal("readers never observed a value")
	}
}

// TestVariableOptions exposes the resolver list used by the catalog picklist.
// Order must be stable so editor config reproducibly shows the same list.
func TestVariableOptions(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "alpha", domain.DataText, "")
	declare(t, service, "beta", domain.DataNumber, float64(0))
	options := service.VariableOptions()
	if len(options) != 2 {
		t.Fatalf("VariableOptions() len = %d, want 2", len(options))
	}
	if options[0].Value != "alpha" || options[1].Value != "beta" {
		t.Fatalf("VariableOptions() order = %#v", options)
	}
	if options[0].Label != "alpha (text)" || options[1].Label != "beta (number)" {
		t.Fatalf("VariableOptions() labels = %#v", options)
	}
}

// TestVariableType powers the Get Global Variable resolver: the declared type
// rewrites the output pin even before any write has occurred.
func TestVariableType(t *testing.T) {
	service, _ := newService(t)
	declare(t, service, "counter", domain.DataNumber, float64(0))
	declared, ok := service.VariableType("counter")
	if !ok || declared != domain.DataNumber {
		t.Fatalf("VariableType() = %v, %v", declared, ok)
	}
	if _, ok := service.VariableType("missing"); ok {
		t.Fatal("VariableType() found an unknown variable")
	}
}
