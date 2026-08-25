package databases

import (
	"context"
	"database/sql"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// dialect abstracts SQL-dialect-specific operations.
type dialect interface {
	// Name returns the driver identifier persisted on the database row.
	Name() domain.DatabaseDriver
	// Open returns a *sql.DB bound to the driver-specific connection string.
	// secret is the decrypted password (or empty string) sourced from the vault.
	Open(db domain.Database, secret string) (*sql.DB, error)
	// PingQuery returns a single-row, single-column statement used to verify
	// that the database is reachable and the credentials work.
	PingQuery() string
	// PoolDefaults returns recommended connection-pool sizing for the dialect.
	PoolDefaults() (maxOpen, maxIdle int, maxLifetime time.Duration)
	// InspectTables returns the user-visible table names for the database.
	InspectTables(ctx context.Context, db *sql.DB, item domain.Database) ([]string, error)
	// InspectTable returns the column and index catalog for one table.
	InspectTable(ctx context.Context, db *sql.DB, item domain.Database, name string) (domain.DatabaseTable, error)
}

// dialectFor returns the dialect implementation for the given driver, falling
// back to SQLite when the value is empty or unrecognised.
func dialectFor(driver domain.DatabaseDriver) dialect {
	switch driver {
	case domain.DatabaseDriverPostgres:
		return &postgresDialect{}
	case domain.DatabaseDriverMySQL:
		return &mysqlDialect{}
	case domain.DatabaseDriverDuckDB:
		return &duckdbDialect{}
	default:
		return &sqliteDialect{}
	}
}
