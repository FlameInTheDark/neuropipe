package databases

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// postgresDialect speaks PostgreSQL via the pgx/v5 stdlib driver.
type postgresDialect struct{}

func (postgresDialect) Name() domain.DatabaseDriver { return domain.DatabaseDriverPostgres }

// Open builds a libpq-style URL and returns a *sql.DB. Both username and
// password are URL-escaped so special characters survive the round-trip.
func (postgresDialect) Open(item domain.Database, secret string) (*sql.DB, error) {
	sslMode := strings.TrimSpace(item.SSLMode)
	if sslMode == "" {
		sslMode = "prefer"
	}
	schema := strings.TrimSpace(item.Schema)
	if schema == "" {
		schema = "public"
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&search_path=%s",
		url.QueryEscape(strings.TrimSpace(item.Username)),
		url.QueryEscape(secret),
		strings.TrimSpace(item.Host),
		item.Port,
		url.QueryEscape(strings.TrimSpace(item.Database)),
		url.QueryEscape(sslMode),
		url.QueryEscape(schema),
	)
	return sql.Open("pgx", dsn)
}

func (postgresDialect) PingQuery() string { return "SELECT 1" }

func (postgresDialect) PoolDefaults() (int, int, time.Duration) { return 10, 2, 30 * time.Minute }

// InspectTables lists base tables in the configured schema (default "public").
func (postgresDialect) InspectTables(ctx context.Context, db *sql.DB, item domain.Database) ([]string, error) {
	schema := strings.TrimSpace(item.Schema)
	if schema == "" {
		schema = "public"
	}
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`, schema)
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

// InspectTable returns the column and index catalog for one Postgres table.
// Primary keys are resolved through pg_index/pg_attribute; index list and
// definitions come from pg_indexes.
func (postgresDialect) InspectTable(ctx context.Context, db *sql.DB, item domain.Database, name string) (domain.DatabaseTable, error) {
	schema := strings.TrimSpace(item.Schema)
	if schema == "" {
		schema = "public"
	}
	table := domain.DatabaseTable{Name: name, Columns: []domain.DatabaseColumn{}, Indexes: []domain.DatabaseIndex{}}

	columns, err := db.QueryContext(ctx, `SELECT column_name, data_type, is_nullable, column_default, ordinal_position FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, schema, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}
	for columns.Next() {
		var column domain.DatabaseColumn
		var isNullable, columnDefault sql.NullString
		var ordinal int
		if err := columns.Scan(&column.Name, &column.DataType, &isNullable, &columnDefault, &ordinal); err != nil {
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

	primaryKeyRows, err := db.QueryContext(ctx, `
SELECT a.attname
FROM pg_index i
JOIN pg_class c ON c.oid = i.indrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
WHERE n.nspname = $1 AND c.relname = $2 AND i.indisprimary
ORDER BY array_position(i.indkey, a.attnum)`, schema, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
	}
	primaryKeys := map[string]bool{}
	for primaryKeyRows.Next() {
		var pk string
		if err := primaryKeyRows.Scan(&pk); err != nil {
			_ = primaryKeyRows.Close()
			return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
		}
		primaryKeys[pk] = true
	}
	if err := primaryKeyRows.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q primary key: %w", name, err)
	}
	for index := range table.Columns {
		if primaryKeys[table.Columns[index].Name] {
			table.Columns[index].PrimaryKey = true
		}
	}

	indexes, err := db.QueryContext(ctx, `SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = $1 AND tablename = $2 ORDER BY indexname`, schema, name)
	if err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	for indexes.Next() {
		var index domain.DatabaseIndex
		var indexdef string
		if err := indexes.Scan(&index.Name, &indexdef); err != nil {
			_ = indexes.Close()
			return table, fmt.Errorf("inspect table %q index: %w", name, err)
		}
		index.Unique = strings.HasPrefix(indexdef, "CREATE UNIQUE INDEX")
		index.Columns = parsePostgresIndexColumns(indexdef)
		table.Indexes = append(table.Indexes, index)
	}
	if err := indexes.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	return table, nil
}

// parsePostgresIndexColumns extracts the column list from the parenthesised
// tail of a CREATE INDEX statement. Function-based indexes are returned as-is
// (e.g. "lower(name)") — they are display-only.
func parsePostgresIndexColumns(indexdef string) []string {
	open := strings.LastIndex(indexdef, "(")
	close := strings.LastIndex(indexdef, ")")
	if open < 0 || close < 0 || close <= open {
		return nil
	}
	body := indexdef[open+1 : close]
	parts := strings.Split(body, ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name != "" {
			columns = append(columns, name)
		}
	}
	return columns
}
