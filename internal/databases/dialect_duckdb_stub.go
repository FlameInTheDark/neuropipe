//go:build !duckdb

package databases

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// duckdbDialect placeholder for builds without the "duckdb" tag. DuckDB's
// Windows static libraries require an MSVC-compatible link step, so support
// is opt-in: rebuild Neuropipe with "-tags duckdb" to enable it.
type duckdbDialect struct{}

func (duckdbDialect) Name() domain.DatabaseDriver { return domain.DatabaseDriverDuckDB }

var errDuckDBNotCompiled = fmt.Errorf("DuckDB support is not compiled into this build (rebuild with -tags duckdb)")

func (duckdbDialect) Open(domain.Database, string) (*sql.DB, error) {
	return nil, errDuckDBNotCompiled
}

func (duckdbDialect) PingQuery() string                       { return "SELECT 1" }
func (duckdbDialect) PoolDefaults() (int, int, time.Duration) { return 1, 1, 0 }

func (duckdbDialect) InspectTables(context.Context, *sql.DB, domain.Database) ([]string, error) {
	return nil, errDuckDBNotCompiled
}

func (duckdbDialect) InspectTable(context.Context, *sql.DB, domain.Database, string) (domain.DatabaseTable, error) {
	return domain.DatabaseTable{}, errDuckDBNotCompiled
}
