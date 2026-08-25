// Package databases manages registered SQL databases and their connections.
// It supports SQLite (local files), PostgreSQL (via pgx) and MySQL (via
// go-sql-driver/mysql). Driver-specific behaviour lives in the dialect files.
package databases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultMaxRows  = 500
	absoluteMaxRows = 10_000
	pingTimeout     = 5 * time.Second
)

var parameterName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Service struct {
	store  *persistence.Store
	vault  *security.Vault
	mu     sync.Mutex
	dbs    map[string]*sql.DB
	closed bool
}

// New creates a database service. vault may be nil for SQLite-only deployments,
// but Postgres/MySQL connections require it to resolve password references.
func New(store *persistence.Store, vault *security.Vault) *Service {
	return &Service{store: store, vault: vault, dbs: make(map[string]*sql.DB)}
}

func (s *Service) List(ctx context.Context) ([]domain.Database, error) {
	return s.store.ListDatabases(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Database, error) {
	return s.store.GetDatabase(ctx, strings.TrimSpace(id))
}

// BuildDatabase validates a SaveDatabaseRequest and returns a populated
// domain.Database without persisting it. Used by TestConnection so the
// "Test connection" button uses the same defaults as a real save.
func (s *Service) BuildDatabase(request domain.SaveDatabaseRequest) (domain.Database, error) {
	return validateMetadata(request)
}

// Create registers a new database. For SQLite the file is created on disk;
// for Postgres and MySQL the metadata is persisted and a connection is
// opened and pinged.
func (s *Service) Create(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := validateMetadata(request)
	if err != nil {
		return domain.Database{}, err
	}
	if item.Driver == domain.DatabaseDriverSQLite || item.Driver == domain.DatabaseDriverDuckDB {
		return s.createSQLite(ctx, item)
	}
	return s.createNetwork(ctx, item, request.Password)
}

func (s *Service) createSQLite(ctx context.Context, item domain.Database) (domain.Database, error) {
	if err := os.MkdirAll(filepath.Dir(item.Path), 0o700); err != nil {
		return domain.Database{}, fmt.Errorf("create database directory: %w", err)
	}
	file, err := os.OpenFile(item.Path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return domain.Database{}, fmt.Errorf("create database file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(item.Path)
		return domain.Database{}, fmt.Errorf("close database file: %w", err)
	}
	created, err := s.store.CreateDatabase(ctx, item)
	if err != nil {
		_ = os.Remove(item.Path)
		return domain.Database{}, err
	}
	if _, err := s.connection(ctx, created.ID); err != nil {
		_ = s.store.DeleteDatabase(context.Background(), created.ID)
		_ = os.Remove(item.Path)
		return domain.Database{}, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, created.ID, domain.DatabaseStatusConnected); err != nil {
		return domain.Database{}, err
	}
	return s.store.GetDatabase(ctx, created.ID)
}

// createNetwork stores a Postgres or MySQL database row. If a password is
// supplied it is written to the vault under a stable ref; otherwise the
// existing ref is reused.
func (s *Service) createNetwork(ctx context.Context, item domain.Database, password string) (domain.Database, error) {
	if err := s.applyPassword(ctx, &item, password); err != nil {
		return domain.Database{}, err
	}
	status, _ := s.TestConnection(ctx, item, password)
	created, err := s.store.CreateDatabase(ctx, item)
	if err != nil {
		_ = s.deleteSecret(item.PasswordRef)
		return domain.Database{}, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, created.ID, status); err != nil {
		return domain.Database{}, err
	}
	return s.store.GetDatabase(ctx, created.ID)
}

// Register records an existing database without creating it. For SQLite the
// file must already exist; for Postgres/MySQL a connection is opened and pinged.
func (s *Service) Register(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := validateMetadata(request)
	if err != nil {
		return domain.Database{}, err
	}
	if item.Driver == domain.DatabaseDriverSQLite || item.Driver == domain.DatabaseDriverDuckDB {
		return s.registerSQLite(ctx, item)
	}
	return s.createNetwork(ctx, item, request.Password)
}

func (s *Service) registerSQLite(ctx context.Context, item domain.Database) (domain.Database, error) {
	info, err := os.Stat(item.Path)
	if err != nil {
		return domain.Database{}, fmt.Errorf("open database file: %w", err)
	}
	if info.IsDir() {
		return domain.Database{}, fmt.Errorf("database path must be a file")
	}
	created, err := s.store.CreateDatabase(ctx, item)
	if err != nil {
		return domain.Database{}, err
	}
	if _, err := s.connection(ctx, created.ID); err != nil {
		_ = s.store.DeleteDatabase(context.Background(), created.ID)
		return domain.Database{}, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, created.ID, domain.DatabaseStatusConnected); err != nil {
		return domain.Database{}, err
	}
	return s.store.GetDatabase(ctx, created.ID)
}

func (s *Service) Update(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := validateMetadata(request)
	if err != nil {
		return domain.Database{}, err
	}
	if strings.TrimSpace(request.ID) == "" {
		return domain.Database{}, fmt.Errorf("database ID is required")
	}
	item.ID = strings.TrimSpace(request.ID)
	stored, err := s.store.GetDatabase(ctx, item.ID)
	if err != nil {
		return domain.Database{}, err
	}
	if item.Driver == domain.DatabaseDriverSQLite || item.Driver == domain.DatabaseDriverDuckDB {
		if _, err := os.Stat(item.Path); err != nil {
			return domain.Database{}, fmt.Errorf("open database file: %w", err)
		}
	}
	if stored.Path != item.Path || stored.Driver != item.Driver {
		s.closeConnection(item.ID)
	}
	if err := s.applyPassword(ctx, &item, request.Password); err != nil {
		return domain.Database{}, err
	}
	updated, err := s.store.UpdateDatabase(ctx, item)
	if err != nil {
		return domain.Database{}, err
	}
	// Drop any cached connection so subsequent calls reopen with the new config.
	if stored.PasswordRef != item.PasswordRef || stored.Host != item.Host || stored.Port != item.Port || stored.Database != item.Database || stored.Username != item.Username || stored.Schema != item.Schema || stored.SSLMode != item.SSLMode || stored.Charset != item.Charset {
		s.closeConnection(item.ID)
	}
	if _, err := s.connection(ctx, updated.ID); err != nil {
		return domain.Database{}, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, updated.ID, domain.DatabaseStatusConnected); err != nil {
		return domain.Database{}, err
	}
	return s.store.GetDatabase(ctx, updated.ID)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("database ID is required")
	}
	// Look up the passwordRef first so we can purge the secret after the row
	// is gone. A failed lookup is fine — we still attempt the delete below
	// which will surface the canonical "not found" error to the caller.
	item, _ := s.store.GetDatabase(ctx, id)
	s.closeConnection(id)
	if err := s.store.DeleteDatabase(ctx, id); err != nil {
		return err
	}
	_ = s.deleteSecret(item.PasswordRef)
	return nil
}

func (s *Service) Inspect(ctx context.Context, id string) (domain.DatabaseSchema, error) {
	db, item, err := s.connectionWithItem(ctx, id)
	if err != nil {
		return domain.DatabaseSchema{}, err
	}
	dialect := dialectFor(item.Driver)
	names, err := dialect.InspectTables(ctx, db, item)
	if err != nil {
		return domain.DatabaseSchema{}, err
	}
	schema := domain.DatabaseSchema{Tables: make([]domain.DatabaseTable, 0, len(names))}
	for _, name := range names {
		table, err := dialect.InspectTable(ctx, db, item, name)
		if err != nil {
			return domain.DatabaseSchema{}, err
		}
		schema.Tables = append(schema.Tables, table)
	}
	return schema, nil
}

// Ping opens (or reuses) a connection, runs the dialect's ping query, and
// persists the resulting status.
func (s *Service) Ping(ctx context.Context, id string) (domain.DatabaseStatus, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.DatabaseStatusError, fmt.Errorf("database ID is required")
	}
	item, err := s.store.GetDatabase(ctx, id)
	if err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	dialect := dialectFor(item.Driver)
	secret, err := s.resolveSecret(item.PasswordRef)
	if err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	db, err := dialect.Open(item, secret)
	if err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	defer func() { _ = db.Close() }()
	maxOpen, maxIdle, maxLifetime := dialect.PoolDefaults()
	db.SetConnMaxLifetime(maxLifetime)
	db.SetMaxIdleConns(maxIdle)
	db.SetMaxOpenConns(maxOpen)
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	if err := pingQueryRow(pingCtx, db, dialect.PingQuery()); err != nil {
		_ = s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusError)
		return domain.DatabaseStatusError, err
	}
	if err := s.store.UpdateDatabaseStatus(ctx, id, domain.DatabaseStatusConnected); err != nil {
		return domain.DatabaseStatusConnected, err
	}
	return domain.DatabaseStatusConnected, nil
}

