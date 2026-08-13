package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

func (s *Store) CreateDatabase(ctx context.Context, item domain.Database) (domain.Database, error) {
	now := time.Now().UTC()
	item.ID, item.CreatedAt, item.UpdatedAt = uuid.NewString(), now, now
	_, err := statements(s.db).Insert("databases").Columns("id", "name", "path", "created_at", "updated_at").Values(item.ID, item.Name, item.Path, stamp(now), stamp(now)).ExecContext(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.Database{}, fmt.Errorf("database file is already registered")
		}
		return domain.Database{}, fmt.Errorf("register database: %w", err)
	}
	return s.GetDatabase(ctx, item.ID)
}

func (s *Store) ListDatabases(ctx context.Context) ([]domain.Database, error) {
	rows, err := statements(s.db).Select("id", "name", "path", "created_at", "updated_at").From("databases").OrderBy("name COLLATE NOCASE", "id").QueryContext(ctx)
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
	return scanDatabase(statements(s.db).Select("id", "name", "path", "created_at", "updated_at").From("databases").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx))
}

func (s *Store) UpdateDatabase(ctx context.Context, item domain.Database) (domain.Database, error) {
	now := time.Now().UTC()
	result, err := statements(s.db).Update("databases").Set("name", item.Name).Set("path", item.Path).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": item.ID}).ExecContext(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.Database{}, fmt.Errorf("database file is already registered")
		}
		return domain.Database{}, fmt.Errorf("update database: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.Database{}, fmt.Errorf("database %q not found", item.ID)
	}
	return s.GetDatabase(ctx, item.ID)
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
	if err := scanner.Scan(&item.ID, &item.Name, &item.Path, &created, &updated); err != nil {
		return domain.Database{}, fmt.Errorf("get database: %w", err)
	}
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}
