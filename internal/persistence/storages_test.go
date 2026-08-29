package persistence

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestStoragePublicBaseURLRoundTrip(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	created, err := store.CreateStorage(context.Background(), domain.Storage{
		Name: "Mirror", Driver: domain.StorageDriverS3, Bucket: "b", PublicBaseURL: "https://cdn.example.com/assets",
	})
	if err != nil {
		t.Fatalf("CreateStorage() error = %v", err)
	}
	loaded, err := store.GetStorage(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetStorage() error = %v", err)
	}
	if loaded.PublicBaseURL != "https://cdn.example.com/assets" {
		t.Fatalf("PublicBaseURL = %q, want the stored base", loaded.PublicBaseURL)
	}

	loaded.PublicBaseURL = ""
	updated, err := store.UpdateStorage(context.Background(), loaded)
	if err != nil {
		t.Fatalf("UpdateStorage() error = %v", err)
	}
	if updated.PublicBaseURL != "" {
		t.Fatalf("PublicBaseURL after clear = %q", updated.PublicBaseURL)
	}

	// Pre-existing storages survive a reopen without a base (empty default).
	reopened, err := store.ListStorages(context.Background())
	if err != nil || len(reopened) != 1 {
		t.Fatalf("ListStorages() = %d items, %v", len(reopened), err)
	}
}

func TestStoragePublicBaseURLColumnMigratesExistingWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	database, err := sql.Open("sqlite3", filepath.Join(root, "neuropipe.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS storages (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver TEXT NOT NULL DEFAULT 's3',
  endpoint TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  bucket TEXT NOT NULL DEFAULT '',
  access_key TEXT NOT NULL DEFAULT '',
  secret_ref TEXT NOT NULL DEFAULT '',
  secure INTEGER NOT NULL DEFAULT 1,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  password_ref TEXT NOT NULL DEFAULT '',
  tls_mode TEXT NOT NULL DEFAULT 'none',
  base_dir TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unknown',
  last_ping_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`); err != nil {
		t.Fatalf("create legacy storages table: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO storages (id, name, driver, bucket, created_at, updated_at) VALUES ('legacy', 'Legacy bucket', 's3', 'old', '2026-01-01', '2026-01-01')`); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	var columns int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('storages') WHERE name = 'public_base_url'`).Scan(&columns); err != nil {
		t.Fatalf("inspect storages columns: %v", err)
	}
	if columns != 1 {
		t.Fatalf("public_base_url columns = %d, want 1", columns)
	}
	legacy, err := store.GetStorage(context.Background(), "legacy")
	if err != nil {
		t.Fatalf("GetStorage(legacy) error = %v", err)
	}
	if legacy.PublicBaseURL != "" {
		t.Fatalf("legacy PublicBaseURL = %q, want empty default", legacy.PublicBaseURL)
	}
}