// TestConnection opens a transient connection with the supplied config and
// password without persisting any state. Used by the "Test connection"
// button in the create modal.
func (s *Service) TestConnection(ctx context.Context, item domain.Database, password string) (domain.DatabaseStatus, error) {
	dialect := dialectFor(item.Driver)
	db, err := dialect.Open(item, password)
	if err != nil {
		return domain.DatabaseStatusError, err
	}
	defer func() { _ = db.Close() }()
	maxOpen, maxIdle, maxLifetime := dialect.PoolDefaults()
	db.SetConnMaxLifetime(maxLifetime)
	db.SetMaxIdleConns(maxIdle)
	db.SetMaxOpenConns(maxOpen)
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return domain.DatabaseStatusError, err
	}
	if err := pingQueryRow(pingCtx, db, dialect.PingQuery()); err != nil {
		return domain.DatabaseStatusError, err
	}
	return domain.DatabaseStatusConnected, nil
}

func (s *Service) ExecuteSQL(ctx context.Context, request domain.SQLRequest) (domain.SQLResult, error) {
	statement := strings.TrimSpace(request.SQL)
	if statement == "" {
		return domain.SQLResult{}, fmt.Errorf("SQL statement is required")
	}
	if err := validateSingleStatement(statement); err != nil {
		return domain.SQLResult{}, err
	}
	args, err := namedArguments(request.Parameters)
	if err != nil {
		return domain.SQLResult{}, err
	}
	db, err := s.connection(ctx, request.DatabaseID)
	if err != nil {
		return domain.SQLResult{}, err
	}
	return executeStatement(ctx, db, statement, args, rowLimit(request.MaxRows))
}

