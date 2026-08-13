package persistence

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestDatabaseMetadataCRUD(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data.sqlite")
	created, err := store.CreateDatabase(ctx, domain.Database{Name: "Main", Path: path})
	if err != nil {
		t.Fatalf("CreateDatabase() error = %v", err)
	}
	if created.ID == "" || created.Name != "Main" || created.Path != path {
		t.Fatalf("CreateDatabase() = %#v", created)
	}
	items, err := store.ListDatabases(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListDatabases() = %#v, %v", items, err)
	}
	created.Name = "Primary"
	updated, err := store.UpdateDatabase(ctx, created)
	if err != nil || updated.Name != "Primary" {
		t.Fatalf("UpdateDatabase() = %#v, %v", updated, err)
	}
	if err := store.DeleteDatabase(ctx, created.ID); err != nil {
		t.Fatalf("DeleteDatabase() error = %v", err)
	}
	if items, err := store.ListDatabases(ctx); err != nil || len(items) != 0 {
		t.Fatalf("ListDatabases() after delete = %#v, %v", items, err)
	}
}
