// Package persistence provides SQLite-backed storage for Neuropipe aggregates.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/localization"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Store owns the application database connection and query implementations.
type Store struct {
	db   *sql.DB
	root string
}

// New opens the SQLite database under the supplied application-data root.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create application data directory: %w", err)
	}
	database, err := sql.Open("sqlite3", filepath.Join(root, "neuropipe.db")+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(1)
	store := &Store{db: database, root: root}
	if err := store.backupLegacyGraphs(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	if err := store.migrateBlueprintCatalog(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	if err := store.migrateBlueprintV3(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	if err := store.migrateRerouteWaypoints(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	return store, nil
}

// backupLegacyGraphs creates a consistent SQLite copy immediately before the
// one-way Blueprint-v2 migration, but only when v1 graphs actually exist.
func (s *Store) backupLegacyGraphs(ctx context.Context) error {
	var tables int
	if err := statements(s.db).Select("COUNT(*)").From("sqlite_master").Where(squirrel.Eq{"type": "table", "name": "pipelines"}).QueryRowContext(ctx).Scan(&tables); err != nil {
		return fmt.Errorf("check database schema: %w", err)
	}
	if tables == 0 {
		return nil
	}
	var legacy int
	if err := statements(s.db).Select("COUNT(*)").From("pipelines").Where("draft_definition NOT LIKE ?", "%\"schemaVersion\":2%").Where("draft_definition NOT LIKE ?", "%\"schemaVersion\":3%").QueryRowContext(ctx).Scan(&legacy); err != nil {
		return fmt.Errorf("check legacy graphs: %w", err)
	}
	if legacy == 0 {
		return nil
	}
	backup := filepath.Join(s.root, "neuropipe-pre-blueprint-v2-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".db")
	escaped := strings.ReplaceAll(backup, "'", "''")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("back up legacy database: %w", err)
	}
	return nil
}

// Close releases database resources.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS pipelines (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  icon TEXT NOT NULL DEFAULT 'workflow',
  icon_color TEXT NOT NULL DEFAULT '#e4e4e7',
  icon_background TEXT NOT NULL DEFAULT '#27272a',
  status TEXT NOT NULL,
  draft_definition TEXT NOT NULL,
  published_revision INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS pipeline_revisions (
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL,
  definition TEXT NOT NULL,
  published_at TEXT NOT NULL,
  PRIMARY KEY (pipeline_id, revision)
);
CREATE TABLE IF NOT EXISTS trigger_bindings (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL,
  revision INTEGER NOT NULL,
  kind TEXT NOT NULL,
          node_type TEXT NOT NULL DEFAULT '',
          config_json TEXT NOT NULL DEFAULT '{}',
  label TEXT NOT NULL,
  icon TEXT NOT NULL,
  color TEXT NOT NULL,
  grid_position INTEGER NOT NULL DEFAULT 0,
  hotkey TEXT NOT NULL DEFAULT '',
  cron TEXT NOT NULL DEFAULT '',
  timezone TEXT NOT NULL DEFAULT 'Local',
  enabled INTEGER NOT NULL DEFAULT 0,
  trusted INTEGER NOT NULL DEFAULT 0,
  next_run_at TEXT,
  last_run_at TEXT,
  last_run_status TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(pipeline_id, node_id, revision)
);
CREATE TABLE IF NOT EXISTS executions (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  trigger_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS node_runs (
  execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  node_id TEXT NOT NULL,
  node_type TEXT NOT NULL,
  status TEXT NOT NULL,
  input_json TEXT NOT NULL DEFAULT '',
  output_json TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  PRIMARY KEY (execution_id, ordinal)
);
CREATE TABLE IF NOT EXISTS reports (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL,
  title TEXT NOT NULL,
  tags_json TEXT NOT NULL DEFAULT '[]',
  markdown TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS reports_created_at ON reports(created_at DESC);
CREATE TABLE IF NOT EXISTS chat_conversations (
  id TEXT PRIMARY KEY,
  mode TEXT NOT NULL,
  title TEXT NOT NULL,
  pipeline_id TEXT NOT NULL DEFAULT '',
  trigger_binding_id TEXT NOT NULL DEFAULT '',
  action_policy TEXT NOT NULL DEFAULT 'ask',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chat_conversations_updated_at ON chat_conversations(updated_at DESC);
CREATE TABLE IF NOT EXISTS chat_runs (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  execution_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  status_text TEXT NOT NULL DEFAULT 'Working',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chat_runs_conversation_created ON chat_runs(conversation_id, created_at DESC);
CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  chat_run_id TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  tool_call_id TEXT NOT NULL DEFAULT '',
  tool_name TEXT NOT NULL DEFAULT '',
  tool_calls_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chat_messages_conversation_created ON chat_messages(conversation_id, created_at ASC);
CREATE TABLE IF NOT EXISTS chat_run_events (
  id TEXT PRIMARY KEY,
  chat_run_id TEXT NOT NULL REFERENCES chat_runs(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  summary TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chat_run_events_run_created ON chat_run_events(chat_run_id, created_at ASC);
CREATE TABLE IF NOT EXISTS chat_approvals (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  chat_run_id TEXT NOT NULL REFERENCES chat_runs(id) ON DELETE CASCADE,
  tool_call_json TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  resolved_at TEXT
);
CREATE INDEX IF NOT EXISTS chat_approvals_conversation_status ON chat_approvals(conversation_id, status, created_at ASC);
CREATE TABLE IF NOT EXISTS chat_tool_grants (
  conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  tool_name TEXT NOT NULL,
  target_id TEXT NOT NULL,
  revision INTEGER NOT NULL,
  granted_at TEXT NOT NULL,
  PRIMARY KEY (conversation_id, tool_name, target_id)
);
CREATE TABLE IF NOT EXISTS permissions (
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL,
  capability TEXT NOT NULL,
  scope TEXT NOT NULL,
  granted_at TEXT NOT NULL,
  PRIMARY KEY (pipeline_id, revision, capability, scope)
);
CREATE TABLE IF NOT EXISTS global_variables (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  data_type TEXT NOT NULL,
  default_value TEXT NOT NULL DEFAULT 'null',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS global_variable_values (
  name TEXT PRIMARY KEY REFERENCES global_variables(name) ON DELETE CASCADE,
  value TEXT NOT NULL DEFAULT 'null',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS databases (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver TEXT NOT NULL DEFAULT 'sqlite',
  path TEXT NOT NULL DEFAULT '',
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  database_name TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  password_ref TEXT NOT NULL DEFAULT '',
  schema_name TEXT NOT NULL DEFAULT '',
  ssl_mode TEXT NOT NULL DEFAULT '',
  charset TEXT NOT NULL DEFAULT '',
  options TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unknown',
  last_ping_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS databases_name ON databases(name COLLATE NOCASE);
CREATE TABLE IF NOT EXISTS functions (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'Functions',
  icon TEXT NOT NULL DEFAULT 'braces',
  icon_color TEXT NOT NULL DEFAULT '#c4b5fd',
  icon_background TEXT NOT NULL DEFAULT '#2e1065',
  kind TEXT NOT NULL DEFAULT 'function',
  mode TEXT NOT NULL,
  inputs_json TEXT NOT NULL DEFAULT '[]',
  outputs_json TEXT NOT NULL DEFAULT '[]',
  draft_definition TEXT NOT NULL,
  published_revision INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS function_revisions (
  function_id TEXT NOT NULL REFERENCES functions(id) ON DELETE CASCADE,
  revision INTEGER NOT NULL,
  metadata_json TEXT NOT NULL,
  definition TEXT NOT NULL,
  published_at TEXT NOT NULL,
  PRIMARY KEY (function_id, revision)
);
CREATE INDEX IF NOT EXISTS functions_updated_at ON functions(updated_at DESC);
CREATE INDEX IF NOT EXISTS function_revisions_function_id ON function_revisions(function_id, revision DESC);
CREATE TABLE IF NOT EXISTS legacy_graphs (
  pipeline_id TEXT PRIMARY KEY REFERENCES pipelines(id) ON DELETE CASCADE,
  detected_at TEXT NOT NULL,
  reason TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS blueprint_migration_issues (
  id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  issue TEXT NOT NULL,
  detected_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS metric_execution_events (
  execution_id TEXT PRIMARY KEY,
  pipeline_id TEXT NOT NULL,
  pipeline_name TEXT NOT NULL,
  trigger_kind TEXT NOT NULL,
  status TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  duration_ms REAL NOT NULL DEFAULT 0,
  queue_wait_ms REAL NOT NULL DEFAULT 0,
  node_count INTEGER NOT NULL DEFAULT 0,
  failed_node_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS metric_execution_events_occurred ON metric_execution_events(occurred_at DESC);
CREATE INDEX IF NOT EXISTS metric_execution_events_pipeline ON metric_execution_events(pipeline_id, occurred_at DESC);
CREATE TABLE IF NOT EXISTS metric_node_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  execution_id TEXT NOT NULL,
  pipeline_id TEXT NOT NULL,
  node_type TEXT NOT NULL,
  status TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  duration_ms REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS metric_node_events_occurred ON metric_node_events(occurred_at DESC);
CREATE INDEX IF NOT EXISTS metric_node_events_type ON metric_node_events(node_type, occurred_at DESC);
CREATE TABLE IF NOT EXISTS metric_llm_events (
  id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL DEFAULT '',
  chat_run_id TEXT NOT NULL DEFAULT '',
  pipeline_id TEXT NOT NULL DEFAULT '',
  node_type TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL DEFAULT '',
  provider_id TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  provider_kind TEXT NOT NULL,
  model TEXT NOT NULL,
  succeeded INTEGER NOT NULL,
  tokens_reported INTEGER NOT NULL,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  queue_wait_ms REAL NOT NULL DEFAULT 0,
  duration_ms REAL NOT NULL DEFAULT 0,
  estimated_cost_usd REAL,
  occurred_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS metric_llm_events_occurred ON metric_llm_events(occurred_at DESC);
CREATE INDEX IF NOT EXISTS metric_llm_events_model ON metric_llm_events(provider_id, model, occurred_at DESC);
CREATE TABLE IF NOT EXISTS metric_activity_events (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  outcome TEXT NOT NULL DEFAULT '',
  duration_ms REAL NOT NULL DEFAULT 0,
  occurred_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS metric_activity_events_occurred ON metric_activity_events(occurred_at DESC);
CREATE TABLE IF NOT EXISTS metric_resource_samples (
  id TEXT PRIMARY KEY,
  process_name TEXT NOT NULL,
  cpu_percent REAL NOT NULL DEFAULT 0,
  working_set_bytes INTEGER NOT NULL DEFAULT 0,
  occurred_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS metric_resource_samples_occurred ON metric_resource_samples(occurred_at DESC);
CREATE TABLE IF NOT EXISTS metric_execution_rollups (
  bucket TEXT NOT NULL,
  pipeline_id TEXT NOT NULL,
  pipeline_name TEXT NOT NULL,
  trigger_kind TEXT NOT NULL,
  status TEXT NOT NULL,
  run_count INTEGER NOT NULL DEFAULT 0,
  duration_sum_ms REAL NOT NULL DEFAULT 0,
  queue_wait_sum_ms REAL NOT NULL DEFAULT 0,
  node_count INTEGER NOT NULL DEFAULT 0,
  failed_node_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(bucket, pipeline_id, trigger_kind, status)
);
CREATE TABLE IF NOT EXISTS metric_llm_rollups (
  bucket TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  provider_kind TEXT NOT NULL,
  model TEXT NOT NULL,
  succeeded INTEGER NOT NULL,
  call_count INTEGER NOT NULL DEFAULT 0,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  tokens_reported_count INTEGER NOT NULL DEFAULT 0,
  duration_sum_ms REAL NOT NULL DEFAULT 0,
  queue_wait_sum_ms REAL NOT NULL DEFAULT 0,
  estimated_cost_sum_usd REAL NOT NULL DEFAULT 0,
  priced_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(bucket, provider_id, model, succeeded)
);
CREATE TABLE IF NOT EXISTS metric_activity_rollups (
  bucket TEXT NOT NULL,
  kind TEXT NOT NULL,
  outcome TEXT NOT NULL,
  event_count INTEGER NOT NULL DEFAULT 0,
  duration_sum_ms REAL NOT NULL DEFAULT 0,
  PRIMARY KEY(bucket, kind, outcome)
);
CREATE TABLE IF NOT EXISTS metric_resource_rollups (
  bucket TEXT NOT NULL,
  process_name TEXT NOT NULL,
  sample_count INTEGER NOT NULL DEFAULT 0,
  cpu_sum REAL NOT NULL DEFAULT 0,
  cpu_peak REAL NOT NULL DEFAULT 0,
  working_set_sum INTEGER NOT NULL DEFAULT 0,
  working_set_peak INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(bucket, process_name)
);`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	if err := s.ensureReportTagsColumn(ctx); err != nil {
		return err
	}
	if err := s.ensurePipelineIconColumn(ctx); err != nil {
		return err
	}
	if err := s.ensureIconAppearanceColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureFunctionKindColumn(ctx); err != nil {
		return err
	}
	if err := s.ensureExecutionTimingColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureTriggerNodeMetadataColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureDatabaseMultiDialectColumns(ctx); err != nil {
		return err
	}
	if err := s.repairV3LegacyMarkers(ctx); err != nil {
		return err
	}
	// Graphs without a supported schema version are preserved for manual rebuild
	// and their triggers are disabled before the V3 runtime can process them.
	legacyPipelines := sqliteStatements.Select("id").Column("?", stamp(time.Now().UTC())).Column("?", "Blueprint v3 rebuild required").From("pipelines").Where("draft_definition NOT LIKE ?", "%\"schemaVersion\":2%").Where("draft_definition NOT LIKE ?", "%\"schemaVersion\":3%")
	_, _ = statements(s.db).Insert("legacy_graphs").Columns("pipeline_id", "detected_at", "reason").Select(legacyPipelines).Suffix("ON CONFLICT (pipeline_id) DO NOTHING").ExecContext(ctx)
	legacyMarkers := sqliteStatements.Select("pipeline_id").From("legacy_graphs")
	_, _ = statements(s.db).Update("trigger_bindings").Set("enabled", false).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Expr("pipeline_id IN (?)", legacyMarkers)).ExecContext(ctx)
	_, _ = statements(s.db).Update("pipelines").Set("status", domain.PipelineLegacy).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Expr("id IN (?)", legacyMarkers)).ExecContext(ctx)
	return nil
}

// repairV3LegacyMarkers removes records written by the brief V3 detector bug
// that classified valid V3 graphs as legacy. The graph is deliberately made a
// draft: restoring publication would require reconstructing the prior revision
// and trust state, while drafts are immediately editable and safe.
func (s *Store) repairV3LegacyMarkers(ctx context.Context) error {
	v3Pipelines := sqliteStatements.Select("id").From("pipelines").Where("draft_definition LIKE ?", "%\"schemaVersion\":3%")
	if _, err := statements(s.db).Delete("legacy_graphs").Where(squirrel.Expr("pipeline_id IN (?)", v3Pipelines)).ExecContext(ctx); err != nil {
		return fmt.Errorf("remove invalid V3 legacy markers: %w", err)
	}
	if _, err := statements(s.db).Update("pipelines").Set("status", domain.PipelineDraft).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"status": domain.PipelineLegacy}).Where("draft_definition LIKE ?", "%\"schemaVersion\":3%").ExecContext(ctx); err != nil {
		return fmt.Errorf("restore invalid V3 legacy pipelines: %w", err)
	}
	return nil
}

func (s *Store) ensureReportTagsColumn(ctx context.Context) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('reports') WHERE name = 'tags_json'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect reports tag migration: %w", err)
	}
	if exists > 0 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE reports ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return fmt.Errorf("add reports tags column: %w", err)
	}
	return nil
}

func (s *Store) ensurePipelineIconColumn(ctx context.Context) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('pipelines') WHERE name = 'icon'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect pipeline icon migration: %w", err)
	}
	if exists > 0 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE pipelines ADD COLUMN icon TEXT NOT NULL DEFAULT 'workflow'`); err != nil {
		return fmt.Errorf("add pipeline icon column: %w", err)
	}
	return nil
}

func (s *Store) ensureIconAppearanceColumns(ctx context.Context) error {
	columns := []struct {
		table, name, definition string
	}{
		{"pipelines", "icon_color", "TEXT NOT NULL DEFAULT '#e4e4e7'"},
		{"pipelines", "icon_background", "TEXT NOT NULL DEFAULT '#27272a'"},
		{"functions", "icon_color", "TEXT NOT NULL DEFAULT '#c4b5fd'"},
		{"functions", "icon_background", "TEXT NOT NULL DEFAULT '#2e1065'"},
	}
	for _, column := range columns {
		var exists int
		query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", column.table)
		if err := s.db.QueryRowContext(ctx, query, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect %s.%s migration: %w", column.table, column.name, err)
		}
		if exists > 0 {
			continue
		}
		statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", column.table, column.name, column.definition)
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("add %s.%s column: %w", column.table, column.name, err)
		}
	}
	return nil
}

func (s *Store) ensureFunctionKindColumn(ctx context.Context) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('functions') WHERE name = 'kind'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect functions.kind migration: %w", err)
	}
	if exists > 0 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE functions ADD COLUMN kind TEXT NOT NULL DEFAULT 'function'`); err != nil {
		return fmt.Errorf("add functions.kind column: %w", err)
	}
	return nil
}

func (s *Store) ensureExecutionTimingColumns(ctx context.Context) error {
	columns := []struct {
		name       string
		definition string
	}{
		{name: "queued_at", definition: "TEXT"},
		{name: "run_started_at", definition: "TEXT"},
	}
	for _, column := range columns {
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('executions') WHERE name = ?`, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect executions.%s migration: %w", column.name, err)
		}
		if exists > 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE executions ADD COLUMN %s %s", column.name, column.definition)); err != nil {
			return fmt.Errorf("add executions.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (s *Store) ensureTriggerNodeMetadataColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{name: "node_type", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "config_json", definition: "TEXT NOT NULL DEFAULT '{}'"},
	}
	for _, column := range columns {
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('trigger_bindings') WHERE name = ?`, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect trigger_bindings.%s migration: %w", column.name, err)
		}
		if exists > 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE trigger_bindings ADD COLUMN %s %s", column.name, column.definition)); err != nil {
			return fmt.Errorf("add trigger_bindings.%s column: %w", column.name, err)
		}
	}
	return nil
}

// ensureDatabaseMultiDialectColumns adds the new multi-dialect columns to the
// databases table for existing installations. Fresh installs get them from the
// CREATE TABLE statement; this only handles the ALTER TABLE path.
func (s *Store) ensureDatabaseMultiDialectColumns(ctx context.Context) error {
	// First check if the old schema exists (path UNIQUE, no driver column)
	var driverExists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('databases') WHERE name = 'driver'`).Scan(&driverExists); err != nil {
		return fmt.Errorf("inspect databases.driver migration: %w", err)
	}
	if driverExists > 0 {
		return nil // already migrated
	}
	// Drop the old UNIQUE constraint on path by recreating the table.
	// SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we recreate.
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS databases_new (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver TEXT NOT NULL DEFAULT 'sqlite',
  path TEXT NOT NULL DEFAULT '',
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  database_name TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  password_ref TEXT NOT NULL DEFAULT '',
  schema_name TEXT NOT NULL DEFAULT '',
  ssl_mode TEXT NOT NULL DEFAULT '',
  charset TEXT NOT NULL DEFAULT '',
  options TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unknown',
  last_ping_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
INSERT INTO databases_new (id, name, driver, path, created_at, updated_at)
SELECT id, name, 'sqlite', path, created_at, updated_at FROM databases;
DROP TABLE databases;
ALTER TABLE databases_new RENAME TO databases;
CREATE INDEX IF NOT EXISTS databases_name ON databases(name COLLATE NOCASE);
`); err != nil {
		return fmt.Errorf("migrate databases table to multi-dialect: %w", err)
	}
	return nil
}

// CreatePipeline persists a new draft pipeline.
func (s *Store) CreatePipeline(ctx context.Context, name string, definition domain.FlowDefinition) (domain.Pipeline, error) {
	now := time.Now().UTC()
	pipeline := domain.Pipeline{ID: uuid.NewString(), Name: strings.TrimSpace(name), Icon: "workflow", IconColor: "#e4e4e7", IconBackground: "#27272a", Status: domain.PipelineDraft, DraftDefinition: definition, CreatedAt: now, UpdatedAt: now}
	if pipeline.Name == "" {
		pipeline.Name = "Untitled pipeline"
	}
	definitionJSON, err := encode(definition)
	if err != nil {
		return domain.Pipeline{}, err
	}
	_, err = statements(s.db).Insert("pipelines").Columns("id", "name", "description", "icon", "icon_color", "icon_background", "status", "draft_definition", "published_revision", "created_at", "updated_at").Values(pipeline.ID, pipeline.Name, pipeline.Description, pipeline.Icon, pipeline.IconColor, pipeline.IconBackground, pipeline.Status, definitionJSON, 0, stamp(now), stamp(now)).ExecContext(ctx)
	if err != nil {
		return domain.Pipeline{}, fmt.Errorf("create pipeline: %w", err)
	}
	return pipeline, nil
}

// DeletePipeline permanently removes a pipeline and its revisions, bindings,
// executions, and reports through the SQLite foreign-key relationships.
func (s *Store) DeletePipeline(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("pipelines").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete pipeline: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("pipeline %q not found", id)
	}
	return nil
}

// ListPipelines returns concise cards ordered by newest edits.
func (s *Store) ListPipelines(ctx context.Context) ([]domain.PipelineSummary, error) {
	migrationIssue := sqliteStatements.Select("issue").From("blueprint_migration_issues mi").Where("mi.pipeline_id = p.id").OrderBy("mi.detected_at DESC").Limit(1)
	rows, err := statements(s.db).Select("p.id", "p.name", "p.description", "p.icon", "p.icon_color", "p.icon_background", "p.status", "p.published_revision", "p.updated_at", "COUNT(t.id)").Column(squirrel.Expr("COALESCE((?), '')", migrationIssue)).From("pipelines p").LeftJoin("trigger_bindings t ON t.pipeline_id = p.id").GroupBy("p.id").OrderBy("p.updated_at DESC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.PipelineSummary, 0)
	for rows.Next() {
		var item domain.PipelineSummary
		var status string
		var updated string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Icon, &item.IconColor, &item.IconBackground, &status, &item.PublishedRevision, &updated, &item.TriggerCount, &item.MigrationIssue); err != nil {
			return nil, fmt.Errorf("scan pipeline summary: %w", err)
		}
		item.Status = domain.PipelineStatus(status)
		item.UpdatedAt = parseTime(updated)
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetPipeline loads an editor-ready pipeline.
func (s *Store) GetPipeline(ctx context.Context, id string) (domain.Pipeline, error) {
	var pipeline domain.Pipeline
	var status, definitionJSON, created, updated string
	migrationIssue := sqliteStatements.Select("issue").From("blueprint_migration_issues mi").Where("mi.pipeline_id = p.id").OrderBy("mi.detected_at DESC").Limit(1)
	err := statements(s.db).Select("p.id", "p.name", "p.description", "p.icon", "p.icon_color", "p.icon_background", "p.status", "p.draft_definition", "p.published_revision", "p.created_at", "p.updated_at").Column(squirrel.Expr("COALESCE((?), '')", migrationIssue)).From("pipelines p").Where(squirrel.Eq{"p.id": id}).QueryRowContext(ctx).Scan(&pipeline.ID, &pipeline.Name, &pipeline.Description, &pipeline.Icon, &pipeline.IconColor, &pipeline.IconBackground, &status, &definitionJSON, &pipeline.PublishedRevision, &created, &updated, &pipeline.MigrationIssue)
	if err != nil {
		return domain.Pipeline{}, fmt.Errorf("get pipeline: %w", err)
	}
	if err := decode(definitionJSON, &pipeline.DraftDefinition); err != nil {
		return domain.Pipeline{}, err
	}
	pipeline.Status = domain.PipelineStatus(status)
	if pipeline.PublishedRevision > 0 {
		var publishedDefinitionJSON string
		if err := statements(s.db).Select("definition").From("pipeline_revisions").Where(squirrel.Eq{
			"pipeline_id": pipeline.ID,
			"revision":    pipeline.PublishedRevision,
		}).QueryRowContext(ctx).Scan(&publishedDefinitionJSON); err != nil {
			return domain.Pipeline{}, fmt.Errorf("get published pipeline definition: %w", err)
		}
		pipeline.HasUnpublishedChanges = definitionJSON != publishedDefinitionJSON
	}
	pipeline.CreatedAt, pipeline.UpdatedAt = parseTime(created), parseTime(updated)
	return pipeline, nil
}

// SaveDraft updates the editable definition without changing live triggers.
func (s *Store) SaveDraft(ctx context.Context, pipeline domain.Pipeline) (domain.Pipeline, error) {
	var status string
	if err := statements(s.db).Select("status").From("pipelines").Where(squirrel.Eq{"id": pipeline.ID}).QueryRowContext(ctx).Scan(&status); err != nil {
		return domain.Pipeline{}, fmt.Errorf("read draft status: %w", err)
	}
	if domain.PipelineStatus(status) == domain.PipelineLegacy {
		return domain.Pipeline{}, fmt.Errorf("legacy pipeline %q is read-only; create a new Blueprint v3 pipeline", pipeline.Name)
	}
	definitionJSON, err := encode(pipeline.DraftDefinition)
	if err != nil {
		return domain.Pipeline{}, err
	}
	pipeline.UpdatedAt = time.Now().UTC()
	pipeline.Icon = strings.TrimSpace(pipeline.Icon)
	if pipeline.Icon == "" {
		pipeline.Icon = "workflow"
	}
	pipeline.IconColor = strings.TrimSpace(pipeline.IconColor)
	if pipeline.IconColor == "" {
		pipeline.IconColor = "#e4e4e7"
	}
	pipeline.IconBackground = strings.TrimSpace(pipeline.IconBackground)
	if pipeline.IconBackground == "" {
		pipeline.IconBackground = "#27272a"
	}
	_, err = statements(s.db).Update("pipelines").Set("name", strings.TrimSpace(pipeline.Name)).Set("description", pipeline.Description).Set("icon", pipeline.Icon).Set("icon_color", pipeline.IconColor).Set("icon_background", pipeline.IconBackground).Set("draft_definition", definitionJSON).Set("updated_at", stamp(pipeline.UpdatedAt)).Where(squirrel.Eq{"id": pipeline.ID}).ExecContext(ctx)
	if err != nil {
		return domain.Pipeline{}, fmt.Errorf("save draft: %w", err)
	}
	return s.GetPipeline(ctx, pipeline.ID)
}

// Publish captures an immutable revision and replaces its active trigger bindings.
func (s *Store) Publish(ctx context.Context, pipeline domain.Pipeline, bindings []domain.TriggerBinding) (domain.Pipeline, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Pipeline{}, fmt.Errorf("start publish transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current int
	if err := statements(tx).Select("published_revision").From("pipelines").Where(squirrel.Eq{"id": pipeline.ID}).QueryRowContext(ctx).Scan(&current); err != nil {
		return domain.Pipeline{}, fmt.Errorf("read pipeline revision: %w", err)
	}
	next := current + 1
	now := time.Now().UTC()
	definitionJSON, err := encode(pipeline.DraftDefinition)
	if err != nil {
		return domain.Pipeline{}, err
	}
	if _, err := statements(tx).Insert("pipeline_revisions").Columns("pipeline_id", "revision", "definition", "published_at").Values(pipeline.ID, next, definitionJSON, stamp(now)).ExecContext(ctx); err != nil {
		return domain.Pipeline{}, fmt.Errorf("save published revision: %w", err)
	}
	priorTrusted := make(map[string]bool)
	priorRows, err := statements(tx).Select("node_id", "trusted").From("trigger_bindings").Where(squirrel.Eq{"pipeline_id": pipeline.ID}).QueryContext(ctx)
	if err != nil {
		return domain.Pipeline{}, fmt.Errorf("read prior trusted state: %w", err)
	}
	for priorRows.Next() {
		var nodeID string
		var trusted bool
		if err := priorRows.Scan(&nodeID, &trusted); err != nil {
			_ = priorRows.Close()
			return domain.Pipeline{}, fmt.Errorf("scan prior trusted state: %w", err)
		}
		if trusted {
			priorTrusted[nodeID] = true
		}
	}
	_ = priorRows.Close()
	if _, err := statements(tx).Delete("trigger_bindings").Where(squirrel.Eq{"pipeline_id": pipeline.ID}).ExecContext(ctx); err != nil {
		return domain.Pipeline{}, fmt.Errorf("remove prior trigger bindings: %w", err)
	}
	for _, binding := range bindings {
		binding.ID = uuid.NewString()
		binding.PipelineID, binding.Revision = pipeline.ID, next
		binding.Enabled = binding.Kind == domain.TriggerButton || binding.Kind == domain.TriggerChat || binding.Kind == domain.TriggerHotkey || binding.Kind == domain.TriggerTwitch
		binding.Trusted = priorTrusted[binding.NodeID]
		binding.CreatedAt, binding.UpdatedAt = now, now
		configJSON, err := encode(binding.Config)
		if err != nil {
			return domain.Pipeline{}, fmt.Errorf("encode trigger binding config: %w", err)
		}
		if _, err := statements(tx).Insert("trigger_bindings").Columns("id", "pipeline_id", "node_id", "revision", "kind", "node_type", "config_json", "label", "icon", "color", "grid_position", "hotkey", "cron", "timezone", "enabled", "trusted", "created_at", "updated_at").Values(binding.ID, binding.PipelineID, binding.NodeID, binding.Revision, binding.Kind, binding.NodeType, configJSON, binding.Label, binding.Icon, binding.Color, binding.GridPosition, binding.Hotkey, binding.Cron, binding.Timezone, binding.Enabled, binding.Trusted, stamp(now), stamp(now)).ExecContext(ctx); err != nil {
			return domain.Pipeline{}, fmt.Errorf("save trigger binding: %w", err)
		}
	}
	if _, err := statements(tx).Delete("permissions").Where(squirrel.Eq{"pipeline_id": pipeline.ID}).ExecContext(ctx); err != nil {
		return domain.Pipeline{}, fmt.Errorf("revoke prior permission grants: %w", err)
	}
	if _, err := statements(tx).Delete("blueprint_migration_issues").Where(squirrel.Eq{"pipeline_id": pipeline.ID}).ExecContext(ctx); err != nil {
		return domain.Pipeline{}, fmt.Errorf("clear resolved migration issues: %w", err)
	}
	if _, err := statements(tx).Update("pipelines").Set("status", domain.PipelineActive).Set("published_revision", next).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": pipeline.ID}).ExecContext(ctx); err != nil {
		return domain.Pipeline{}, fmt.Errorf("activate pipeline: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Pipeline{}, fmt.Errorf("commit publish: %w", err)
	}
	return s.GetPipeline(ctx, pipeline.ID)
}

// PublishedDefinition returns the immutable revision used for trigger execution.
func (s *Store) PublishedDefinition(ctx context.Context, pipelineID string, revision int) (domain.FlowDefinition, error) {
	var definitionJSON string
	err := statements(s.db).Select("definition").From("pipeline_revisions").Where(squirrel.Eq{"pipeline_id": pipelineID, "revision": revision}).QueryRowContext(ctx).Scan(&definitionJSON)
	if err != nil {
		return domain.FlowDefinition{}, fmt.Errorf("get published definition: %w", err)
	}
	var definition domain.FlowDefinition
	if err := decode(definitionJSON, &definition); err != nil {
		return domain.FlowDefinition{}, err
	}
	return definition, nil
}

// CreateFunction stores a new standard global function draft. It remains as a
// compact repository API for callers that do not need creation metadata.
func (s *Store) CreateFunction(ctx context.Context, name string, mode domain.NodeExecutionMode) (domain.CustomFunction, error) {
	return s.CreateFunctionWithRequest(ctx, domain.CreateFunctionRequest{Name: name, Kind: domain.FunctionStandard, Mode: mode})
}

// CreateFunctionWithRequest stores a new global function draft from the
// explicit kind and metadata selected in the function-creation dialog.
func (s *Store) CreateFunctionWithRequest(ctx context.Context, request domain.CreateFunctionRequest) (domain.CustomFunction, error) {
	kind := request.Kind
	if kind == "" {
		kind = domain.FunctionStandard
	}
	if kind != domain.FunctionStandard && kind != domain.FunctionTool {
		return domain.CustomFunction{}, fmt.Errorf("invalid function kind %q", kind)
	}
	mode := request.Mode
	if kind == domain.FunctionTool || mode != domain.NodePure {
		mode = domain.NodeImpure
	}
	now := time.Now().UTC()
	definition := defaultFunctionDefinition(mode)
	function := domain.CustomFunction{ID: uuid.NewString(), Name: strings.TrimSpace(request.Name), Description: strings.TrimSpace(request.Description), Category: "Functions", Icon: "braces", IconColor: "#c4b5fd", IconBackground: "#2e1065", Kind: kind, Mode: mode, Inputs: []domain.FunctionPin{}, Outputs: []domain.FunctionPin{}, DraftDefinition: definition, CreatedAt: now, UpdatedAt: now}
	if kind == domain.FunctionTool {
		function.Icon, function.IconColor, function.IconBackground = "wrench", "#f472b6", "#500724"
	}
	if function.Name == "" {
		function.Name = "Untitled function"
	}
	inputs, err := encode(function.Inputs)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	outputs, err := encode(function.Outputs)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	flow, err := encode(function.DraftDefinition)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	_, err = statements(s.db).Insert("functions").Columns("id", "name", "description", "category", "icon", "icon_color", "icon_background", "kind", "mode", "inputs_json", "outputs_json", "draft_definition", "published_revision", "created_at", "updated_at").Values(function.ID, function.Name, function.Description, function.Category, function.Icon, function.IconColor, function.IconBackground, function.Kind, function.Mode, inputs, outputs, flow, 0, stamp(now), stamp(now)).ExecContext(ctx)
	if err != nil {
		return domain.CustomFunction{}, fmt.Errorf("create function: %w", err)
	}
	return function, nil
}

// ListFunctions returns the global Functions-library cards.
func (s *Store) ListFunctions(ctx context.Context) ([]domain.FunctionSummary, error) {
	rows, err := statements(s.db).Select("id", "name", "description", "category", "icon", "icon_color", "icon_background", "kind", "mode", "published_revision", "updated_at").From("functions").OrderBy("category COLLATE NOCASE", "name COLLATE NOCASE").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list functions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.FunctionSummary, 0)
	for rows.Next() {
		var item domain.FunctionSummary
		var kind, mode, updated string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.Icon, &item.IconColor, &item.IconBackground, &kind, &mode, &item.PublishedRevision, &updated); err != nil {
			return nil, fmt.Errorf("scan function summary: %w", err)
		}
		item.Kind, item.Mode, item.UpdatedAt = domain.FunctionKind(kind), domain.NodeExecutionMode(mode), parseTime(updated)
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetFunction loads an editable global function draft.
func (s *Store) GetFunction(ctx context.Context, id string) (domain.CustomFunction, error) {
	var function domain.CustomFunction
	var kind, mode, inputs, outputs, definition, created, updated string
	err := statements(s.db).Select("id", "name", "description", "category", "icon", "icon_color", "icon_background", "kind", "mode", "inputs_json", "outputs_json", "draft_definition", "published_revision", "created_at", "updated_at").From("functions").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx).Scan(&function.ID, &function.Name, &function.Description, &function.Category, &function.Icon, &function.IconColor, &function.IconBackground, &kind, &mode, &inputs, &outputs, &definition, &function.PublishedRevision, &created, &updated)
	if err != nil {
		return domain.CustomFunction{}, fmt.Errorf("get function: %w", err)
	}
	if err := decode(inputs, &function.Inputs); err != nil {
		return domain.CustomFunction{}, err
	}
	if err := decode(outputs, &function.Outputs); err != nil {
		return domain.CustomFunction{}, err
	}
	if err := decode(definition, &function.DraftDefinition); err != nil {
		return domain.CustomFunction{}, err
	}
	function.Kind, function.Mode, function.CreatedAt, function.UpdatedAt = domain.FunctionKind(kind), domain.NodeExecutionMode(mode), parseTime(created), parseTime(updated)
	return function, nil
}

// SaveFunctionDraft updates a function without affecting call nodes.
func (s *Store) SaveFunctionDraft(ctx context.Context, function domain.CustomFunction) (domain.CustomFunction, error) {
	if strings.TrimSpace(function.Name) == "" {
		return domain.CustomFunction{}, fmt.Errorf("function name is required")
	}
	if function.Kind == "" {
		function.Kind = domain.FunctionStandard
	}
	if function.Kind != domain.FunctionStandard && function.Kind != domain.FunctionTool {
		return domain.CustomFunction{}, fmt.Errorf("invalid function kind %q", function.Kind)
	}
	if function.Kind == domain.FunctionTool {
		function.Mode = domain.NodeImpure
	}
	if strings.TrimSpace(function.Category) == "" {
		function.Category = "Functions"
	}
	inputs, err := encode(function.Inputs)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	outputs, err := encode(function.Outputs)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	flow, err := encode(function.DraftDefinition)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(function.Icon) == "" {
		function.Icon = "braces"
	}
	if strings.TrimSpace(function.IconColor) == "" {
		function.IconColor = "#c4b5fd"
	}
	if strings.TrimSpace(function.IconBackground) == "" {
		function.IconBackground = "#2e1065"
	}
	result, err := statements(s.db).Update("functions").Set("name", strings.TrimSpace(function.Name)).Set("description", function.Description).Set("category", function.Category).Set("icon", function.Icon).Set("icon_color", function.IconColor).Set("icon_background", function.IconBackground).Set("kind", function.Kind).Set("mode", function.Mode).Set("inputs_json", inputs).Set("outputs_json", outputs).Set("draft_definition", flow).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": function.ID}).ExecContext(ctx)
	if err != nil {
		return domain.CustomFunction{}, fmt.Errorf("save function draft: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.CustomFunction{}, fmt.Errorf("function %q not found", function.ID)
	}
	return s.GetFunction(ctx, function.ID)
}

// PublishFunction snapshots the current draft. Call nodes intentionally resolve
// this newest revision on their next run.
func (s *Store) PublishFunction(ctx context.Context, function domain.CustomFunction) (domain.CustomFunction, error) {
	if _, err := s.SaveFunctionDraft(ctx, function); err != nil {
		return domain.CustomFunction{}, err
	}
	function, err := s.GetFunction(ctx, function.ID)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CustomFunction{}, fmt.Errorf("start function publish: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var revision int
	if err := statements(tx).Select("published_revision").From("functions").Where(squirrel.Eq{"id": function.ID}).QueryRowContext(ctx).Scan(&revision); err != nil {
		return domain.CustomFunction{}, fmt.Errorf("read function revision: %w", err)
	}
	metadata, err := encode(struct {
		Name, Description, Category, Icon, IconColor, IconBackground string
		Kind                                                         domain.FunctionKind
		Mode                                                         domain.NodeExecutionMode
		Inputs, Outputs                                              []domain.FunctionPin
	}{function.Name, function.Description, function.Category, function.Icon, function.IconColor, function.IconBackground, function.Kind, function.Mode, function.Inputs, function.Outputs})
	if err != nil {
		return domain.CustomFunction{}, err
	}
	definition, err := encode(function.DraftDefinition)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	next := revision + 1
	now := time.Now().UTC()
	if _, err := statements(tx).Insert("function_revisions").Columns("function_id", "revision", "metadata_json", "definition", "published_at").Values(function.ID, next, metadata, definition, stamp(now)).ExecContext(ctx); err != nil {
		return domain.CustomFunction{}, fmt.Errorf("save function revision: %w", err)
	}
	if _, err := statements(tx).Update("functions").Set("published_revision", next).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": function.ID}).ExecContext(ctx); err != nil {
		return domain.CustomFunction{}, fmt.Errorf("activate function revision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomFunction{}, fmt.Errorf("commit function publish: %w", err)
	}
	return s.GetFunction(ctx, function.ID)
}

// GetPublishedFunction resolves the latest live definition for the engine.
func (s *Store) GetPublishedFunction(ctx context.Context, id string) (domain.CustomFunction, error) {
	function, err := s.GetFunction(ctx, id)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	if function.PublishedRevision < 1 {
		return domain.CustomFunction{}, fmt.Errorf("function %q has not been published", function.Name)
	}
	var metadata, definition string
	err = statements(s.db).Select("metadata_json", "definition").From("function_revisions").Where(squirrel.Eq{"function_id": id, "revision": function.PublishedRevision}).QueryRowContext(ctx).Scan(&metadata, &definition)
	if err != nil {
		return domain.CustomFunction{}, fmt.Errorf("get function revision: %w", err)
	}
	var snapshot struct {
		Name, Description, Category, Icon, IconColor, IconBackground string
		Kind                                                         domain.FunctionKind
		Mode                                                         domain.NodeExecutionMode
		Inputs, Outputs                                              []domain.FunctionPin
	}
	if err := decode(metadata, &snapshot); err != nil {
		return domain.CustomFunction{}, err
	}
	if err := decode(definition, &function.DraftDefinition); err != nil {
		return domain.CustomFunction{}, err
	}
	function.Name, function.Description, function.Category, function.Icon, function.IconColor, function.IconBackground, function.Kind, function.Mode, function.Inputs, function.Outputs = snapshot.Name, snapshot.Description, snapshot.Category, snapshot.Icon, snapshot.IconColor, snapshot.IconBackground, snapshot.Kind, snapshot.Mode, snapshot.Inputs, snapshot.Outputs
	if function.Kind == "" {
		function.Kind = domain.FunctionStandard
	}
	return function, nil
}

// PublishedFunctionDefinitions makes functions discoverable in the node palette.
func (s *Store) PublishedFunctionDefinitions(ctx context.Context) ([]domain.NodeDefinition, error) {
	functions, err := s.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	definitions := make([]domain.NodeDefinition, 0, len(functions))
	for _, summary := range functions {
		if summary.PublishedRevision < 1 {
			continue
		}
		function, err := s.GetPublishedFunction(ctx, summary.ID)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, FunctionNodeDefinition(function))
	}
	return definitions, nil
}

// DeleteFunction refuses to break an existing pipeline or composed function.
func (s *Store) DeleteFunction(ctx context.Context, id string) error {
	var count int
	needle := "%function:" + id + "%"
	pipelineRevisions := sqliteStatements.Select("pipeline_id").From("pipeline_revisions").Where("definition LIKE ?", needle)
	if err := statements(s.db).Select("COUNT(*)").From("pipelines").Where(squirrel.Or{squirrel.Expr("draft_definition LIKE ?", needle), squirrel.Expr("id IN (?)", pipelineRevisions)}).QueryRowContext(ctx).Scan(&count); err != nil {
		return fmt.Errorf("check function dependencies: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("function is used by %d pipeline definition(s)", count)
	}
	functionRevisions := sqliteStatements.Select("function_id").From("function_revisions").Where("function_id != ?", id).Where("definition LIKE ?", needle)
	if err := statements(s.db).Select("COUNT(*)").From("functions").Where(squirrel.Or{squirrel.Expr("id != ? AND draft_definition LIKE ?", id, needle), squirrel.Expr("id IN (?)", functionRevisions)}).QueryRowContext(ctx).Scan(&count); err != nil {
		return fmt.Errorf("check composed function dependencies: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("function is used by %d custom function definition(s)", count)
	}
	result, err := statements(s.db).Delete("functions").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete function: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("function %q not found", id)
	}
	return nil
}

func defaultFunctionDefinition(mode domain.NodeExecutionMode) domain.FlowDefinition {
	definition := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Viewport: domain.Viewport{Zoom: 1}}
	if mode == domain.NodePure {
		definition.Nodes = []domain.FlowNode{{ID: "inputs", Type: "function:input", Position: domain.Position{X: 100, Y: 180}, Data: map[string]any{"config": map[string]any{}}}, {ID: "outputs", Type: "function:output", Position: domain.Position{X: 420, Y: 180}, Data: map[string]any{"config": map[string]any{}}}}
		return definition
	}
	definition.Nodes = []domain.FlowNode{{ID: "entry", Type: "function:entry", Position: domain.Position{X: 100, Y: 180}, Data: map[string]any{"config": map[string]any{}}}, {ID: "return", Type: "function:return", Position: domain.Position{X: 420, Y: 180}, Data: map[string]any{"config": map[string]any{}}}}
	definition.Edges = []domain.FlowEdge{{ID: "entry-return", Source: "entry", SourceHandle: "out", Target: "return", TargetHandle: "in", Kind: domain.PinExec}}
	return definition
}

// FunctionNodeDefinition projects a published function into the generic node
// catalogue used by validation and React Flow.
func FunctionNodeDefinition(function domain.CustomFunction) domain.NodeDefinition {
	if function.Kind == domain.FunctionTool {
		return domain.NodeDefinition{
			Type:        "function:" + function.ID,
			Category:    function.Category,
			Label:       function.Name,
			Description: function.Description,
			Icon:        function.Icon,
			Color:       "#f472b6",
			// Tool functions declare an availability contract, not a graph step.
			// Visual mode prevents a generic Result pin or executable call surface
			// from being inferred by the catalogue normalizer.
			Mode:          domain.NodeVisual,
			Inputs:        []domain.NodePort{},
			Outputs:       []domain.NodePort{{ID: "tool", Label: "Tools", Kind: domain.PinTool, Direction: domain.PinOutput, Color: "#a78bfa"}},
			Fields:        []domain.ConfigField{},
			Capabilities:  []domain.Capability{},
			DefaultConfig: map[string]any{},
			Source:        "function",
		}
	}
	inputs, outputs := make([]domain.NodePort, 0, len(function.Inputs)+1), make([]domain.NodePort, 0, len(function.Outputs)+1)
	if function.Mode == domain.NodeImpure {
		inputs = append(inputs, domain.NodePort{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1})
		outputs = append(outputs, domain.NodePort{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1})
	}
	for _, pin := range function.Inputs {
		typeSpec := pin.Type
		if typeSpec == nil {
			resolved := typespec.FromDataType(pin.DataType)
			typeSpec = &resolved
		}
		inputs = append(inputs, domain.NodePort{ID: pin.ID, Label: pin.Name, Kind: domain.PinData, Direction: domain.PinInput, DataType: pin.DataType, Type: typeSpec, Required: pin.Required, Default: pin.Default, MaxConnections: 1})
	}
	for _, pin := range function.Outputs {
		typeSpec := pin.Type
		if typeSpec == nil {
			resolved := typespec.FromDataType(pin.DataType)
			typeSpec = &resolved
		}
		outputs = append(outputs, domain.NodePort{ID: pin.ID, Label: pin.Name, Kind: domain.PinData, Direction: domain.PinOutput, DataType: pin.DataType, Type: typeSpec, MaxConnections: 1})
	}
	return domain.NodeDefinition{Type: "function:" + function.ID, Category: function.Category, Label: function.Name, Description: function.Description, Icon: function.Icon, Color: "#a78bfa", Mode: function.Mode, Inputs: inputs, Outputs: outputs, Fields: []domain.ConfigField{}, Capabilities: []domain.Capability{}, DefaultConfig: map[string]any{}, Source: "function"}
}

// ListTriggers returns button or cron bindings for the matching product surface.
func (s *Store) ListTriggers(ctx context.Context, kind domain.TriggerKind) ([]domain.TriggerBinding, error) {
	query := statements(s.db).Select("id", "pipeline_id", "node_id", "revision", "kind", "node_type", "config_json", "label", "icon", "color", "grid_position", "hotkey", "cron", "timezone", "enabled", "trusted", "next_run_at", "last_run_at", "last_run_status", "created_at", "updated_at").From("trigger_bindings")
	if kind != "" {
		query = query.Where(squirrel.Eq{"kind": kind})
	}
	rows, err := query.OrderBy("grid_position ASC", "label COLLATE NOCASE ASC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	bindings := make([]domain.TriggerBinding, 0)
	for rows.Next() {
		binding, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

// ListTriggersByPipeline returns all trigger bindings for a specific pipeline.
func (s *Store) ListTriggersByPipeline(ctx context.Context, pipelineID string) ([]domain.TriggerBinding, error) {
	rows, err := statements(s.db).Select("id", "pipeline_id", "node_id", "revision", "kind", "node_type", "config_json", "label", "icon", "color", "grid_position", "hotkey", "cron", "timezone", "enabled", "trusted", "next_run_at", "last_run_at", "last_run_status", "created_at", "updated_at").From("trigger_bindings").Where(squirrel.Eq{"pipeline_id": pipelineID}).OrderBy("updated_at DESC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list triggers by pipeline: %w", err)
	}
	defer func() { _ = rows.Close() }()
	bindings := make([]domain.TriggerBinding, 0)
	for rows.Next() {
		binding, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

// ListAllTriggers returns every active trigger binding for the local API and
// diagnostics surfaces. It deliberately exposes binding metadata only.
func (s *Store) ListAllTriggers(ctx context.Context) ([]domain.TriggerBinding, error) {
	rows, err := statements(s.db).Select("id", "pipeline_id", "node_id", "revision", "kind", "node_type", "config_json", "label", "icon", "color", "grid_position", "hotkey", "cron", "timezone", "enabled", "trusted", "next_run_at", "last_run_at", "last_run_status", "created_at", "updated_at").From("trigger_bindings").OrderBy("updated_at DESC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	bindings := make([]domain.TriggerBinding, 0)
	for rows.Next() {
		binding, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

// GetTrigger returns a selected binding by ID.
func (s *Store) GetTrigger(ctx context.Context, id string) (domain.TriggerBinding, error) {
	row := statements(s.db).Select("id", "pipeline_id", "node_id", "revision", "kind", "node_type", "config_json", "label", "icon", "color", "grid_position", "hotkey", "cron", "timezone", "enabled", "trusted", "next_run_at", "last_run_at", "last_run_status", "created_at", "updated_at").From("trigger_bindings").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx)
	return scanBinding(row)
}

// SetTriggerEnabled changes activation without mutating the pipeline definition.
func (s *Store) SetTriggerEnabled(ctx context.Context, id string, enabled bool) error {
	result, err := statements(s.db).Update("trigger_bindings").Set("enabled", enabled).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("set trigger enabled: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("trigger binding %q not found", id)
	}
	return nil
}

// SetTriggerNextRun updates scheduler metadata without modifying the workflow definition.
func (s *Store) SetTriggerNextRun(ctx context.Context, id string, next time.Time) error {
	_, err := statements(s.db).Update("trigger_bindings").Set("next_run_at", stamp(next)).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("set trigger next run: %w", err)
	}
	return nil
}

// SetTriggerLastRun records a completed trigger execution for the board and schedules view.
func (s *Store) SetTriggerLastRun(ctx context.Context, id string, status domain.RunStatus, when time.Time) error {
	_, err := statements(s.db).Update("trigger_bindings").Set("last_run_at", stamp(when)).Set("last_run_status", status).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("set trigger last run: %w", err)
	}
	return nil
}

// Grant stores trust for a capability scope on the active revision.
func (s *Store) Grant(ctx context.Context, grant domain.PermissionGrant) error {
	_, err := statements(s.db).Replace("permissions").Columns("pipeline_id", "revision", "capability", "scope", "granted_at").Values(grant.PipelineID, grant.Revision, grant.Capability, grant.Scope, stamp(time.Now().UTC())).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("grant permission: %w", err)
	}
	_, err = statements(s.db).Update("trigger_bindings").Set("trusted", true).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"pipeline_id": grant.PipelineID, "revision": grant.Revision}).ExecContext(ctx)
	return err
}

// TrustRevision explicitly permits unattended trigger execution of a published revision.
// Capability-specific grants remain required when the graph requests sensitive work.
func (s *Store) TrustRevision(ctx context.Context, pipelineID string, revision int) error {
	result, err := statements(s.db).Update("trigger_bindings").Set("trusted", true).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"pipeline_id": pipelineID, "revision": revision}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("trust revision: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("no trigger bindings exist for pipeline revision")
	}
	return nil
}

// HasGrant reports whether a scope-independent capability was trusted.
func (s *Store) HasGrant(ctx context.Context, pipelineID string, revision int, capability domain.Capability) (bool, error) {
	var exists int
	permission := sqliteStatements.Select("1").From("permissions").Where(squirrel.Eq{"pipeline_id": pipelineID, "revision": revision, "capability": capability})
	err := statements(s.db).Select().Column(squirrel.Expr("EXISTS(?)", permission)).QueryRowContext(ctx).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return exists == 1, nil
}

// StartExecution records the beginning of a run.
func (s *Store) StartExecution(ctx context.Context, pipelineID, triggerID string) (domain.Execution, error) {
	now := time.Now().UTC()
	execution := domain.Execution{ID: uuid.NewString(), PipelineID: pipelineID, TriggerID: triggerID, Status: domain.RunRunning, StartedAt: now, RunStartedAt: &now}
	_, err := statements(s.db).Insert("executions").Columns("id", "pipeline_id", "trigger_id", "status", "started_at", "run_started_at").Values(execution.ID, execution.PipelineID, execution.TriggerID, execution.Status, stamp(execution.StartedAt), stamp(now)).ExecContext(ctx)
	if err != nil {
		return domain.Execution{}, fmt.Errorf("start execution: %w", err)
	}
	return execution, nil
}

// QueueExecution persists a pending run before it is handed to an owned worker.
func (s *Store) QueueExecution(ctx context.Context, pipelineID, triggerID string) (domain.Execution, error) {
	now := time.Now().UTC()
	execution := domain.Execution{ID: uuid.NewString(), PipelineID: pipelineID, TriggerID: triggerID, Status: domain.RunPending, StartedAt: now, QueuedAt: &now}
	_, err := statements(s.db).Insert("executions").Columns("id", "pipeline_id", "trigger_id", "status", "started_at", "queued_at").Values(execution.ID, execution.PipelineID, execution.TriggerID, execution.Status, stamp(execution.StartedAt), stamp(now)).ExecContext(ctx)
	if err != nil {
		return domain.Execution{}, fmt.Errorf("queue execution: %w", err)
	}
	return execution, nil
}

// MarkExecutionRunning transitions a queued execution when a worker begins it.
func (s *Store) MarkExecutionRunning(ctx context.Context, id string) error {
	result, err := statements(s.db).Update("executions").Set("status", domain.RunRunning).Set("run_started_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": id, "status": domain.RunPending}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("mark execution running: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("queued execution %q is no longer pending", id)
	}
	return nil
}

// CompleteExecution persists redacted node records and final status.
func (s *Store) CompleteExecution(ctx context.Context, execution domain.Execution) error {
	// Update the execution status in its own transaction first. This must
	// succeed even when individual node-run inserts fail, otherwise the
	// execution would stay "running" forever in the DB when a node output
	// cannot be encoded.
	finished := time.Now().UTC()
	if _, err := statements(s.db).Update("executions").Set("status", execution.Status).Set("finished_at", stamp(finished)).Set("error", execution.Error).Where(squirrel.Eq{"id": execution.ID}).ExecContext(ctx); err != nil {
		return fmt.Errorf("complete execution: %w", err)
	}
	// Clear any previously-stored node runs so a re-completion (e.g. after a
	// retry) does not hit a UNIQUE constraint on (execution_id, ordinal).
	if _, err := statements(s.db).Delete("node_runs").Where(squirrel.Eq{"execution_id": execution.ID}).ExecContext(ctx); err != nil {
		return fmt.Errorf("clear stale node runs: %w", err)
	}
	// Insert node runs individually. A failure on one run must not roll back
	// the status update above or discard other runs that encoded successfully.
	for ordinal, run := range execution.NodeRuns {
		input, err := encode(run.Input)
		if err != nil {
			slog.Warn("encode node run input failed; skipping", "executionId", execution.ID, "nodeId", run.NodeID, "err", err)
			continue
		}
		output, err := encode(run.Output)
		if err != nil {
			slog.Warn("encode node run output failed; skipping", "executionId", execution.ID, "nodeId", run.NodeID, "err", err)
			continue
		}
		if _, err := statements(s.db).Insert("node_runs").Columns("execution_id", "ordinal", "node_id", "node_type", "status", "input_json", "output_json", "error", "started_at", "finished_at").Values(execution.ID, ordinal, run.NodeID, run.NodeType, run.Status, input, output, run.Error, stamp(run.StartedAt), stamp(run.FinishedAt)).ExecContext(ctx); err != nil {
			slog.Warn("save node run failed; skipping", "executionId", execution.ID, "nodeId", run.NodeID, "err", err)
			continue
		}
	}
	return nil
}

// ListExecutions returns recent run details for a pipeline.
func (s *Store) ListExecutions(ctx context.Context, pipelineID string, limit int) ([]domain.Execution, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := statements(s.db).Select("id", "pipeline_id", "trigger_id", "status", "started_at", "finished_at", "queued_at", "run_started_at", "error").From("executions").Where(squirrel.Eq{"pipeline_id": pipelineID}).OrderBy("started_at DESC").Limit(uint64(limit)).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	executions := make([]domain.Execution, 0)
	for rows.Next() {
		var execution domain.Execution
		var status, started string
		var finished, queued, runStarted sql.NullString
		if err := rows.Scan(&execution.ID, &execution.PipelineID, &execution.TriggerID, &status, &started, &finished, &queued, &runStarted, &execution.Error); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		applyExecutionTimes(&execution, status, started, finished, queued, runStarted)
		executions = append(executions, execution)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close execution rows: %w", err)
	}
	for index := range executions {
		nodeRuns, err := s.listNodeRuns(ctx, executions[index].ID)
		if err != nil {
			return nil, err
		}
		executions[index].NodeRuns = nodeRuns
	}
	return executions, nil
}

// ListRecentExecutions returns workspace history for API clients and diagnostics.
func (s *Store) ListRecentExecutions(ctx context.Context, limit int) ([]domain.Execution, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := statements(s.db).Select("id", "pipeline_id", "trigger_id", "status", "started_at", "finished_at", "queued_at", "run_started_at", "error").From("executions").OrderBy("started_at DESC").Limit(uint64(limit)).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recent executions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return s.scanExecutions(ctx, rows)
}

// GetExecution returns one execution and its redacted node logs.
func (s *Store) GetExecution(ctx context.Context, id string) (domain.Execution, error) {
	var execution domain.Execution
	var status, started string
	var finished, queued, runStarted sql.NullString
	err := statements(s.db).Select("id", "pipeline_id", "trigger_id", "status", "started_at", "finished_at", "queued_at", "run_started_at", "error").From("executions").Where(squirrel.Eq{"id": id}).QueryRowContext(ctx).Scan(&execution.ID, &execution.PipelineID, &execution.TriggerID, &status, &started, &finished, &queued, &runStarted, &execution.Error)
	if err != nil {
		return domain.Execution{}, fmt.Errorf("get execution: %w", err)
	}
	applyExecutionTimes(&execution, status, started, finished, queued, runStarted)
	nodeRuns, err := s.listNodeRuns(ctx, execution.ID)
	if err != nil {
		return domain.Execution{}, err
	}
	execution.NodeRuns = nodeRuns
	return execution, nil
}

func (s *Store) scanExecutions(ctx context.Context, rows *sql.Rows) ([]domain.Execution, error) {
	executions := make([]domain.Execution, 0)
	for rows.Next() {
		var execution domain.Execution
		var status, started string
		var finished, queued, runStarted sql.NullString
		if err := rows.Scan(&execution.ID, &execution.PipelineID, &execution.TriggerID, &status, &started, &finished, &queued, &runStarted, &execution.Error); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		applyExecutionTimes(&execution, status, started, finished, queued, runStarted)
		executions = append(executions, execution)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range executions {
		nodeRuns, err := s.listNodeRuns(ctx, executions[index].ID)
		if err != nil {
			return nil, err
		}
		executions[index].NodeRuns = nodeRuns
	}
	return executions, nil
}

func applyExecutionTimes(execution *domain.Execution, status, started string, finished, queued, runStarted sql.NullString) {
	execution.Status, execution.StartedAt = domain.RunStatus(status), parseTime(started)
	if finished.Valid {
		value := parseTime(finished.String)
		execution.FinishedAt = &value
	}
	if queued.Valid {
		value := parseTime(queued.String)
		execution.QueuedAt = &value
	}
	if runStarted.Valid {
		value := parseTime(runStarted.String)
		execution.RunStartedAt = &value
	}
}

// CreateReport persists a Markdown document emitted by an executing pipeline.
func (s *Store) CreateReport(ctx context.Context, report domain.Report) (domain.Report, error) {
	report.ID = uuid.NewString()
	report.Title = strings.TrimSpace(report.Title)
	if report.Title == "" {
		report.Title = "Untitled report"
	}
	report.Tags = domain.NormalizeTags(report.Tags)
	tags, err := encode(report.Tags)
	if err != nil {
		return domain.Report{}, fmt.Errorf("encode report tags: %w", err)
	}
	report.CreatedAt = time.Now().UTC()
	_, err = statements(s.db).Insert("reports").Columns("id", "pipeline_id", "execution_id", "node_id", "title", "tags_json", "markdown", "created_at").Values(report.ID, report.PipelineID, report.ExecutionID, report.NodeID, report.Title, tags, report.Markdown, stamp(report.CreatedAt)).ExecContext(ctx)
	if err != nil {
		return domain.Report{}, fmt.Errorf("create report: %w", err)
	}
	// Metrics are deliberately best-effort: producing a report must never fail
	// because local observability storage is temporarily unavailable.
	_ = s.RecordMetricActivity(ctx, domain.MetricActivityEvent{Kind: "report.created", OccurredAt: report.CreatedAt})
	return report, nil
}

// DeleteReport permanently removes one report from the local workspace.
func (s *Store) DeleteReport(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("reports").Where(squirrel.Eq{"id": strings.TrimSpace(id)}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete report: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("report %q not found", id)
	}
	return nil
}

// ListReports returns a newest-first feed with pipeline and execution context.
func (s *Store) ListReports(ctx context.Context, limit int) ([]domain.Report, error) {
	if limit < 1 || limit > 250 {
		limit = 100
	}
	rows, err := statements(s.db).Select("reports.id", "reports.pipeline_id", "pipelines.name", "reports.execution_id", "reports.node_id", "reports.title", "reports.tags_json", "reports.markdown", "reports.created_at", "executions.started_at").From("reports").Join("pipelines ON pipelines.id = reports.pipeline_id").Join("executions ON executions.id = reports.execution_id").OrderBy("reports.created_at DESC").Limit(uint64(limit)).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	reports := make([]domain.Report, 0)
	for rows.Next() {
		var report domain.Report
		var created, executionStarted, tags string
		if err := rows.Scan(&report.ID, &report.PipelineID, &report.PipelineName, &report.ExecutionID, &report.NodeID, &report.Title, &tags, &report.Markdown, &created, &executionStarted); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		if err := decode(tags, &report.Tags); err != nil {
			return nil, fmt.Errorf("decode report tags: %w", err)
		}
		report.Tags = domain.NormalizeTags(report.Tags)
		report.CreatedAt = parseTime(created)
		report.ExecutionStartedAt = parseTime(executionStarted)
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reports: %w", err)
	}
	return reports, nil
}

// GetReport returns one local report for assistant tools and report links.
func (s *Store) GetReport(ctx context.Context, id string) (domain.Report, error) {
	row := statements(s.db).Select("reports.id", "reports.pipeline_id", "pipelines.name", "reports.execution_id", "reports.node_id", "reports.title", "reports.tags_json", "reports.markdown", "reports.created_at", "executions.started_at").From("reports").Join("pipelines ON pipelines.id = reports.pipeline_id").Join("executions ON executions.id = reports.execution_id").Where(squirrel.Eq{"reports.id": strings.TrimSpace(id)}).QueryRowContext(ctx)
	var report domain.Report
	var created, executionStarted, tags string
	if err := row.Scan(&report.ID, &report.PipelineID, &report.PipelineName, &report.ExecutionID, &report.NodeID, &report.Title, &tags, &report.Markdown, &created, &executionStarted); err != nil {
		return domain.Report{}, fmt.Errorf("get report: %w", err)
	}
	if err := decode(tags, &report.Tags); err != nil {
		return domain.Report{}, fmt.Errorf("decode report tags: %w", err)
	}
	report.Tags = domain.NormalizeTags(report.Tags)
	report.CreatedAt, report.ExecutionStartedAt = parseTime(created), parseTime(executionStarted)
	return report, nil
}

// CreateChatConversation persists an independent local transcript.
func (s *Store) CreateChatConversation(ctx context.Context, conversation domain.ChatConversation) (domain.ChatConversation, error) {
	if conversation.Mode != domain.ChatModeModel && conversation.Mode != domain.ChatModePipeline {
		return domain.ChatConversation{}, fmt.Errorf("invalid chat mode %q", conversation.Mode)
	}
	conversation.ID = uuid.NewString()
	conversation.Title = strings.TrimSpace(conversation.Title)
	if conversation.Title == "" {
		if conversation.Mode == domain.ChatModePipeline {
			conversation.Title = "Pipeline chat"
		} else {
			conversation.Title = "New chat"
		}
	}
	if conversation.Mode == domain.ChatModePipeline && strings.TrimSpace(conversation.TriggerBindingID) == "" {
		return domain.ChatConversation{}, fmt.Errorf("pipeline chat requires a chat trigger")
	}
	if conversation.ActionPolicy != domain.ChatActionAlways {
		conversation.ActionPolicy = domain.ChatActionAsk
	}
	now := time.Now().UTC()
	conversation.CreatedAt, conversation.UpdatedAt = now, now
	_, err := statements(s.db).Insert("chat_conversations").Columns("id", "mode", "title", "pipeline_id", "trigger_binding_id", "action_policy", "created_at", "updated_at").Values(conversation.ID, conversation.Mode, conversation.Title, conversation.PipelineID, conversation.TriggerBindingID, conversation.ActionPolicy, stamp(now), stamp(now)).ExecContext(ctx)
	if err != nil {
		return domain.ChatConversation{}, fmt.Errorf("create chat conversation: %w", err)
	}
	_ = s.RecordMetricActivity(ctx, domain.MetricActivityEvent{Kind: "chat.conversation", Outcome: string(conversation.Mode), OccurredAt: conversation.CreatedAt})
	return conversation, nil
}

// ListChatConversations returns most recently active conversations first.
func (s *Store) ListChatConversations(ctx context.Context) ([]domain.ChatConversation, error) {
	rows, err := statements(s.db).Select("id", "mode", "title", "pipeline_id", "trigger_binding_id", "action_policy", "created_at", "updated_at").From("chat_conversations").OrderBy("updated_at DESC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatConversation, 0)
	for rows.Next() {
		item, err := scanChatConversation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetChatConversation loads one persisted conversation.
func (s *Store) GetChatConversation(ctx context.Context, id string) (domain.ChatConversation, error) {
	row := statements(s.db).Select("id", "mode", "title", "pipeline_id", "trigger_binding_id", "action_policy", "created_at", "updated_at").From("chat_conversations").Where(squirrel.Eq{"id": strings.TrimSpace(id)}).QueryRowContext(ctx)
	var item domain.ChatConversation
	var created, updated string
	if err := row.Scan(&item.ID, &item.Mode, &item.Title, &item.PipelineID, &item.TriggerBindingID, &item.ActionPolicy, &created, &updated); err != nil {
		return domain.ChatConversation{}, fmt.Errorf("get chat conversation: %w", err)
	}
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}

// SaveChatConversation updates the user-managed title and action policy.
func (s *Store) SaveChatConversation(ctx context.Context, conversation domain.ChatConversation) (domain.ChatConversation, error) {
	conversation.Title = strings.TrimSpace(conversation.Title)
	if conversation.Title == "" {
		return domain.ChatConversation{}, fmt.Errorf("chat title is required")
	}
	if conversation.ActionPolicy != domain.ChatActionAlways {
		conversation.ActionPolicy = domain.ChatActionAsk
	}
	now := time.Now().UTC()
	result, err := statements(s.db).Update("chat_conversations").Set("title", conversation.Title).Set("action_policy", conversation.ActionPolicy).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": conversation.ID}).ExecContext(ctx)
	if err != nil {
		return domain.ChatConversation{}, fmt.Errorf("save chat conversation: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.ChatConversation{}, fmt.Errorf("chat conversation %q not found", conversation.ID)
	}
	return s.GetChatConversation(ctx, conversation.ID)
}

// DeleteChatConversation permanently removes its local transcript and runs.
func (s *Store) DeleteChatConversation(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("chat_conversations").Where(squirrel.Eq{"id": strings.TrimSpace(id)}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete chat conversation: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("chat conversation %q not found", id)
	}
	return nil
}

// CreateChatMessage appends a redacted-safe transcript item and refreshes its
// conversation ordering.
func (s *Store) CreateChatMessage(ctx context.Context, message domain.ChatMessage) (domain.ChatMessage, error) {
	message.ID = uuid.NewString()
	message.Content = strings.TrimSpace(message.Content)
	if message.Role == "" {
		return domain.ChatMessage{}, fmt.Errorf("chat message role is required")
	}
	if message.ToolCalls == nil {
		message.ToolCalls = make([]domain.ChatToolCall, 0)
	}
	toolCalls, err := encode(message.ToolCalls)
	if err != nil {
		return domain.ChatMessage{}, fmt.Errorf("encode chat tool calls: %w", err)
	}
	message.CreatedAt = time.Now().UTC()
	_, err = statements(s.db).Insert("chat_messages").Columns("id", "conversation_id", "chat_run_id", "role", "content", "tool_call_id", "tool_name", "tool_calls_json", "created_at").Values(message.ID, message.ConversationID, message.ChatRunID, message.Role, message.Content, message.ToolCallID, message.ToolName, toolCalls, stamp(message.CreatedAt)).ExecContext(ctx)
	if err != nil {
		return domain.ChatMessage{}, fmt.Errorf("create chat message: %w", err)
	}
	_, _ = statements(s.db).Update("chat_conversations").Set("updated_at", stamp(message.CreatedAt)).Where(squirrel.Eq{"id": message.ConversationID}).ExecContext(ctx)
	_ = s.RecordMetricActivity(ctx, domain.MetricActivityEvent{Kind: "chat.message." + string(message.Role), OccurredAt: message.CreatedAt})
	return message, nil
}

// ListChatMessages returns chronological transcript entries. A zero limit is
// intentionally bounded to protect prompt construction from unbounded history.
func (s *Store) ListChatMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	recentMessages := sqliteStatements.Select("rowid AS ordinal", "id", "conversation_id", "chat_run_id", "role", "content", "tool_call_id", "tool_name", "tool_calls_json", "created_at").From("chat_messages").Where(squirrel.Eq{"conversation_id": strings.TrimSpace(conversationID)}).OrderBy("created_at DESC", "rowid DESC").Limit(uint64(limit))
	rows, err := statements(s.db).Select("id", "conversation_id", "chat_run_id", "role", "content", "tool_call_id", "tool_name", "tool_calls_json", "created_at").FromSelect(recentMessages, "recent_messages").OrderBy("created_at ASC", "ordinal ASC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatMessage, 0)
	for rows.Next() {
		var item domain.ChatMessage
		var toolCalls, created string
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.ChatRunID, &item.Role, &item.Content, &item.ToolCallID, &item.ToolName, &toolCalls, &created); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		if err := decode(toolCalls, &item.ToolCalls); err != nil {
			return nil, fmt.Errorf("decode chat tool calls: %w", err)
		}
		item.CreatedAt = parseTime(created)
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateChatRun creates a visible unit of work with the default status text.
func (s *Store) CreateChatRun(ctx context.Context, conversationID string) (domain.ChatRun, error) {
	run := domain.ChatRun{ID: uuid.NewString(), ConversationID: strings.TrimSpace(conversationID), Status: domain.RunPending, StatusText: "Working", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	_, err := statements(s.db).Insert("chat_runs").Columns("id", "conversation_id", "execution_id", "status", "status_text", "error", "created_at", "updated_at").Values(run.ID, run.ConversationID, "", run.Status, run.StatusText, "", stamp(run.CreatedAt), stamp(run.UpdatedAt)).ExecContext(ctx)
	if err != nil {
		return domain.ChatRun{}, fmt.Errorf("create chat run: %w", err)
	}
	_ = s.RecordMetricActivity(ctx, domain.MetricActivityEvent{Kind: "chat.run", OccurredAt: run.CreatedAt})
	return run, nil
}

// GetChatRun returns a visible work item.
func (s *Store) GetChatRun(ctx context.Context, id string) (domain.ChatRun, error) {
	row := statements(s.db).Select("id", "conversation_id", "execution_id", "status", "status_text", "error", "created_at", "updated_at").From("chat_runs").Where(squirrel.Eq{"id": strings.TrimSpace(id)}).QueryRowContext(ctx)
	var item domain.ChatRun
	var created, updated string
	if err := row.Scan(&item.ID, &item.ConversationID, &item.ExecutionID, &item.Status, &item.StatusText, &item.Error, &created, &updated); err != nil {
		return domain.ChatRun{}, fmt.Errorf("get chat run: %w", err)
	}
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}

// ListChatRuns returns recent work in a conversation, newest first.
func (s *Store) ListChatRuns(ctx context.Context, conversationID string) ([]domain.ChatRun, error) {
	rows, err := statements(s.db).Select("id", "conversation_id", "execution_id", "status", "status_text", "error", "created_at", "updated_at").From("chat_runs").Where(squirrel.Eq{"conversation_id": strings.TrimSpace(conversationID)}).OrderBy("created_at DESC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatRun, 0)
	for rows.Next() {
		var item domain.ChatRun
		var created, updated string
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.ExecutionID, &item.Status, &item.StatusText, &item.Error, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan chat run: %w", err)
		}
		item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateChatRun changes the status visible in the chat feed.
func (s *Store) UpdateChatRun(ctx context.Context, runID string, status domain.RunStatus, statusText, executionID, runError string) error {
	now := time.Now().UTC()
	result, err := statements(s.db).Update("chat_runs").Set("status", status).Set("status_text", strings.TrimSpace(statusText)).Set("execution_id", squirrel.Expr("CASE WHEN ? = '' THEN execution_id ELSE ? END", executionID, executionID)).Set("error", strings.TrimSpace(runError)).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": strings.TrimSpace(runID)}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("update chat run: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("chat run %q not found", runID)
	}
	return nil
}

// AddChatRunEvent appends a compact disclosure item without mutating messages.
func (s *Store) AddChatRunEvent(ctx context.Context, event domain.ChatRunEvent) (domain.ChatRunEvent, error) {
	event.ID = uuid.NewString()
	event.Summary = strings.TrimSpace(event.Summary)
	if event.Summary == "" {
		return domain.ChatRunEvent{}, fmt.Errorf("chat activity summary is required")
	}
	event.CreatedAt = time.Now().UTC()
	_, err := statements(s.db).Insert("chat_run_events").Columns("id", "chat_run_id", "kind", "summary", "detail", "status", "created_at").Values(event.ID, event.ChatRunID, event.Kind, event.Summary, event.Detail, event.Status, stamp(event.CreatedAt)).ExecContext(ctx)
	if err != nil {
		return domain.ChatRunEvent{}, fmt.Errorf("create chat activity: %w", err)
	}
	return event, nil
}

// ListChatRunEvents returns activity in execution order.
func (s *Store) ListChatRunEvents(ctx context.Context, chatRunID string) ([]domain.ChatRunEvent, error) {
	rows, err := statements(s.db).Select("id", "chat_run_id", "kind", "summary", "detail", "status", "created_at").From("chat_run_events").Where(squirrel.Eq{"chat_run_id": strings.TrimSpace(chatRunID)}).OrderBy("created_at ASC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat activity: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatRunEvent, 0)
	for rows.Next() {
		var item domain.ChatRunEvent
		var created string
		if err := rows.Scan(&item.ID, &item.ChatRunID, &item.Kind, &item.Summary, &item.Detail, &item.Status, &created); err != nil {
			return nil, fmt.Errorf("scan chat activity: %w", err)
		}
		item.CreatedAt = parseTime(created)
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateChatApproval persists a paused model tool call for the app-owned
// confirmation dialog. Duplicate calls remain distinct audit entries.
func (s *Store) CreateChatApproval(ctx context.Context, approval domain.ChatApproval) (domain.ChatApproval, error) {
	approval.ID = uuid.NewString()
	approval.Status = "pending"
	approval.CreatedAt = time.Now().UTC()
	encoded, err := encode(approval.ToolCall)
	if err != nil {
		return domain.ChatApproval{}, fmt.Errorf("encode chat approval: %w", err)
	}
	_, err = statements(s.db).Insert("chat_approvals").Columns("id", "conversation_id", "chat_run_id", "tool_call_json", "status", "created_at", "resolved_at").Values(approval.ID, approval.ConversationID, approval.ChatRunID, encoded, approval.Status, stamp(approval.CreatedAt), nil).ExecContext(ctx)
	if err != nil {
		return domain.ChatApproval{}, fmt.Errorf("create chat approval: %w", err)
	}
	return approval, nil
}

// ListPendingChatApprovals returns unresolved confirmations for one transcript.
func (s *Store) ListPendingChatApprovals(ctx context.Context, conversationID string) ([]domain.ChatApproval, error) {
	rows, err := statements(s.db).Select("id", "conversation_id", "chat_run_id", "tool_call_json", "status", "created_at", "resolved_at").From("chat_approvals").Where(squirrel.Eq{"conversation_id": strings.TrimSpace(conversationID), "status": "pending"}).OrderBy("created_at ASC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatApproval, 0)
	for rows.Next() {
		item, err := scanChatApproval(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetChatApproval resolves a single persisted confirmation request.
func (s *Store) GetChatApproval(ctx context.Context, id string) (domain.ChatApproval, error) {
	row := statements(s.db).Select("id", "conversation_id", "chat_run_id", "tool_call_json", "status", "created_at", "resolved_at").From("chat_approvals").Where(squirrel.Eq{"id": strings.TrimSpace(id)}).QueryRowContext(ctx)
	return scanChatApproval(row)
}

// ResolveChatApproval atomically marks a pending request and returns it.
func (s *Store) ResolveChatApproval(ctx context.Context, id string, approved bool) (domain.ChatApproval, error) {
	status := "denied"
	if approved {
		status = "approved"
	}
	now := time.Now().UTC()
	result, err := statements(s.db).Update("chat_approvals").Set("status", status).Set("resolved_at", stamp(now)).Where(squirrel.Eq{"id": strings.TrimSpace(id), "status": "pending"}).ExecContext(ctx)
	if err != nil {
		return domain.ChatApproval{}, fmt.Errorf("resolve chat approval: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.ChatApproval{}, fmt.Errorf("chat approval %q is no longer pending", id)
	}
	return s.GetChatApproval(ctx, id)
}

// CancelChatApprovalsForRun retires outstanding confirmations when a user
// stops a turn. A stopped turn must never be resumed by a stale dialog.
func (s *Store) CancelChatApprovalsForRun(ctx context.Context, chatRunID string) error {
	now := time.Now().UTC()
	_, err := statements(s.db).Update("chat_approvals").Set("status", "cancelled").Set("resolved_at", stamp(now)).Where(squirrel.Eq{"chat_run_id": strings.TrimSpace(chatRunID), "status": "pending"}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("cancel chat approvals: %w", err)
	}
	return nil
}

// HasChatToolGrant reports whether a conversation's explicit always-allow
// choice still matches the published revision targeted by a pipeline tool.
func (s *Store) HasChatToolGrant(ctx context.Context, conversationID, toolName, targetID string, revision int) (bool, error) {
	var found int
	grant := sqliteStatements.Select("1").From("chat_tool_grants").Where(squirrel.Eq{"conversation_id": strings.TrimSpace(conversationID), "tool_name": strings.TrimSpace(toolName), "target_id": strings.TrimSpace(targetID), "revision": revision})
	err := statements(s.db).Select().Column(squirrel.Expr("EXISTS(?)", grant)).QueryRowContext(ctx).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("check chat tool grant: %w", err)
	}
	return found == 1, nil
}

// SaveChatToolGrant records the exact published revision accepted by a user.
// Re-publishing a target naturally invalidates the matching lookup.
func (s *Store) SaveChatToolGrant(ctx context.Context, conversationID, toolName, targetID string, revision int) error {
	_, err := statements(s.db).Insert("chat_tool_grants").Columns("conversation_id", "tool_name", "target_id", "revision", "granted_at").Values(strings.TrimSpace(conversationID), strings.TrimSpace(toolName), strings.TrimSpace(targetID), revision, stamp(time.Now().UTC())).Suffix("ON CONFLICT(conversation_id, tool_name, target_id) DO UPDATE SET revision = excluded.revision, granted_at = excluded.granted_at").ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("save chat tool grant: %w", err)
	}
	return nil
}

// AppendChatReply implements pipeline.ChatWriter for Reply to Chat nodes.
func (s *Store) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	var conversationID string
	if err := statements(s.db).Select("conversation_id").From("chat_runs").Where(squirrel.Eq{"id": strings.TrimSpace(chatRunID)}).QueryRowContext(ctx).Scan(&conversationID); err != nil {
		return domain.ChatMessage{}, fmt.Errorf("resolve chat reply target: %w", err)
	}
	message, err := s.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversationID, ChatRunID: chatRunID, Role: domain.ChatRoleAssistant, Content: content})
	if err != nil {
		return domain.ChatMessage{}, err
	}
	_, _ = s.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: chatRunID, Kind: "reply", Summary: "Sent chat reply", Detail: message.Content, Status: domain.RunCompleted})
	return message, nil
}

// UpdateChatStatus implements pipeline.ChatWriter for Update Chat Status nodes.
func (s *Store) UpdateChatStatus(ctx context.Context, chatRunID, statusText string) error {
	if strings.TrimSpace(statusText) == "" {
		statusText = "Working"
	}
	if err := s.UpdateChatRun(ctx, chatRunID, domain.RunRunning, statusText, "", ""); err != nil {
		return err
	}
	_, _ = s.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: chatRunID, Kind: "status", Summary: statusText, Status: domain.RunRunning})
	return nil
}

// ReadChatHistory implements pipeline.ChatWriter for pure history nodes.
func (s *Store) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	return s.ListChatMessages(ctx, chatID, limit)
}

// ListChatPipelines projects active published chat trigger bindings for the picker.
func (s *Store) ListChatPipelines(ctx context.Context) ([]domain.ChatPipeline, error) {
	rows, err := statements(s.db).Select("trigger_bindings.id", "trigger_bindings.pipeline_id", "pipelines.name", "trigger_bindings.label", "trigger_bindings.icon", "trigger_bindings.color", "trigger_bindings.revision").From("trigger_bindings").Join("pipelines ON pipelines.id = trigger_bindings.pipeline_id").Where(squirrel.Eq{"trigger_bindings.kind": domain.TriggerChat, "trigger_bindings.enabled": true, "pipelines.status": domain.PipelineActive}).OrderBy("pipelines.name COLLATE NOCASE", "trigger_bindings.label COLLATE NOCASE").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chat pipelines: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.ChatPipeline, 0)
	for rows.Next() {
		var item domain.ChatPipeline
		if err := rows.Scan(&item.BindingID, &item.PipelineID, &item.PipelineName, &item.Label, &item.Icon, &item.Color, &item.Revision); err != nil {
			return nil, fmt.Errorf("scan chat pipeline: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) listNodeRuns(ctx context.Context, executionID string) ([]domain.NodeRun, error) {
	rows, err := statements(s.db).Select("node_id", "node_type", "status", "input_json", "output_json", "error", "started_at", "finished_at").From("node_runs").Where(squirrel.Eq{"execution_id": executionID}).OrderBy("ordinal").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list node runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	nodeRuns := make([]domain.NodeRun, 0)
	for rows.Next() {
		var nodeRun domain.NodeRun
		var status, input, output, started, finished string
		if err := rows.Scan(&nodeRun.NodeID, &nodeRun.NodeType, &status, &input, &output, &nodeRun.Error, &started, &finished); err != nil {
			return nil, fmt.Errorf("scan node run: %w", err)
		}
		if err := decode(input, &nodeRun.Input); err != nil {
			return nil, fmt.Errorf("decode node run input: %w", err)
		}
		if err := decode(output, &nodeRun.Output); err != nil {
			return nil, fmt.Errorf("decode node run output: %w", err)
		}
		nodeRun.Status = domain.RunStatus(status)
		nodeRun.StartedAt, nodeRun.FinishedAt = parseTime(started), parseTime(finished)
		nodeRuns = append(nodeRuns, nodeRun)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node runs: %w", err)
	}
	return nodeRuns, nil
}

// PurgeExecutions deletes old execution history without touching definitions.
func (s *Store) PurgeExecutions(ctx context.Context, retentionDays int) error {
	if retentionDays < 1 {
		retentionDays = 30
	}
	_, err := statements(s.db).Delete("executions").Where("started_at < ?", stamp(time.Now().UTC().AddDate(0, 0, -retentionDays))).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("purge executions: %w", err)
	}
	return nil
}

// LoadSettings returns persisted application settings or safe defaults.
func (s *Store) LoadSettings(ctx context.Context, pluginDirectory string) (domain.Settings, error) {
	settings := domain.Settings{
		Language:             localization.English,
		DefaultProviderID:    "ollama-local",
		RetentionDays:        30,
		WebhookPort:          7878,
		PluginDirectory:      pluginDirectory,
		MaxConcurrentRuns:    2,
		MaxConcurrentLLMRuns: 1,
		Metrics: domain.MetricsSettings{
			DetailRetentionDays:   30,
			RollupRetentionDays:   365,
			SampleIntervalSeconds: 30,
			PriceRates:            []domain.ModelPriceRate{},
		},
		API: domain.APISettings{
			BindAddress:    "127.0.0.1",
			Port:           7878,
			AuthMode:       domain.APIAuthToken,
			AllowedOrigins: []string{},
		},
		LlamaRuntime: domain.LlamaRuntimeSettings{Mode: domain.RuntimeAuto, ContextSize: 8192},
		Providers: []domain.ProviderConfig{{
			ID: "ollama-local", Name: "Local Ollama", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true,
		}},
		Twitch: domain.TwitchSettings{Identities: []domain.TwitchIdentity{}},
	}
	var value string
	err := statements(s.db).Select("value").From("settings").Where(squirrel.Eq{"key": "app"}).QueryRowContext(ctx).Scan(&value)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}
	if err := decode(value, &settings); err != nil {
		return domain.Settings{}, err
	}
	if settings.RetentionDays < 1 {
		settings.RetentionDays = 30
	}
	settings.Language = localization.Normalize(settings.Language)
	if settings.MaxConcurrentRuns < 1 {
		settings.MaxConcurrentRuns = 2
	}
	if settings.MaxConcurrentLLMRuns < 1 {
		settings.MaxConcurrentLLMRuns = 1
	}
	normalizeMetricsSettings(&settings.Metrics)
	if settings.LlamaRuntime.Mode == "" {
		settings.LlamaRuntime.Mode = domain.RuntimeAuto
	}
	if settings.LlamaRuntime.ContextSize < 1024 {
		settings.LlamaRuntime.ContextSize = 8192
	}
	if settings.API.Port == 0 {
		settings.API.Port = settings.WebhookPort
		if settings.API.Port == 0 {
			settings.API.Port = 7878
		}
	}
	if settings.API.BindAddress == "" {
		settings.API.BindAddress = "127.0.0.1"
	}
	if settings.API.AuthMode == "" {
		settings.API.AuthMode = domain.APIAuthToken
	}
	if settings.API.AllowedOrigins == nil {
		settings.API.AllowedOrigins = []string{}
	}
	if settings.PluginDirectory == "" {
		settings.PluginDirectory = pluginDirectory
	}
	if len(settings.Providers) == 0 {
		settings.Providers = []domain.ProviderConfig{{
			ID: "ollama-local", Name: "Local Ollama", Kind: domain.ProviderOllama, BaseURL: "http://127.0.0.1:11434", Enabled: true,
		}}
	}
	if settings.DefaultProviderID == "" {
		settings.DefaultProviderID = settings.Providers[0].ID
	}
	if settings.Twitch.Identities == nil {
		settings.Twitch.Identities = []domain.TwitchIdentity{}
	}
	return settings, nil
}

func normalizeMetricsSettings(settings *domain.MetricsSettings) {
	if settings.DetailRetentionDays < 1 {
		settings.DetailRetentionDays = 30
	}
	if settings.RollupRetentionDays < settings.DetailRetentionDays {
		settings.RollupRetentionDays = 365
	}
	if settings.SampleIntervalSeconds < 10 {
		settings.SampleIntervalSeconds = 30
	}
	if settings.SampleIntervalSeconds > 300 {
		settings.SampleIntervalSeconds = 300
	}
	if settings.PriceRates == nil {
		settings.PriceRates = []domain.ModelPriceRate{}
	}
}

// SaveSettings atomically replaces the small app-settings document.
func (s *Store) SaveSettings(ctx context.Context, settings domain.Settings) error {
	value, err := encode(settings)
	if err != nil {
		return err
	}
	_, err = statements(s.db).Replace("settings").Columns("key", "value").Values("app", value).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

type bindingScanner interface{ Scan(...any) error }

type chatConversationScanner interface{ Scan(...any) error }

type chatApprovalScanner interface{ Scan(...any) error }

func scanChatConversation(scanner chatConversationScanner) (domain.ChatConversation, error) {
	var conversation domain.ChatConversation
	var created, updated string
	if err := scanner.Scan(&conversation.ID, &conversation.Mode, &conversation.Title, &conversation.PipelineID, &conversation.TriggerBindingID, &conversation.ActionPolicy, &created, &updated); err != nil {
		return domain.ChatConversation{}, fmt.Errorf("scan chat conversation: %w", err)
	}
	conversation.CreatedAt, conversation.UpdatedAt = parseTime(created), parseTime(updated)
	return conversation, nil
}

func scanChatApproval(scanner chatApprovalScanner) (domain.ChatApproval, error) {
	var approval domain.ChatApproval
	var toolCall, created string
	var resolved sql.NullString
	if err := scanner.Scan(&approval.ID, &approval.ConversationID, &approval.ChatRunID, &toolCall, &approval.Status, &created, &resolved); err != nil {
		return domain.ChatApproval{}, fmt.Errorf("scan chat approval: %w", err)
	}
	if err := decode(toolCall, &approval.ToolCall); err != nil {
		return domain.ChatApproval{}, fmt.Errorf("decode chat approval: %w", err)
	}
	approval.CreatedAt = parseTime(created)
	if resolved.Valid {
		value := parseTime(resolved.String)
		approval.ResolvedAt = &value
	}
	return approval, nil
}

func scanBinding(scanner bindingScanner) (domain.TriggerBinding, error) {
	var binding domain.TriggerBinding
	var kind, nodeType, configJSON, lastStatus, created, updated string
	var enabled, trusted int
	var nextRun, lastRun sql.NullString
	err := scanner.Scan(&binding.ID, &binding.PipelineID, &binding.NodeID, &binding.Revision, &kind, &nodeType, &configJSON, &binding.Label, &binding.Icon, &binding.Color, &binding.GridPosition, &binding.Hotkey, &binding.Cron, &binding.Timezone, &enabled, &trusted, &nextRun, &lastRun, &lastStatus, &created, &updated)
	if err != nil {
		return domain.TriggerBinding{}, fmt.Errorf("scan trigger binding: %w", err)
	}
	binding.Kind, binding.NodeType, binding.Enabled, binding.Trusted = domain.TriggerKind(kind), nodeType, enabled == 1, trusted == 1
	binding.Config = map[string]any{}
	if strings.TrimSpace(configJSON) != "" && decode(configJSON, &binding.Config) != nil {
		return domain.TriggerBinding{}, fmt.Errorf("decode trigger binding configuration")
	}
	binding.LastRunStatus, binding.CreatedAt, binding.UpdatedAt = domain.RunStatus(lastStatus), parseTime(created), parseTime(updated)
	if nextRun.Valid {
		value := parseTime(nextRun.String)
		binding.NextRunAt = &value
	}
	if lastRun.Valid {
		value := parseTime(lastRun.String)
		binding.LastRunAt = &value
	}
	return binding, nil
}

func encode(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode JSON: %w", err)
	}
	return string(data), nil
}

func decode(value string, destination any) error {
	if err := json.Unmarshal([]byte(value), destination); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func stamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