// Debug accepts a SQL script for the interactive workbench. Pipeline nodes use
// ExecuteSQL and remain limited to one statement for deterministic execution.
func (s *Service) Debug(ctx context.Context, request domain.SQLDebugRequest) (domain.SQLResult, error) {
	statements, err := splitStatements(request.SQL)
	if err != nil {
		return domain.SQLResult{}, err
	}
	if len(statements) == 0 {
		return domain.SQLResult{}, fmt.Errorf("SQL statement is required")
	}
	args, err := namedArguments(request.Parameters)
	if err != nil {
		return domain.SQLResult{}, err
	}
	db, err := s.connection(ctx, request.DatabaseID)
	if err != nil {
		return domain.SQLResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return domain.SQLResult{}, fmt.Errorf("begin SQL debug transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var output domain.SQLResult
	for _, statement := range statements {
		output, err = executeStatement(ctx, tx, statement, args, rowLimit(request.MaxRows))
		if err != nil {
			return domain.SQLResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.SQLResult{}, fmt.Errorf("commit SQL debug transaction: %w", err)
	}
	return output, nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func executeStatement(ctx context.Context, executor sqlExecutor, statement string, args []any, limit int) (domain.SQLResult, error) {
	if queryStatement(statement) {
		return query(ctx, executor, statement, args, limit)
	}
	result, err := executor.ExecContext(ctx, statement, args...)
	if err != nil {
		return domain.SQLResult{}, fmt.Errorf("execute SQL: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.SQLResult{}, fmt.Errorf("read affected rows: %w", err)
	}
	output := domain.SQLResult{Columns: []string{}, Rows: []map[string]any{}, RowsAffected: affected}
	if id, idErr := result.LastInsertId(); idErr == nil {
		output.LastInsertID = &id
	}
	return output, nil
}

func namedArguments(parameters []domain.SQLArgument) ([]any, error) {
	args := make([]any, 0, len(parameters))
	seen := make(map[string]struct{}, len(parameters))
	for _, parameter := range parameters {
		name := strings.TrimLeft(strings.TrimSpace(parameter.Name), ":@$")
		if !parameterName.MatchString(name) {
			return nil, fmt.Errorf("invalid SQL parameter name %q", parameter.Name)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("duplicate SQL parameter %q", name)
		}
		seen[name] = struct{}{}
		value, err := sqliteArgument(parameter.Value)
		if err != nil {
			return nil, fmt.Errorf("SQL parameter %q: %w", name, err)
		}
		args = append(args, sql.Named(name, value))
	}
	return args, nil
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var first error
	for id, db := range s.dbs {
		if err := db.Close(); err != nil && first == nil {
			first = fmt.Errorf("close database %q: %w", id, err)
		}
	}
	clear(s.dbs)
	return first
}

// connection returns the cached *sql.DB for id, opening one on first use.
func (s *Service) connection(ctx context.Context, id string) (*sql.DB, error) {
	db, _, err := s.connectionWithItem(ctx, id)
	return db, err
}

// connectionWithItem is the shared open-or-reuse path. It returns the cached
// *sql.DB along with the domain.Database metadata so callers can dispatch to
// dialect-specific helpers (InspectTables, InspectTable).
func (s *Service) connectionWithItem(ctx context.Context, id string) (*sql.DB, domain.Database, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.Database{}, fmt.Errorf("database ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, domain.Database{}, fmt.Errorf("database service is closed")
	}
	if db := s.dbs[id]; db != nil {
		item, err := s.store.GetDatabase(ctx, id)
		if err != nil {
			return nil, domain.Database{}, err
		}
		return db, item, nil
	}
	item, err := s.store.GetDatabase(ctx, id)
	if err != nil {
		return nil, domain.Database{}, err
	}
	dialect := dialectFor(item.Driver)
	secret, err := s.resolveSecret(item.PasswordRef)
	if err != nil {
		return nil, domain.Database{}, err
	}
	db, err := dialect.Open(item, secret)
	if err != nil {
		return nil, domain.Database{}, fmt.Errorf("open registered database: %w", err)
	}
	maxOpen, maxIdle, maxLifetime := dialect.PoolDefaults()
	db.SetConnMaxLifetime(maxLifetime)
	db.SetMaxIdleConns(maxIdle)
	db.SetMaxOpenConns(maxOpen)
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, domain.Database{}, fmt.Errorf("open registered database: %w", err)
	}
	if err := pingQueryRow(pingCtx, db, dialect.PingQuery()); err != nil {
		_ = db.Close()
		return nil, domain.Database{}, fmt.Errorf("validate registered database: %w", err)
	}
	s.dbs[id] = db
	return db, item, nil
}

func (s *Service) closeConnection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db := s.dbs[id]; db != nil {
		_ = db.Close()
		delete(s.dbs, id)
	}
}

// resolveSecret returns the plaintext password stored under ref. If the
// service has no vault or ref is empty, the empty string is returned so that
// connections without passwords still proceed.
func (s *Service) resolveSecret(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return "", nil
	}
	secret, err := s.vault.Get(ref)
	if err != nil {
		return "", fmt.Errorf("load database password: %w", err)
	}
	return secret, nil
}

// applyPassword writes a new password (if any) to the vault and updates
// item.PasswordRef. If password is empty the existing ref is preserved.
func (s *Service) applyPassword(_ context.Context, item *domain.Database, password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil
	}
	if s.vault == nil {
		return fmt.Errorf("database password storage is unavailable")
	}
	ref := strings.TrimSpace(item.PasswordRef)
	if ref == "" {
		ref = "dbpw:" + uuid.NewString()
		item.PasswordRef = ref
	}
	if err := s.vault.Put(ref, password); err != nil {
		return fmt.Errorf("store database password: %w", err)
	}
	return nil
}

// deleteSecret removes the password entry for ref. Errors are swallowed
// because the database row deletion has already succeeded by the time we call
// this and we don't want to leak dangling secrets, but we also don't want to
// block the user-facing Delete call.
func (s *Service) deleteSecret(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return nil
	}
	return s.vault.Delete(ref)
}

