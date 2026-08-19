package databases

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// sqliteDialect mirrors the original SQLite-only behaviour of the service.
// It is the default dialect for rows that have no driver recorded.
type sqliteDialect struct{}

func (sqliteDialect) Name() domain.DatabaseDriver { return domain.DatabaseDriverSQLite }

// Open returns a *sql.DB bound to the SQLite file with foreign keys and a
// 5-second busy timeout. The path is the absolute file location.
func (sqliteDialect) Open(item domain.Database, _ string) (*sql.DB, error) {
	return sql.Open("sqlite3", "file:"+filepath.ToSlash(item.Path)+"?_foreign_keys=on&_busy_timeout=5000")
}

func (sqliteDialect) PingQuery() string { return `SELECT sqlite_version()` }

func (sqliteDialect) PoolDefaults() (int, int, time.Duration) { return 4, 1, 0 }

// InspectTables enumerates user-created tables, excluding SQLite's internal
// sqlite_* schema objects.
func (sqliteDialect) InspectTables(ctx context.Context, db *sql.DB, _ domain.Database) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name COLLATE NOCASE`)
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

// InspectTable returns the columns and indexes for one SQLite table via the
// pragma_table_info, pragma_index_list and pragma_index_info table-valued
// functions.
func (sqliteDialect) InspectTable(ctx context.Context, db *sql.DB, _ domain.Database, name string) (domain.DatabaseTable, error) {
	table := domain.DatabaseTable{Name: name, Columns: []domain.DatabaseColumn{}, Indexes: []domain.DatabaseIndex{}}
	rows, err := db.QueryContext(ctx, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}
	for rows.Next() {
		var column domain.DatabaseColumn
		var notNull, primary int
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.DataType, &notNull, &defaultValue, &primary); err != nil {
			_ = rows.Close()
			return table, fmt.Errorf("inspect table %q column: %w", name, err)
		}
		column.Nullable, column.PrimaryKey = notNull == 0, primary > 0
		if defaultValue.Valid {
			column.Default = &defaultValue.String
		}
		table.Columns = append(table.Columns, column)
	}
	if err := rows.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}
	indexes, err := db.QueryContext(ctx, `SELECT name, "unique" FROM pragma_index_list(?) ORDER BY name`, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	for indexes.Next() {
		var index domain.DatabaseIndex
		var unique int
		if err := indexes.Scan(&index.Name, &unique); err != nil {
			_ = indexes.Close()
			return table, fmt.Errorf("inspect table %q index: %w", name, err)
		}
		index.Unique = unique != 0
		columns, err := db.QueryContext(ctx, `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index.Name)
		if err != nil {
			_ = indexes.Close()
			return table, fmt.Errorf("inspect index %q: %w", index.Name, err)
		}
		for columns.Next() {
			var column string
			if err := columns.Scan(&column); err != nil {
				_ = columns.Close()
				_ = indexes.Close()
				return table, fmt.Errorf("inspect index %q column: %w", index.Name, err)
			}
			index.Columns = append(index.Columns, column)
		}
		_ = columns.Close()
		table.Indexes = append(table.Indexes, index)
	}
	if err := indexes.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	return table, nil
}
