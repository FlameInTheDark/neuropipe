package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

// CreateGlobalVariable declares a new workspace variable. The name is the
// immutable identifier referenced by node configurations; the caller has
// already validated it through domain.ValidateGlobalVariableName.
func (s *Store) CreateGlobalVariable(ctx context.Context, variable domain.GlobalVariable) (domain.GlobalVariable, error) {
	variable.Name = strings.TrimSpace(variable.Name)
	if err := domain.ValidateGlobalVariableName(variable.Name); err != nil {
		return domain.GlobalVariable{}, err
	}
	if !domain.ValidDataType(variable.DataType) {
		return domain.GlobalVariable{}, fmt.Errorf("invalid data type %q", variable.DataType)
	}
	now := time.Now().UTC()
	variable.ID = uuid.NewString()
	variable.CreatedAt, variable.UpdatedAt = now, now
	defaultJSON, err := encode(variable.DefaultValue)
	if err != nil {
		return domain.GlobalVariable{}, err
	}
	_, err = statements(s.db).Insert("global_variables").Columns("id", "name", "description", "data_type", "default_value", "created_at", "updated_at").Values(variable.ID, variable.Name, strings.TrimSpace(variable.Description), variable.DataType, defaultJSON, stamp(now), stamp(now)).ExecContext(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.GlobalVariable{}, fmt.Errorf("variable %q already exists", variable.Name)
		}
		return domain.GlobalVariable{}, fmt.Errorf("create global variable: %w", err)
	}
	return s.GetGlobalVariable(ctx, variable.ID)
}

// ListGlobalVariables returns all declarations for the Variables library and
// the node-catalog picklist, in stable name order.
func (s *Store) ListGlobalVariables(ctx context.Context) ([]domain.GlobalVariable, error) {
	rows, err := statements(s.db).Select("id", "name", "description", "data_type", "default_value", "created_at", "updated_at").From("global_variables").OrderBy("name COLLATE NOCASE").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list global variables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	variables := make([]domain.GlobalVariable, 0)
	for rows.Next() {
		var item domain.GlobalVariable
		var dataType, defaultJSON, created, updated string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &dataType, &defaultJSON, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan global variable: %w", err)
		}
		item.DataType = domain.DataType(dataType)
		if err := decode(defaultJSON, &item.DefaultValue); err != nil {
			return nil, err
		}
		item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
		variables = append(variables, item)
	}
	return variables, rows.Err()
}

// GetGlobalVariable loads one declaration by identifier.
func (s *Store) GetGlobalVariable(ctx context.Context, id string) (domain.GlobalVariable, error) {
	var variable domain.GlobalVariable
	var dataType, defaultJSON, created, updated string
	err := statements(s.db).Select("id", "name", "description", "data_type", "default_value", "created_at", "updated_at").From("global_variables").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx).Scan(&variable.ID, &variable.Name, &variable.Description, &dataType, &defaultJSON, &created, &updated)
	if err != nil {
		return domain.GlobalVariable{}, fmt.Errorf("get global variable: %w", err)
	}
	variable.DataType = domain.DataType(dataType)
	if err := decode(defaultJSON, &variable.DefaultValue); err != nil {
		return domain.GlobalVariable{}, err
	}
	variable.CreatedAt, variable.UpdatedAt = parseTime(created), parseTime(updated)
	return variable, nil
}

// UpdateGlobalVariableMetadata only edits fields that do not affect existing
// published graphs. Name and DataType are immutable because node configs
// reference variables by name and previously published revisions bake the type
// into output pins.
func (s *Store) UpdateGlobalVariableMetadata(ctx context.Context, variable domain.GlobalVariable) (domain.GlobalVariable, error) {
	stored, err := s.GetGlobalVariable(ctx, variable.ID)
	if err != nil {
		return domain.GlobalVariable{}, err
	}
	if variable.Name != stored.Name {
		return domain.GlobalVariable{}, fmt.Errorf("variable name cannot be changed after creation")
	}
	if variable.DataType != stored.DataType {
		return domain.GlobalVariable{}, fmt.Errorf("variable data type cannot be changed after creation")
	}
	defaultJSON, err := encode(variable.DefaultValue)
	if err != nil {
		return domain.GlobalVariable{}, err
	}
	now := time.Now().UTC()
	_, err = statements(s.db).Update("global_variables").Set("description", strings.TrimSpace(variable.Description)).Set("default_value", defaultJSON).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": variable.ID}).ExecContext(ctx)
	if err != nil {
		return domain.GlobalVariable{}, fmt.Errorf("update global variable: %w", err)
	}
	return s.GetGlobalVariable(ctx, variable.ID)
}

// DeleteGlobalVariable refuses to remove a declaration still referenced by a
// pipeline or function draft or published revision, mirroring DeleteFunction.
func (s *Store) DeleteGlobalVariable(ctx context.Context, id string) error {
	variable, err := s.GetGlobalVariable(ctx, id)
	if err != nil {
		return err
	}
	needle := "%" + variable.Name + "%"
	var count int
	pipelineRevisions := sqliteStatements.Select("pipeline_id").From("pipeline_revisions").Where("definition LIKE ?", needle)
	if err := statements(s.db).Select("COUNT(*)").From("pipelines").Where(squirrel.Or{squirrel.Expr("draft_definition LIKE ?", needle), squirrel.Expr("id IN (?)", pipelineRevisions)}).QueryRowContext(ctx).Scan(&count); err != nil {
		return fmt.Errorf("check variable dependencies: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("variable is used by %d pipeline definition(s)", count)
	}
	functionRevisions := sqliteStatements.Select("function_id").From("function_revisions").Where("definition LIKE ?", needle)
	if err := statements(s.db).Select("COUNT(*)").From("functions").Where(squirrel.Or{squirrel.Expr("draft_definition LIKE ?", needle), squirrel.Expr("id IN (?)", functionRevisions)}).QueryRowContext(ctx).Scan(&count); err != nil {
		return fmt.Errorf("check composed variable dependencies: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("variable is used by %d custom function definition(s)", count)
	}
	result, err := statements(s.db).Delete("global_variables").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete global variable: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("global variable %q not found", id)
	}
	return nil
}

// LoadGlobalVariableValues returns every persisted live value keyed by name.
// Unknown names (declarations removed since the flush) are silently dropped;
// the values table cascades on definition delete, so leftovers are rare.
func (s *Store) LoadGlobalVariableValues(ctx context.Context) (map[string]any, error) {
	rows, err := statements(s.db).Select("name", "value").From("global_variable_values").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("load global variable values: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string]any)
	for rows.Next() {
		var name, payload string
		if err := rows.Scan(&name, &payload); err != nil {
			return nil, fmt.Errorf("scan global variable value: %w", err)
		}
		var value any
		if err := decode(payload, &value); err != nil {
			return nil, err
		}
		result[name] = value
	}
	return result, rows.Err()
}

// FlushGlobalVariableValues atomically persists a snapshot of all live values.
// Each write goes through a single transaction so concurrent flushes serialize,
// and the connection is already pinned to one DB handle with WAL mode.
func (s *Store) FlushGlobalVariableValues(ctx context.Context, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin global variable flush: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := stamp(time.Now().UTC())
	for name, value := range values {
		payload, err := encode(value)
		if err != nil {
			return err
		}
		if _, err := statements(tx).Insert("global_variable_values").Columns("name", "value", "updated_at").Values(name, payload, now).Suffix("ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at").ExecContext(ctx); err != nil {
			return fmt.Errorf("flush global variable %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit global variable flush: %w", err)
	}
	return nil
}