// pingQueryRow runs a single-row, single-column statement used to verify
// the connection is fully usable beyond a bare TCP ping. A nil error from
// sql.ErrNoRows is treated as success.
func pingQueryRow(ctx context.Context, db *sql.DB, query string) error {
	var ignored any
	if err := db.QueryRowContext(ctx, query).Scan(&ignored); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func validateMetadata(request domain.SaveDatabaseRequest) (domain.Database, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return domain.Database{}, fmt.Errorf("database name is required")
	}
	driver := request.Driver
	if driver == "" {
		driver = domain.DatabaseDriverSQLite
	}
	item := domain.Database{
		Name:        name,
		Driver:      driver,
		Host:        strings.TrimSpace(request.Host),
		Port:        request.Port,
		Database:    strings.TrimSpace(request.Database),
		Username:    strings.TrimSpace(request.Username),
		PasswordRef: strings.TrimSpace(request.PasswordRef),
		Schema:      strings.TrimSpace(request.Schema),
		SSLMode:     strings.TrimSpace(request.SSLMode),
		Charset:     strings.TrimSpace(request.Charset),
		Options:     strings.TrimSpace(request.Options),
	}
	switch driver {
	case domain.DatabaseDriverSQLite, domain.DatabaseDriverDuckDB:
		path := strings.TrimSpace(request.Path)
		if path == "" {
			return domain.Database{}, fmt.Errorf("database path is required")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return domain.Database{}, fmt.Errorf("resolve database path: %w", err)
		}
		item.Path = filepath.Clean(absolute)
	case domain.DatabaseDriverPostgres:
		if item.Host == "" {
			return domain.Database{}, fmt.Errorf("database host is required")
		}
		if item.Database == "" {
			return domain.Database{}, fmt.Errorf("database name is required")
		}
		if item.Port == 0 {
			item.Port = 5432
		}
		if item.SSLMode == "" {
			item.SSLMode = "prefer"
		}
		if item.Schema == "" {
			item.Schema = "public"
		}
	case domain.DatabaseDriverMySQL:
		if item.Host == "" {
			return domain.Database{}, fmt.Errorf("database host is required")
		}
		if item.Database == "" {
			return domain.Database{}, fmt.Errorf("database name is required")
		}
		if item.Port == 0 {
			item.Port = 3306
		}
		if item.Charset == "" {
			item.Charset = "utf8mb4"
		}
	default:
		return domain.Database{}, fmt.Errorf("unsupported database driver %q", driver)
	}
	return item, nil
}

