// Package databases manages registered local SQLite files and their connections.
package databases

import (
	"context"
	"database/sql"
	"encoding/json"
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
	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultMaxRows  = 500
	absoluteMaxRows = 10_000
)

var parameterName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Service struct {
	store  *persistence.Store
	mu     sync.Mutex
	dbs    map[string]*sql.DB
	closed bool
}

func New(store *persistence.Store) *Service {
	return &Service{store: store, dbs: make(map[string]*sql.DB)}
}

func (s *Service) List(ctx context.Context) ([]domain.Database, error) {
	return s.store.ListDatabases(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Database, error) {
	return s.store.GetDatabase(ctx, strings.TrimSpace(id))
}

// Create makes a new SQLite file and registers it. Existing files are never overwritten.
func (s *Service) Create(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := validateMetadata(request)
	if err != nil {
		return domain.Database{}, err
	}
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
	return created, nil
}

// Register records an existing SQLite file without changing it.
func (s *Service) Register(ctx context.Context, request domain.SaveDatabaseRequest) (domain.Database, error) {
	item, err := validateMetadata(request)
	if err != nil {
		return domain.Database{}, err
	}
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
	return created, nil
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
	if _, err := os.Stat(item.Path); err != nil {
		return domain.Database{}, fmt.Errorf("open database file: %w", err)
	}
	stored, err := s.store.GetDatabase(ctx, item.ID)
	if err != nil {
		return domain.Database{}, err
	}
	if stored.Path != item.Path {
		s.closeConnection(item.ID)
	}
	updated, err := s.store.UpdateDatabase(ctx, item)
	if err != nil {
		return domain.Database{}, err
	}
	if _, err := s.connection(ctx, updated.ID); err != nil {
		return domain.Database{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("database ID is required")
	}
	s.closeConnection(id)
	return s.store.DeleteDatabase(ctx, id)
}

func (s *Service) Inspect(ctx context.Context, id string) (domain.DatabaseSchema, error) {
	db, err := s.connection(ctx, id)
	if err != nil {
		return domain.DatabaseSchema{}, err
	}
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return domain.DatabaseSchema{}, fmt.Errorf("inspect database tables: %w", err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return domain.DatabaseSchema{}, fmt.Errorf("inspect database table: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return domain.DatabaseSchema{}, fmt.Errorf("inspect database tables: %w", err)
	}
	schema := domain.DatabaseSchema{Tables: make([]domain.DatabaseTable, 0, len(names))}
	for _, name := range names {
		table, err := inspectTable(ctx, db, name)
		if err != nil {
			return domain.DatabaseSchema{}, err
		}
		schema.Tables = append(schema.Tables, table)
	}
	return schema, nil
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

func (s *Service) connection(ctx context.Context, id string) (*sql.DB, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("database ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("database service is closed")
	}
	if db := s.dbs[id]; db != nil {
		return db, nil
	}
	item, err := s.store.GetDatabase(ctx, id)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(item.Path)+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open registered database: %w", err)
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(4)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open registered database: %w", err)
	}
	var version string
	if err := db.QueryRowContext(pingCtx, `SELECT sqlite_version()`).Scan(&version); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("validate registered database: %w", err)
	}
	s.dbs[id] = db
	return db, nil
}

func (s *Service) closeConnection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db := s.dbs[id]; db != nil {
		_ = db.Close()
		delete(s.dbs, id)
	}
}

func validateMetadata(request domain.SaveDatabaseRequest) (domain.Database, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return domain.Database{}, fmt.Errorf("database name is required")
	}
	path := strings.TrimSpace(request.Path)
	if path == "" {
		return domain.Database{}, fmt.Errorf("database path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return domain.Database{}, fmt.Errorf("resolve database path: %w", err)
	}
	return domain.Database{Name: name, Path: filepath.Clean(absolute)}, nil
}

func inspectTable(ctx context.Context, db *sql.DB, name string) (domain.DatabaseTable, error) {
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

func queryStatement(statement string) bool {
	fields := strings.Fields(strings.ToUpper(statement))
	return len(fields) > 0 && (fields[0] == "SELECT" || fields[0] == "PRAGMA" || fields[0] == "EXPLAIN" || fields[0] == "WITH")
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
