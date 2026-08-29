package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

func storageColumns() []string {
	return []string{"id", "name", "driver", "endpoint", "region", "bucket", "access_key", "secret_ref", "secure", "host", "port", "username", "password_ref", "tls_mode", "base_dir", "public_base_url", "status", "last_ping_at", "created_at", "updated_at"}
}

func (s *Store) CreateStorage(ctx context.Context, item domain.Storage) (domain.Storage, error) {
	now := time.Now().UTC()
	item.ID, item.CreatedAt, item.UpdatedAt = uuid.NewString(), now, now
	if item.Driver == "" {
		item.Driver = domain.StorageDriverS3
	}
	if item.Status == "" {
		item.Status = domain.DatabaseStatusUnverified
	}
	if item.TLSMode == "" {
		item.TLSMode = domain.StorageTLSNone
	}
	_, err := statements(s.db).Insert("storages").Columns(storageColumns()...).Values(
		item.ID, item.Name, string(item.Driver), item.Endpoint, item.Region, item.Bucket, item.AccessKey, item.SecretRef,
		storageSecure(item), item.Host, item.Port, item.Username, item.PasswordRef, string(item.TLSMode), item.BaseDir, item.PublicBaseURL,
		string(item.Status), stampOrNil(item.LastPingAt), stamp(now), stamp(now),
	).ExecContext(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Storage{}, fmt.Errorf("storage name is already registered")
		}
		return domain.Storage{}, fmt.Errorf("register storage: %w", err)
	}
	return s.GetStorage(ctx, item.ID)
}

func (s *Store) ListStorages(ctx context.Context) ([]domain.Storage, error) {
	rows, err := statements(s.db).Select(storageColumns()...).From("storages").OrderBy("name COLLATE NOCASE", "id").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list storages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.Storage, 0)
	for rows.Next() {
		item, err := scanStorage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetStorage(ctx context.Context, id string) (domain.Storage, error) {
	row := statements(s.db).Select(storageColumns()...).From("storages").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx)
	return scanStorage(row)
}

func (s *Store) UpdateStorage(ctx context.Context, item domain.Storage) (domain.Storage, error) {
	now := time.Now().UTC()
	result, err := statements(s.db).Update("storages").
		Set("name", item.Name).
		Set("driver", string(item.Driver)).
		Set("endpoint", item.Endpoint).
		Set("region", item.Region).
		Set("bucket", item.Bucket).
		Set("access_key", item.AccessKey).
		Set("secret_ref", item.SecretRef).
		Set("secure", storageSecure(item)).
		Set("host", item.Host).
		Set("port", item.Port).
		Set("username", item.Username).
		Set("password_ref", item.PasswordRef).
		Set("tls_mode", string(item.TLSMode)).
		Set("base_dir", item.BaseDir).
		Set("public_base_url", item.PublicBaseURL).
		Set("status", string(item.Status)).
		Set("last_ping_at", stampOrNil(item.LastPingAt)).
		Set("updated_at", stamp(now)).
		Where(squirrel.Eq{"id": item.ID}).ExecContext(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Storage{}, fmt.Errorf("storage name is already registered")
		}
		return domain.Storage{}, fmt.Errorf("update storage: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.Storage{}, fmt.Errorf("storage %q not found", item.ID)
	}
	return s.GetStorage(ctx, item.ID)
}

func (s *Store) UpdateStorageStatus(ctx context.Context, id string, status domain.DatabaseStatus) error {
	now := time.Now().UTC()
	_, err := statements(s.db).Update("storages").
		Set("status", string(status)).
		Set("last_ping_at", stamp(now)).
		Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("update storage status: %w", err)
	}
	return nil
}

func (s *Store) DeleteStorage(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("storages").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete storage: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("storage %q not found", id)
	}
	return nil
}

type storageScanner interface{ Scan(...any) error }

func scanStorage(scanner storageScanner) (domain.Storage, error) {
	var item domain.Storage
	var created, updated string
	var driver, status, tlsMode string
	var endpoint, region, bucket, accessKey, secretRef, host, username, passwordRef, baseDir, publicBaseURL, lastPing sql.NullString
	var port, secure sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Name, &driver, &endpoint, &region, &bucket, &accessKey, &secretRef,
		&secure, &host, &port, &username, &passwordRef, &tlsMode, &baseDir, &publicBaseURL,
		&status, &lastPing, &created, &updated,
	); err != nil {
		return domain.Storage{}, fmt.Errorf("get storage: %w", err)
	}
	item.Driver = domain.StorageDriver(driver)
	if item.Driver == "" {
		item.Driver = domain.StorageDriverS3
	}
	item.Endpoint = endpoint.String
	item.Region = region.String
	item.Bucket = bucket.String
	item.AccessKey = accessKey.String
	item.SecretRef = secretRef.String
	if secure.Valid {
		value := secure.Int64 != 0
		item.Secure = &value
	}
	item.Host = host.String
	item.Port = int(port.Int64)
	item.Username = username.String
	item.PasswordRef = passwordRef.String
	item.TLSMode = domain.StorageTLSMode(tlsMode)
	if item.TLSMode == "" {
		item.TLSMode = domain.StorageTLSNone
	}
	item.BaseDir = baseDir.String
	item.PublicBaseURL = publicBaseURL.String
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

// storageSecure returns the persisted Secure flag (true when unset, so
// existing rows keep TLS on for custom endpoints).
func storageSecure(item domain.Storage) bool {
	if item.Secure == nil {
		return true
	}
	return *item.Secure
}