func query(ctx context.Context, executor sqlExecutor, statement string, args []any, limit int) (domain.SQLResult, error) {
	rows, err := executor.QueryContext(ctx, statement, args...)
	if err != nil {
		return domain.SQLResult{}, fmt.Errorf("query SQL: %w", err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return domain.SQLResult{}, fmt.Errorf("read SQL columns: %w", err)
	}
	result := domain.SQLResult{Columns: columns, Rows: make([]map[string]any, 0)}
	for rows.Next() {
		if len(result.Rows) >= limit {
			result.Truncated = true
			break
		}
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return domain.SQLResult{}, fmt.Errorf("scan SQL row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for index, column := range columns {
			row[column] = jsonValue(values[index])
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return domain.SQLResult{}, fmt.Errorf("read SQL rows: %w", err)
	}
	return result, nil
}

// sqliteArgument normalises parameter values into JSON-safe primitives
// accepted by every supported driver. Despite the name it is dialect-neutral.
func sqliteArgument(value any) (any, error) {
	switch value := value.(type) {
	case nil, string, bool, int64, float64, []byte:
		return value, nil
	case int:
		return int64(value), nil
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			return integer, nil
		}
		return value.Float64()
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("value is not JSON-safe: %w", err)
		}
		return string(encoded), nil
	}
}

