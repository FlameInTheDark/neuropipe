package databases

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	_ "github.com/go-sql-driver/mysql" // registers the "mysql" database/sql driver
)

// mysqlDialect speaks MySQL via the go-sql-driver/mysql driver.
type mysqlDialect struct{}

func (mysqlDialect) Name() domain.DatabaseDriver { return domain.DatabaseDriverMySQL }

// Open builds a MySQL DSN of the form
// "user:pass@tcp(host:port)/dbname?charset=utf8mb4&parseTime=true&loc=UTC&timeout=5s".
func (mysqlDialect) Open(item domain.Database, secret string) (*sql.DB, error) {
	charset := strings.TrimSpace(item.Charset)
	if charset == "" {
		charset = "utf8mb4"
	}
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=UTC&timeout=5s",
		strings.TrimSpace(item.Username),
		secret,
		strings.TrimSpace(item.Host),
		item.Port,
		strings.TrimSpace(item.Database),
		charset,
	)
	return sql.Open("mysql", dsn)
}

func (mysqlDialect) PingQuery() string { return "SELECT 1" }

func (mysqlDialect) PoolDefaults() (int, int, time.Duration) { return 10, 2, 15 * time.Minute }

// InspectTables lists user tables in the connected database via SHOW TABLES.
// The result column is named "Tables_in_<dbname>" so we scan by position.
func (mysqlDialect) InspectTables(ctx context.Context, db *sql.DB, _ domain.Database) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW TABLES")
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

// InspectTable returns the column and index catalog for one MySQL table.
// SHOW FULL COLUMNS and SHOW INDEX do not accept placeholders for the table
// name, so the identifier is backtick-quoted with embedded backticks doubled.
func (mysqlDialect) InspectTable(ctx context.Context, db *sql.DB, _ domain.Database, name string) (domain.DatabaseTable, error) {
	table := domain.DatabaseTable{Name: name, Columns: []domain.DatabaseColumn{}, Indexes: []domain.DatabaseIndex{}}
	qualified := quoteMySQLIdentifier(name)

	columns, err := db.QueryContext(ctx, "SHOW FULL COLUMNS FROM "+qualified)
	if err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}
	for columns.Next() {
		var column domain.DatabaseColumn
		var null, key sql.NullString
		var defaultValue sql.NullString
		var collation, extra, privileges, comment sql.NullString
		if err := columns.Scan(&column.Name, &column.DataType, &collation, &null, &key, &defaultValue, &extra, &privileges, &comment); err != nil {
			_ = columns.Close()
			return table, fmt.Errorf("inspect table %q column: %w", name, err)
		}
		column.Nullable = strings.EqualFold(null.String, "YES")
		column.PrimaryKey = strings.EqualFold(key.String, "PRI")
		if defaultValue.Valid {
			value := defaultValue.String
			column.Default = &value
		}
		table.Columns = append(table.Columns, column)
	}
	if err := columns.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q columns: %w", name, err)
	}

	indexRows, err := db.QueryContext(ctx, "SHOW INDEX FROM "+qualified)
	if err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	// Preserve first-seen order; within an index, columns are ordered by Seq_in_index.
	ordered := make([]string, 0)
	grouped := make(map[string]*domain.DatabaseIndex)
	for indexRows.Next() {
		var tableField, keyName, columnName, collation, indexType, comment, indexComment sql.NullString
		var nonUnique int
		var seqInIndex int64
		var cardinality, subPart, packed sql.NullInt64
		var nullCol sql.NullString
		if err := indexRows.Scan(&tableField, &nonUnique, &keyName, &seqInIndex, &columnName, &collation, &cardinality, &subPart, &packed, &nullCol, &indexType, &comment, &indexComment); err != nil {
			_ = indexRows.Close()
			return table, fmt.Errorf("inspect table %q index: %w", name, err)
		}
		if keyName.String == "" {
			continue
		}
		index, exists := grouped[keyName.String]
		if !exists {
			index = &domain.DatabaseIndex{Name: keyName.String, Unique: nonUnique == 0, Columns: []string{}}
			grouped[keyName.String] = index
			ordered = append(ordered, keyName.String)
		}
		if columnName.String != "" {
			index.Columns = append(index.Columns, columnName.String)
		}
	}
	if err := indexRows.Close(); err != nil {
		return table, fmt.Errorf("inspect table %q indexes: %w", name, err)
	}
	for _, keyName := range ordered {
		table.Indexes = append(table.Indexes, *grouped[keyName])
	}
	return table, nil
}

// quoteMySQLIdentifier wraps a name in backticks and escapes any embedded
// backtick by doubling it, per MySQL identifier-quoting rules.
func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
