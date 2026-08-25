//go:build duckdb

package databases

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	_ "github.com/duckdb/duckdb-go/v2" // registers the "duckdb" database/sql driver
)

// duckdbDialect speaks DuckDB through the go-duckdb driver. Like SQLite it is
// file-backed (the database path points at a .duckdb/.ddb file), so no
// credentials are involved.
type duckdbDialect struct{}

func (duckdbDialect) Name() domain.DatabaseDriver { return domain.DatabaseDriverDuckDB }

// Open binds a *sql.DB to the DuckDB file. The driver creates the file on
// first open when it does not exist yet.
func (duckdbDialect) Open(item domain.Database, _ string) (*sql.DB, error) {
	return sql.Open("duckdb", item.Path)
}

func (duckdbDialect) PingQuery() string { return "SELECT version()" }

// DuckDB allows either many readers or one writer per process; keep the pool
// conservative and connections long-lived like the SQLite dialect.
func (duckdbDialect) PoolDefaults() (int, int, time.Duration) { return 4, 1, 0 }

// InspectTables lists base tables in the default "main" schema.
func (duckdbDialect) InspectTables(ctx context.Context, db *sql.DB, _ domain.Database) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'main' AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("inspect database tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("inspect database table: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// InspectTable returns columns (information_schema), primary-key membership
// (table_constraints/key_column_usage) and indexes (duckdb_indexes).
func (duckdbDialect) InspectTable(ctx context.Context, db *sql.DB, _ domain.Database, name string) (domain.DatabaseTable, error) {
	table := domain.DatabaseTable{Name: name, Columns: []domain.DatabaseColumn{}, Indexes: []domain.DatabaseIndex{}}

	columns, err := db.QueryContext(ctx, `SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = 'main' AND table_name = ? ORDER BY ordinal_position`, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}
	for columns.Next() {
		var column domain.DatabaseColumn
		var isNullable, columnDefault sql.NullString
		if err := columns.Scan(&column.Name, &column.DataType, &isNullable, &columnDefault); err != nil {
			_ = columns.Close()
			return table, fmt.Errorf("inspect table %q column: %w", name, err)
		}
		column.Nullable = strings.EqualFold(isNullable.String, "YES")
		if columnDefault.Valid {
			value := columnDefault.String
			column.Default = &value
		}
		table.Columns = append(table.Columns, column)
	}
	if err := columns.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}

	primaryKeys, err := db.QueryContext(ctx, `
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = 'main' AND tc.table_name = ? AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
	}
	primaryKeySeen := map[string]bool{}
	for primaryKeys.Next() {
		var pk string
		if err := primaryKeys.Scan(&pk); err != nil {
			_ = primaryKeys.Close()
			return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
		}
		primaryKeySeen[pk] = true
	}
	if err := primaryKeys.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
	}
	for index := range table.Columns {
		if primaryKeySeen[table.Columns[index].Name] {
			table.Columns[index].PrimaryKey = true
		}
	}

	indexes, err := db.QueryContext(ctx, `SELECT index_name, is_unique, expressions FROM duckdb_indexes() WHERE schema_name = 'main' AND table_name = ? ORDER BY index_name`, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	for indexes.Next() {
		var index domain.DatabaseIndex
		var unique int
		var expressions string
		if err := indexes.Scan(&index.Name, &unique, &expressions); err != nil {
			_ = indexes.Close()
			return table, fmt.Errorf("inspect index %q: %w", name, err)
		}
		index.Unique = unique != 0
		index.Columns = parsePostgresIndexColumns(expressions)
		table.Indexes = append(table.Indexes, index)
	}
	if err := indexes.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	return table, nil
}
