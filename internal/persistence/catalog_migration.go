package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	squirrel "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

const blueprintCatalogMigrationKey = "migration.blueprint-catalog-v3"
const rerouteWaypointMigrationKey = "migration.reroute-waypoints-v1"

// migrateRerouteWaypoints replaces the old transparent reroute runtime nodes
// with editor-only points on direct wires. Published revisions remain readable
// through the legacy registrations; all editable drafts are upgraded once.
func (s *Store) migrateRerouteWaypoints(ctx context.Context) error {
	var completed string
	err := statements(s.db).Select("value").From("settings").Where(squirrel.Eq{"key": rerouteWaypointMigrationKey}).QueryRowContext(ctx).Scan(&completed)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("read reroute waypoint migration marker: %w", err)
	}
	rows, err := statements(s.db).Select("id", "draft_definition").From("pipelines").Where("draft_definition LIKE ? OR draft_definition LIKE ?", "%flow:reroute%", "%data:reroute%").QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("scan reroute drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		id         string
		definition domain.FlowDefinition
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		var raw string
		if err := rows.Scan(&item.id, &raw); err != nil {
			return err
		}
		if err := decode(raw, &item.definition); err != nil {
			return fmt.Errorf("decode reroute draft %q: %w", item.id, err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range candidates {
		next, err := collapseRerouteNodes(item.definition)
		if err != nil {
			return fmt.Errorf("migrate pipeline %q reroutes: %w", item.id, err)
		}
		encoded, err := encode(next)
		if err != nil {
			return err
		}
		if _, err := statements(tx).Update("pipelines").Set("draft_definition", encoded).Set("status", domain.PipelineDraft).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": item.id}).ExecContext(ctx); err != nil {
			return err
		}
	}
	functionRows, err := statements(tx).Select("id", "draft_definition").From("functions").Where("draft_definition LIKE ? OR draft_definition LIKE ?", "%flow:reroute%", "%data:reroute%").QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("scan function reroute drafts: %w", err)
	}
	defer func() { _ = functionRows.Close() }()
	var functions []candidate
	for functionRows.Next() {
		var item candidate
		var raw string
		if err := functionRows.Scan(&item.id, &raw); err != nil {
			return err
		}
		if err := decode(raw, &item.definition); err != nil {
			return fmt.Errorf("decode function reroute draft %q: %w", item.id, err)
		}
		functions = append(functions, item)
	}
	if err := functionRows.Err(); err != nil {
		return err
	}
	for _, item := range functions {
		next, err := collapseRerouteNodes(item.definition)
		if err != nil {
			return fmt.Errorf("migrate function %q reroutes: %w", item.id, err)
		}
		encoded, err := encode(next)
		if err != nil {
			return err
		}
		if _, err := statements(tx).Update("functions").Set("draft_definition", encoded).Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"id": item.id}).ExecContext(ctx); err != nil {
			return err
		}
	}
	if _, err := statements(tx).Insert("settings").Columns("key", "value").Values(rerouteWaypointMigrationKey, stamp(time.Now().UTC())).ExecContext(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func collapseRerouteNodes(definition domain.FlowDefinition) (domain.FlowDefinition, error) {
	for _, node := range definition.Nodes {
		if node.Type != "flow:reroute" && node.Type != "data:reroute" {
			continue
		}
		var incoming, outgoing []domain.FlowEdge
		for _, edge := range definition.Edges {
			if edge.Target == node.ID {
				incoming = append(incoming, edge)
			}
			if edge.Source == node.ID {
				outgoing = append(outgoing, edge)
			}
		}
		if len(incoming) != 1 || len(outgoing) == 0 {
			return definition, fmt.Errorf("reroute %q must have one input and at least one output", node.ID)
		}
		if incoming[0].Kind != "" && outgoing[0].Kind != "" && incoming[0].Kind != outgoing[0].Kind {
			return definition, fmt.Errorf("reroute %q mixes pin kinds", node.ID)
		}
		nextEdges := make([]domain.FlowEdge, 0, len(definition.Edges)-1+len(outgoing))
		for _, edge := range definition.Edges {
			if edge.Source != node.ID && edge.Target != node.ID {
				nextEdges = append(nextEdges, edge)
			}
		}
		// A collapsed relay contributes every preceding waypoint (including any
		// already migrated upstream reroutes), then its own canvas position, in
		// wire order, ahead of the waypoints the outgoing wire already carried.
		prefix := make([]domain.Position, 0, len(incoming[0].Waypoints)+1)
		prefix = append(prefix, incoming[0].Waypoints...)
		prefix = append(prefix, domain.Position{X: node.Position.X, Y: node.Position.Y})
		for _, edge := range outgoing {
			edge.Source, edge.SourceHandle = incoming[0].Source, incoming[0].SourceHandle
			edge.Waypoints = append(append([]domain.Position(nil), prefix...), edge.Waypoints...)
			nextEdges = append(nextEdges, edge)
		}
		definition.Edges = nextEdges
		kept := make([]domain.FlowNode, 0, len(definition.Nodes)-1)
		for _, candidate := range definition.Nodes {
			if candidate.ID != node.ID {
				kept = append(kept, candidate)
			}
		}
		definition.Nodes = kept
	}
	return definition, nil
}

// migrateBlueprintCatalog converts only unambiguous packet-era draft nodes to
// their Blueprint-v2 equivalents. Immutable revisions and execution history
// remain untouched. Every affected pipeline is paused for user review.
func (s *Store) migrateBlueprintCatalog(ctx context.Context) error {
	var completed string
	err := statements(s.db).Select("value").From("settings").Where(squirrel.Eq{"key": blueprintCatalogMigrationKey}).QueryRowContext(ctx).Scan(&completed)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("read Blueprint catalog migration marker: %w", err)
	}

	rows, err := statements(s.db).Select("id", "draft_definition").From("pipelines").Where("draft_definition LIKE ?", "%logic:%").QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("scan packet-era drafts: %w", err)
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
			return fmt.Errorf("read packet-era draft: %w", err)
		}
		if err := decode(raw, &item.definition); err != nil {
			return fmt.Errorf("decode packet-era draft %q: %w", item.id, err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("scan packet-era drafts: %w", err)
	}
	if len(candidates) == 0 {
		_, err := statements(s.db).Insert("settings").Columns("key", "value").Values(blueprintCatalogMigrationKey, stamp(time.Now().UTC())).ExecContext(ctx)
		return err
	}
	if err := s.backupDatabase(ctx, "pre-blueprint-catalog-v3"); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start Blueprint catalog migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	for _, candidate := range candidates {
		definition, issues, changed := migrateBlueprintDefinition(candidate.definition)
		if !changed {
			continue
		}
		encoded, err := encode(definition)
		if err != nil {
			return fmt.Errorf("encode migrated draft %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Update("pipelines").Set("draft_definition", encoded).Set("status", domain.PipelineDraft).Set("updated_at", stamp(now)).Where(squirrel.Eq{"id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("save migrated draft %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Update("trigger_bindings").Set("enabled", false).Set("trusted", false).Set("updated_at", stamp(now)).Where(squirrel.Eq{"pipeline_id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("pause migrated triggers %q: %w", candidate.id, err)
		}
		if _, err := statements(tx).Delete("permissions").Where(squirrel.Eq{"pipeline_id": candidate.id}).ExecContext(ctx); err != nil {
			return fmt.Errorf("revoke migrated trust %q: %w", candidate.id, err)
		}
		for _, issue := range issues {
			if _, err := statements(tx).Insert("blueprint_migration_issues").Columns("id", "pipeline_id", "issue", "detected_at").Values(uuid.NewString(), candidate.id, issue, stamp(now)).ExecContext(ctx); err != nil {
				return fmt.Errorf("record migration issue for %q: %w", candidate.id, err)
			}
		}
	}
	if _, err := statements(tx).Insert("settings").Columns("key", "value").Values(blueprintCatalogMigrationKey, stamp(now)).ExecContext(ctx); err != nil {
		return fmt.Errorf("save Blueprint catalog migration marker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Blueprint catalog migration: %w", err)
	}
	return nil
}

func (s *Store) backupDatabase(ctx context.Context, label string) error {
	backup := filepath.Join(s.root, "neuropipe-"+label+"-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".db")
	escaped := strings.ReplaceAll(backup, "'", "''")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("back up database before %s: %w", label, err)
	}
	return nil
}

// migrateBlueprintDefinition returns a revised draft and repair messages. It
// intentionally refuses to guess packet field/path semantics: users must make
// those transformations explicit with typed nodes.
func migrateBlueprintDefinition(definition domain.FlowDefinition) (domain.FlowDefinition, []string, bool) {
	issues := make([]string, 0)
	changed := false
	existingIDs := make(map[string]struct{}, len(definition.Nodes))
	for _, node := range definition.Nodes {
		existingIDs[node.ID] = struct{}{}
	}
	newNodes := make([]domain.FlowNode, 0)
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		switch node.Type {
		case "logic:return":
			node.Type = "flow:return"
			changed = true
		case "logic:reroute":
			kind, ok := legacyRerouteKind(*node, definition.Edges)
			if !ok {
				issues = append(issues, fmt.Sprintf("%s uses a mixed reroute; replace it with separate Flow Reroute or Data Reroute nodes.", node.ID))
				changed = true
				continue
			}
			if kind == domain.PinData {
				node.Type = "data:reroute"
			} else {
				node.Type = "flow:reroute"
			}
			changed = true
		case "logic:store_value":
			node.Type = "flow:set_variable"
			config := flowNodeConfig(node)
			if value, exists := config["value"]; exists && !hasDataInput(definition.Edges, node.ID, "value") {
				constantID := uniqueMigrationNodeID(node.ID+"-value", existingIDs)
				existingIDs[constantID] = struct{}{}
				newNodes = append(newNodes, domain.FlowNode{ID: constantID, Type: "data:constant", Position: domain.Position{X: node.Position.X - 260, Y: node.Position.Y + 70}, Data: map[string]any{"config": map[string]any{"value": value}}})
				definition.Edges = append(definition.Edges, domain.FlowEdge{ID: uniqueMigrationEdgeID(node.ID+"-value", definition.Edges), Source: constantID, Target: node.ID, SourceHandle: "value", TargetHandle: "value", Kind: domain.PinData})
			}
			delete(config, "value")
			node.Data["config"] = config
			changed = true
		case "logic:set", "logic:extract_value", "logic:condition", "logic:switch", "logic:merge", "logic:filter", "logic:aggregate", "logic:limit":
			issues = append(issues, fmt.Sprintf("%s uses removed packet-routing node %q; rebuild this step with typed Blueprint data and flow nodes.", node.ID, node.Type))
			changed = true
		}
	}
	definition.Nodes = append(definition.Nodes, newNodes...)
	return definition, issues, changed
}

func flowNodeConfig(node *domain.FlowNode) map[string]any {
	if node.Data == nil {
		node.Data = map[string]any{}
	}
	config, ok := node.Data["config"].(map[string]any)
	if !ok {
		config = map[string]any{}
	}
	return config
}

func hasDataInput(edges []domain.FlowEdge, nodeID, handle string) bool {
	for _, edge := range edges {
		if edge.Target == nodeID && edge.TargetHandle == handle && edge.Kind == domain.PinData {
			return true
		}
	}
	return false
}

func legacyRerouteKind(node domain.FlowNode, edges []domain.FlowEdge) (domain.PinKind, bool) {
	kind := domain.PinKind("")
	for _, edge := range edges {
		if edge.Source != node.ID && edge.Target != node.ID {
			continue
		}
		edgeKind := edge.Kind
		if edgeKind == "" {
			edgeKind = domain.PinExec
		}
		if kind != "" && kind != edgeKind {
			return "", false
		}
		kind = edgeKind
	}
	if kind == "" {
		kind = domain.PinExec
	}
	return kind, true
}

func uniqueMigrationNodeID(prefix string, existing map[string]struct{}) string {
	id := prefix
	for index := 2; ; index++ {
		if _, exists := existing[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s-%d", prefix, index)
	}
}

func uniqueMigrationEdgeID(prefix string, edges []domain.FlowEdge) string {
	seen := make(map[string]struct{}, len(edges))
	for _, edge := range edges {
		seen[edge.ID] = struct{}{}
	}
	id := prefix
	for index := 2; ; index++ {
		if _, exists := seen[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s-%d", prefix, index)
	}
}
