package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

// isUniqueViolation checks for duplicate-key errors across SQLite, Postgres,
// and MySQL without importing driver-specific types.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "Duplicate entry")
}

func databaseColumns() []string {
	return []string{"id", "name", "driver", "path", "host", "port", "database_name", "username", "password_ref", "schema_name", "ssl_mode", "charset", "options", "db_index", "use_tls", "client_name", "address", "status", "last_ping_at", "created_at", "updated_at"}
}

func (s *Store) CreateDatabase(ctx context.Context, item domain.Database) (domain.Database, error) {
	now := time.Now().UTC()
	item.ID, item.CreatedAt, item.UpdatedAt = uuid.NewString(), now, now
	if item.Driver == "" {
		item.Driver = domain.DatabaseDriverSQLite
	}
	if item.Status == "" {
		item.Status = domain.DatabaseStatusUnverified
	}
	_, err := statements(s.db).Insert("databases").Columns(databaseColumns()...).Values(
		item.ID, item.Name, string(item.Driver), item.Path, item.Host, item.Port, item.Database,
		item.Username, item.PasswordRef, item.Schema, item.SSLMode, item.Charset, item.Options,
		item.DBIndex, item.UseTLS, item.ClientName, item.Address,
		string(item.Status), stampOrNil(item.LastPingAt), stamp(now), stamp(now),
	).ExecContext(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Database{}, fmt.Errorf("database name is already registered")
		}
		return domain.Database{}, fmt.Errorf("register database: %w", err)
	}
	return s.GetDatabase(ctx, item.ID)
}

func (s *Store) ListDatabases(ctx context.Context) ([]domain.Database, error) {
	rows, err := statements(s.db).Select(databaseColumns()...).From("databases").OrderBy("name COLLATE NOCASE", "id").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.Database, 0)
	for rows.Next() {
		item, err := scanDatabase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetDatabase(ctx context.Context, id string) (domain.Database, error) {
	row := statements(s.db).Select(databaseColumns()...).From("databases").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx)
	return scanDatabase(row)
}

func (s *Store) UpdateDatabase(ctx context.Context, item domain.Database) (domain.Database, error) {
	now := time.Now().UTC()
	result, err := statements(s.db).Update("databases").
		Set("name", item.Name).
		Set("driver", string(item.Driver)).
		Set("path", item.Path).
		Set("host", item.Host).
		Set("port", item.Port).
		Set("database_name", item.Database).
		Set("username", item.Username).
		Set("password_ref", item.PasswordRef).
		Set("schema_name", item.Schema).
		Set("ssl_mode", item.SSLMode).
		Set("charset", item.Charset).
		Set("options", item.Options).
		Set("db_index", item.DBIndex).
		Set("use_tls", item.UseTLS).
		Set("client_name", item.ClientName).
		Set("address", item.Address).
		Set("status", string(item.Status)).
		Set("last_ping_at", stampOrNil(item.LastPingAt)).
		Set("updated_at", stamp(now)).
		Where(squirrel.Eq{"id": item.ID}).ExecContext(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Database{}, fmt.Errorf("database name is already registered")
		}
		return domain.Database{}, fmt.Errorf("update database: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.Database{}, fmt.Errorf("database %q not found", item.ID)
	}
	return s.GetDatabase(ctx, item.ID)
}

func (s *Store) UpdateDatabaseStatus(ctx context.Context, id string, status domain.DatabaseStatus) error {
	now := time.Now().UTC()
	_, err := statements(s.db).Update("databases").
		Set("status", string(status)).
		Set("last_ping_at", stamp(now)).
		Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("update database status: %w", err)
	}
	return nil
}

func (s *Store) DeleteDatabase(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("databases").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete database: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("database %q not found", id)
	}
	return nil
}

type databaseScanner interface{ Scan(...any) error }

func scanDatabase(scanner databaseScanner) (domain.Database, error) {
	var item domain.Database
	var created, updated string
	var driver, status string
	var path, host, database, username, passwordRef, schema, sslMode, charset, options, clientName, address, lastPing sql.NullString
	var port, dbIndex, useTLS sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Name, &driver, &path, &host, &port, &database,
		&username, &passwordRef, &schema, &sslMode, &charset, &options,
		&dbIndex, &useTLS, &clientName, &address,
		&status, &lastPing, &created, &updated,
	); err != nil {
		return domain.Database{}, fmt.Errorf("get database: %w", err)
	}
	item.Driver = domain.DatabaseDriver(driver)
	if item.Driver == "" {
		item.Driver = domain.DatabaseDriverSQLite
	}
	item.Path = path.String
	item.Host = host.String
	item.Port = int(port.Int64)
	item.Database = database.String
	item.Username = username.String
	item.PasswordRef = passwordRef.String
	item.Schema = schema.String
	item.SSLMode = sslMode.String
	item.Charset = charset.String
	item.Options = options.String
	item.DBIndex = int(dbIndex.Int64)
	item.UseTLS = useTLS.Int64 != 0
	item.ClientName = clientName.String
	item.Address = address.String
	item.Status = domain.DatabaseStatus(status)
	if item.Status == "" {
		item.Status = domain.DatabaseStatusUnknown
	}
	if lastPing.Valid {
		t := parseTime(lastPing.String)
		item.LastPingAt = &t
	}
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}

// stampOrNil returns a formatted timestamp or NULL for nil times.
func stampOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return stamp(*t)
}
