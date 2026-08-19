package databases

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

func TestServiceExecuteInspectAndDelete(t *testing.T) {
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	service := New(store, nil)
	defer func() { _ = service.Close() }()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "people.sqlite")
	registered, err := service.Create(ctx, domain.SaveDatabaseRequest{Name: "People", Path: path})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.ExecuteSQL(ctx, domain.SQLRequest{DatabaseID: registered.ID, SQL: `CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`}); err != nil {
		t.Fatalf("create table error = %v", err)
	}
	for _, name := range []string{"Ada", "Grace", "Linus"} {
		result, err := service.ExecuteSQL(ctx, domain.SQLRequest{DatabaseID: registered.ID, SQL: `INSERT INTO people(name) VALUES (:name)`, Parameters: []domain.SQLArgument{{Name: "name", Value: name}}})
		if err != nil || result.RowsAffected != 1 || result.LastInsertID == nil {
			t.Fatalf("insert result = %#v, %v", result, err)
		}
	}
	result, err := service.ExecuteSQL(ctx, domain.SQLRequest{DatabaseID: registered.ID, SQL: `SELECT id, name, NULL AS optional FROM people WHERE id >= :minimum ORDER BY id`, Parameters: []domain.SQLArgument{{Name: "minimum", Value: 1}}, MaxRows: 2})
	if err != nil {
		t.Fatalf("query error = %v", err)
	}
	if len(result.Rows) != 2 || !result.Truncated || result.Rows[0]["name"] != "Ada" || result.Rows[0]["optional"] != nil {
		t.Fatalf("query result = %#v", result)
	}
	schema, err := service.Inspect(ctx, registered.ID)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "people" || len(schema.Tables[0].Columns) != 2 || !schema.Tables[0].Columns[0].PrimaryKey {
		t.Fatalf("Inspect() = %#v", schema)
	}
	if _, err := service.ExecuteSQL(ctx, domain.SQLRequest{DatabaseID: registered.ID, SQL: `SELECT ?`}); err == nil {
		t.Fatal("ExecuteSQL() accepted a positional placeholder")
	}
	if err := service.Delete(ctx, registered.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Delete() removed database file: %v", err)
	}
}

func TestServiceDebugRunsMultipleStatementsAtomically(t *testing.T) {
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	service := New(store, nil)
	defer func() { _ = service.Close() }()
	registered, err := service.Create(context.Background(), domain.SaveDatabaseRequest{Name: "News", Path: filepath.Join(t.TempDir(), "news.sqlite")})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	result, err := service.Debug(context.Background(), domain.SQLDebugRequest{DatabaseID: registered.ID, SQL: `
CREATE TABLE news (id INTEGER PRIMARY KEY, title TEXT NOT NULL);
CREATE INDEX idx_news_title ON news(title);
SELECT name FROM sqlite_schema WHERE type = 'index' AND tbl_name = 'news';`})
	if err != nil {
		t.Fatalf("Debug() error = %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0]["name"] != "idx_news_title" {
		t.Fatalf("Debug() = %#v", result)
	}
	if _, err := service.Debug(context.Background(), domain.SQLDebugRequest{DatabaseID: registered.ID, SQL: `CREATE TABLE rollback_test (id INTEGER); INVALID SQL`}); err == nil {
		t.Fatal("Debug() accepted an invalid script")
	}
	schema, err := service.Inspect(context.Background(), registered.ID)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	for _, table := range schema.Tables {
		if table.Name == "rollback_test" {
			t.Fatal("Debug() committed statements before an invalid statement")
		}
	}
}