func jsonValue(value any) any {
	switch value := value.(type) {
	case []byte:
		return string(value)
	case int64:
		if value > 1<<53 || value < -(1<<53) {
			return fmt.Sprintf("%d", value)
		}
		return value
	case float64:
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return nil
		}
		return value
	default:
		return value
	}
}

func rowLimit(value int) int {
	if value <= 0 {
		return defaultMaxRows
	}
	if value > absoluteMaxRows {
		return absoluteMaxRows
	}
	return value
}

// queryStatement returns true when the statement should be run as a query
// rather than an exec. A statement is treated as a query when it begins with
// SELECT, PRAGMA, EXPLAIN, WITH, or VALUES, or when an INSERT/UPDATE/DELETE
// (or PostgreSQL UPSERT-style INSERT ... ON CONFLICT) statement contains a
// top-level RETURNING clause.
func queryStatement(statement string) bool {
	body := stripSQLLiterals(statement)
	upper := strings.ToUpper(body)
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "SELECT", "PRAGMA", "EXPLAIN", "WITH", "VALUES":
		return true
	case "INSERT", "UPDATE", "DELETE", "MERGE":
		return hasTopLevelKeyword(upper, "RETURNING")
	}
	return false
}

// stripSQLLiterals returns the statement with single-quoted, double-quoted,
// backtick-quoted, and line/block comments replaced by spaces so that
// keyword detection is not confused by string contents. Dollar-quoted
// Postgres strings ($$...$$) are also stripped.
func stripSQLLiterals(statement string) string {
	var builder strings.Builder
	builder.Grow(len(statement))
	inSingle, inDouble, inBacktick, inLineComment, inBlockComment := false, false, false, false, false
	runes := []rune(statement)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		next := rune(0)
		if index+1 < len(runes) {
			next = runes[index+1]
		}
		switch {
		case inLineComment:
			if char == '\n' {
				inLineComment = false
				builder.WriteRune(char)
			} else {
				builder.WriteRune(' ')
			}
		case inBlockComment:
			if char == '*' && next == '/' {
				inBlockComment = false
				index++
				builder.WriteString("  ")
			} else {
				builder.WriteRune(' ')
			}
		case inSingle:
			if char == '\'' {
				if next == '\'' {
					index++
					builder.WriteString("  ")
				} else {
					inSingle = false
					builder.WriteRune(' ')
				}
			} else {
				builder.WriteRune(' ')
			}
		case inDouble:
			if char == '"' {
				if next == '"' {
					index++
					builder.WriteString("  ")
				} else {
					inDouble = false
					builder.WriteRune(' ')
				}
			} else {
				builder.WriteRune(' ')
			}
		case inBacktick:
			if char == '`' {
				if next == '`' {
					index++
					builder.WriteString("  ")
				} else {
					inBacktick = false
					builder.WriteRune(' ')
				}
			} else {
				builder.WriteRune(' ')
			}
		default:
			switch {
			case char == '-' && next == '-':
				inLineComment = true
				index++
				builder.WriteString("  ")
			case char == '/' && next == '*':
				inBlockComment = true
				index++
				builder.WriteString("  ")
			case char == '\'':
				inSingle = true
				builder.WriteRune(' ')
			case char == '"':
				inDouble = true
				builder.WriteRune(' ')
			case char == '`':
				inBacktick = true
				builder.WriteRune(' ')
			default:
				builder.WriteRune(char)
			}
		}
	}
	// Best-effort Postgres dollar-quote stripping ($$...$$). We replace the
	// contents with spaces so RETURNING outside the literal still matches.
	return stripDollarQuotes(builder.String())
}

