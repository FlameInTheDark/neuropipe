package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

const blueprintV3MigrationKey = "migration.blueprint-v3"

// migrateBlueprintV3 upgrades editable v2 drafts to the strict wire contract.
// Published revisions remain immutable. Every migrated pipeline is returned to
// draft, its triggers are disabled, and its prior trust is revoked; this avoids
// executing a formerly-permissive graph under the new type rules.
func (s *Store) migrateBlueprintV3(ctx context.Context) error {
	var completed string
	err := statements(s.db).Select("value").From("settings").Where(squirrel.Eq{"key": blueprintV3MigrationKey}).QueryRowContext(ctx).Scan(&completed)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("read Blueprint v3 migration marker: %w", err)
	}

	rows, err := statements(s.db).Select("id", "draft_definition").From("pipelines").Where("draft_definition LIKE ?", "%\"schemaVersion\":2%").QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("scan Blueprint v2 drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		id         string
		definition domain.FlowDefinition
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		var raw string
		if err := rows.Scan(&item.id, &raw); err != nil {
			return fmt.Errorf("read Blueprint v2 draft: %w", err)
		}
		if err := decode(raw, &item.definition); err != nil {
			return fmt.Errorf("decode Blueprint v2 draft %q: %w", item.id, err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("scan Blueprint v2 drafts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close Blueprint v2 draft scan: %w", err)
	}
	functionRows, err := statements(s.db).Select("id", "draft_definition").From("functions").Where("draft_definition LIKE ?", "%\"schemaVersion\":2%").QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("scan Blueprint v2 function drafts: %w", err)
	}
	defer func() { _ = functionRows.Close() }()
	type functionCandidate struct {
		id         string
		definition domain.FlowDefinition
	}
	functions := make([]functionCandidate, 0)
	for functionRows.Next() {
		var item functionCandidate
		var raw string
		if err := functionRows.Scan(&item.id, &raw); err != nil {
			return fmt.Errorf("read Blueprint v2 function draft: %w", err)
		}
		if err := decode(raw, &item.definition); err != nil {
			return fmt.Errorf("decode Blueprint v2 function draft %q: %w", item.id, err)
		}
		functions = append(functions, item)
	}
	if err := functionRows.Err(); err != nil {
		return fmt.Errorf("scan Blueprint v2 function drafts: %w", err)
	}
	if err := functionRows.Close(); err != nil {
		return fmt.Errorf("close Blueprint v2 function draft scan: %w", err)
	}
	if len(candidates) == 0 && len(functions) == 0 {
		_, err := statements(s.db).Insert("settings").Columns("key", "value").Values(blueprintV3MigrationKey, stamp(time.Now().UTC())).ExecContext(ctx)
		return err
	}
	if err := s.backupDatabase(ctx, "pre-blueprint-v3"); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start Blueprint v3 migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	for _, candidate := range candidates {
		definition := candidate.definition
		definition.SchemaVersion = domain.GraphSchemaV3
		issues := migrateV2ConfigurationValues(&definition)
		if hasV2DataEdges(definition) {
			// V2 did not retain enough static type information to prove every old
			// data wire safe. Do not guess: the editor surfaces this persisted
			// issue and users must review the draft before publishing again.
			issues = append(issues, "Blueprint v3 review required: reconnect or add Cast/Type Assert nodes for every typed data wire.")
		}
		encoded, err := encode(definition)
		if err != nil {
			return fmt.Errorf("encode Blueprint v3 draft %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Update("pipelines").Set("draft_definition", encoded).Set("status", domain.PipelineDraft).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("save Blueprint v3 draft %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Update("trigger_bindings").Set("enabled", false).Set("trusted", false).Set("updated_at", stamp(now)).Where(squirrel.Eq{"pipeline_id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("pause migrated triggers %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Delete("permissions").Where(squirrel.Eq{"pipeline_id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("revoke migrated trust %q: %w", candidate.id, err)
		}
		for _, issue := range issues {
			if _, err := statements(tx).Insert("blueprint_migration_issues").Columns("id", "pipeline_id", "issue", "detected_at").Values(uuid.NewString(), candidate.id, issue, stamp(now)).ExecContext(ctx); err != nil {
				return fmt.Errorf("record Blueprint v3 issue for %q: %w", candidate.id, err)
			}
		}
	}
	for _, function := range functions {
		definition := function.definition
		definition.SchemaVersion = domain.GraphSchemaV3
		// Functions are unpublished below, so ambiguous values cannot execute.
		// Convert only safe values; their remaining repair happens in the editor.
		_ = migrateV2ConfigurationValues(&definition)
		encoded, err := encode(definition)
		if err != nil {
			return fmt.Errorf("encode Blueprint v3 function draft %q: %w", function.id, err)
		}
		// Function revisions are immutable. Clearing the published pointer makes
		// callers wait for an explicit V3 review and republish.
		if _, err := statements(tx).Update("functions").Set("draft_definition", encoded).Set("published_revision", 0).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": function.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("save Blueprint v3 function draft %q: %w", function.id, err)
		}
	}
	if _, err := statements(tx).Insert("settings").Columns("key", "value").Values(blueprintV3MigrationKey, stamp(now)).ExecContext(ctx); err != nil {
		return fmt.Errorf("save Blueprint v3 migration marker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Blueprint v3 migration: %w", err)
	}
	return nil
}

// migrateV2ConfigurationValues converts only values whose V2 text
// representation has exactly one canonical V3 JSON value. Any malformed or
// non-finite value is left untouched and gets an actionable repair issue.
func migrateV2ConfigurationValues(definition *domain.FlowDefinition) []string {
	issues := make([]string, 0)
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		config := migrationConfig(node)
		if config == nil {
			continue
		}
		switch node.Type {
		case "data:constant":
			switch config["type"] {
			case "number":
				if !canonicalNumber(config, "value") {
					issues = append(issues, fmt.Sprintf("%s has a non-canonical Number constant; enter a finite number or use Cast.", node.ID))
				}
			case "boolean":
				if !canonicalBool(config, "value") {
					issues = append(issues, fmt.Sprintf("%s has a non-canonical Boolean constant; enter true or false or use Cast.", node.ID))
				}
			}
		case "flow:gate":
			if !canonicalBool(config, "startOpen") {
				issues = append(issues, fmt.Sprintf("%s has a non-canonical Start open setting; choose true or false.", node.ID))
			}
		case "flow:multi_gate":
			if !canonicalBool(config, "loop") {
				issues = append(issues, fmt.Sprintf("%s has a non-canonical Loop setting; choose true or false.", node.ID))
			}
		default:
			for _, key := range v2NumericConfigKeys(node.Type) {
				if !canonicalNumber(config, key) {
					issues = append(issues, fmt.Sprintf("%s has a non-canonical %s number; enter a finite number.", node.ID, key))
				}
			}
		}
	}
	return issues
}

func migrationConfig(node *domain.FlowNode) map[string]any {
	if node.Data == nil {
		return nil
	}
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return node.Data
}

func canonicalNumber(config map[string]any, key string) bool {
	value, exists := config[key]
	if !exists || value == nil {
		return true
	}
	if number, ok := value.(float64); ok {
		return !math.IsNaN(number) && !math.IsInf(number, 0)
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return false
	}
	config[key] = number
	return true
}

func canonicalBool(config map[string]any, key string) bool {
	value, exists := config[key]
	if !exists || value == nil {
		return true
	}
	if _, ok := value.(bool); ok {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	boolean, err := strconv.ParseBool(strings.TrimSpace(text))
	if err != nil {
		return false
	}
	config[key] = boolean
	return true
}

func v2NumericConfigKeys(nodeType string) []string {
	switch nodeType {
	case "math:add", "math:subtract", "math:multiply", "math:divide":
		return []string{"a", "b"}
	case "date:create":
		return []string{"year", "month", "day", "hour", "minute", "second", "millisecond"}
	case "date:add", "date:subtract":
		return []string{"years", "months", "days", "hours", "minutes", "seconds", "milliseconds"}
	case "data:array_get":
		return []string{"index"}
	case "data:chat_history":
		return []string{"limit"}
	case "flow:for_loop":
		return []string{"first", "last"}
	case "llm:agent", "llm:coding_agent":
		return []string{"maxTurns"}
	default:
		return nil
	}
}

func hasV2DataEdges(definition domain.FlowDefinition) bool {
	for _, edge := range definition.Edges {
		if edge.Kind == domain.PinData {
			return true
		}
	}
	return false
}