// stripDollarQuotes replaces the body of Postgres $$...$$ (and $tag$...$tag$)
// string literals with spaces. Tag matching is greedy on the leading
// $...$ delimiter.
func stripDollarQuotes(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	runes := []rune(text)
	index := 0
	for index < len(runes) {
		if runes[index] != '$' {
			builder.WriteRune(runes[index])
			index++
			continue
		}
		// Capture the tag: $tag$
		start := index
		index++
		for index < len(runes) && runes[index] != '$' {
			index++
		}
		if index >= len(runes) {
			builder.WriteString(string(runes[start:]))
			return builder.String()
		}
		index++ // consume closing $ of opener
		tag := string(runes[start:index])
		closeTag := tag
		end := index
		found := false
		for end+len(closeTag) <= len(runes) {
			if string(runes[end:end+len(closeTag)]) == closeTag {
				found = true
				break
			}
			end++
		}
		if !found {
			builder.WriteString(string(runes[start:index]))
			continue
		}
		// Replace body (including delimiters) with spaces, preserving length.
		for count := 0; count < end+len(closeTag)-start; count++ {
			builder.WriteRune(' ')
		}
		index = end + len(closeTag)
	}
	return builder.String()
}

// hasTopLevelKeyword reports whether the keyword occurs as a top-level token
// in upper. The input is expected to have literals/comments stripped already.
func hasTopLevelKeyword(upper, keyword string) bool {
	upper = " " + upper + " "
	target := " " + keyword + " "
	return strings.Contains(upper, target)
}

func validateSingleStatement(statement string) error {
	statements, err := splitStatements(statement)
	if err != nil {
		return err
	}
	if len(statements) != 1 {
		return fmt.Errorf("SQL must contain exactly one statement")
	}
	return nil
}

func splitStatements(script string) ([]string, error) {
	var statements []string
	start := 0
	quote := rune(0)
	inLineComment, inBlockComment := false, false
	runes := []rune(script)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		next := rune(0)
		if index+1 < len(runes) {
			next = runes[index+1]
		}
		if inLineComment {
			if char == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if char == '*' && next == '/' {
				inBlockComment = false
				index++
			}
			continue
		}
		if quote != 0 {
			if char == quote {
				if next == quote {
					index++
					continue
				}
				quote = 0
			}
			continue
		}
		if char == '-' && next == '-' {
			inLineComment = true
			index++
			continue
		}
		if char == '/' && next == '*' {
			inBlockComment = true
			index++
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
		case '?':
			return nil, fmt.Errorf("SQL must use named parameters instead of positional placeholders")
		case ';':
			statement := strings.TrimSpace(string(runes[start:index]))
			if statement != "" {
				statements = append(statements, statement)
			}
			start = index + 1
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("SQL contains an unterminated quoted value")
	}
	if inBlockComment {
		return nil, fmt.Errorf("SQL contains an unterminated comment")
	}
	if statement := strings.TrimSpace(string(runes[start:])); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}
