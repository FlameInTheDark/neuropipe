package persistence

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestSettingsPersistHideToTrayOnClose(t *testing.T) {
	t.Parallel()
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	settings, err := store.LoadSettings(context.Background(), filepath.Join(t.TempDir(), "plugins"))
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if settings.HideToTrayOnClose {
		t.Fatal("HideToTrayOnClose = true for a new profile, want false")
	}
	settings.HideToTrayOnClose = true
	if err := store.SaveSettings(context.Background(), settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	loaded, err := store.LoadSettings(context.Background(), filepath.Join(t.TempDir(), "plugins"))
	if err != nil {
		t.Fatalf("LoadSettings() after save error = %v", err)
	}
	if !loaded.HideToTrayOnClose {
		t.Fatal("HideToTrayOnClose = false after save, want true")
	}
}

func TestPublishCreatesImmutableRevisionAndBindings(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}}}}
	pipeline, err := store.CreatePipeline(ctx, "Inbox", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	published, err := store.Publish(ctx, pipeline, []domain.TriggerBinding{{NodeID: "button", Kind: domain.TriggerButton, Label: "Inbox", Icon: "play", Color: "#fff"}})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.PublishedRevision != 1 || published.Status != domain.PipelineActive {
		t.Fatalf("published = %#v, want active revision 1", published)
	}
	bindings, err := store.ListTriggers(ctx, domain.TriggerButton)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("ListTriggers() = %d, %v; want one binding", len(bindings), err)
	}
	if !bindings[0].Enabled || bindings[0].Trusted {
		t.Fatal("published button bindings must start enabled and untrusted")
	}
	if _, err := store.PublishedDefinition(ctx, pipeline.ID, 1); err != nil {
		t.Fatalf("PublishedDefinition() error = %v", err)
	}
}

func TestTrustedRevisionStillAllowsDraftUpdatesAndRepublishing(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{{
		ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Before"}},
	}}}
	pipeline, err := store.CreatePipeline(ctx, "Editable after trust", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	binding := domain.TriggerBinding{NodeID: "button", Kind: domain.TriggerButton, Label: "Run"}
	published, err := store.Publish(ctx, pipeline, []domain.TriggerBinding{binding})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := store.TrustRevision(ctx, published.ID, published.PublishedRevision); err != nil {
		t.Fatalf("TrustRevision() error = %v", err)
	}

	published.DraftDefinition.Nodes[0].Data = map[string]any{"config": map[string]any{"label": "After"}}
	saved, err := store.SaveDraft(ctx, published)
	if err != nil {
		t.Fatalf("SaveDraft() after trust error = %v", err)
	}
	if saved.PublishedRevision != 1 || saved.Status != domain.PipelineActive || !saved.HasUnpublishedChanges {
		t.Fatalf("saved pipeline = %#v, want active revision 1 with an unpublished draft", saved)
	}

	republished, err := store.Publish(ctx, saved, []domain.TriggerBinding{binding})
	if err != nil {
		t.Fatalf("Publish() after trust error = %v", err)
	}
	if republished.PublishedRevision != 2 || republished.HasUnpublishedChanges {
		t.Fatalf("republished pipeline = %#v, want revision 2 without draft changes", republished)
	}
	bindings, err := store.ListTriggers(ctx, domain.TriggerButton)
	if err != nil || len(bindings) != 1 || bindings[0].Revision != 2 || bindings[0].Trusted {
		t.Fatalf("replacement bindings = %#v, %v; want an untrusted revision 2 binding", bindings, err)
	}
}

func TestPublishEnablesGlobalHotkeyBinding(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Hotkey", domain.FlowDefinition{})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if _, err := store.Publish(ctx, pipeline, []domain.TriggerBinding{{NodeID: "hotkey", Kind: domain.TriggerHotkey, Label: "Launch", Hotkey: "Ctrl+Alt+N"}}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	bindings, err := store.ListTriggers(ctx, domain.TriggerHotkey)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("ListTriggers() = %#v, %v", bindings, err)
	}
	if !bindings[0].Enabled || bindings[0].Trusted {
		t.Fatalf("hotkey binding = %#v, want enabled and untrusted", bindings[0])
	}
}

func TestBlueprintCatalogMigrationConvertsSafeNodesAndFlagsAmbiguity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Migration", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		{ID: "store", Type: "logic:store_value", Data: map[string]any{"config": map[string]any{"name": "Greeting", "value": "hello"}}},
		{ID: "condition", Type: "logic:condition", Data: map[string]any{"config": map[string]any{"path": "status"}}},
	}})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if _, err := statements(store.db).Delete("settings").Where("key = ?", blueprintCatalogMigrationKey).ExecContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateBlueprintCatalog(ctx); err != nil {
		t.Fatalf("migrateBlueprintCatalog() error = %v", err)
	}
	updated, err := store.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DraftDefinition.Nodes[0].Type != "flow:set_variable" {
		t.Fatalf("safe node = %q, want flow:set_variable", updated.DraftDefinition.Nodes[0].Type)
	}
	if updated.MigrationIssue == "" || !strings.Contains(updated.MigrationIssue, "logic:condition") {
		t.Fatalf("MigrationIssue = %q", updated.MigrationIssue)
	}
	if len(updated.DraftDefinition.Nodes) != 3 || updated.DraftDefinition.Nodes[2].Type != "data:constant" {
		t.Fatalf("migrated nodes = %#v", updated.DraftDefinition.Nodes)
	}
	backups, err := filepath.Glob(filepath.Join(root, "neuropipe-pre-blueprint-catalog-v3-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup files = %v, %v", backups, err)
	}
}

func TestBlueprintV3MigrationBacksUpAndPausesPublishedDrafts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	definition := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		{ID: "notice", Type: "action:notification", Data: map[string]any{"config": map[string]any{"title": "Done", "message": "Done"}}},
	}, Edges: []domain.FlowEdge{{ID: "run", Source: "button", SourceHandle: "out", Target: "notice", TargetHandle: "in", Kind: domain.PinExec}}}
	pipeline, err := store.CreatePipeline(ctx, "V3 migration", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if _, err := store.Publish(ctx, pipeline, []domain.TriggerBinding{{NodeID: "button", Kind: domain.TriggerButton, Label: "Run"}}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if _, err := statements(store.db).Delete("settings").Where("key = ?", blueprintV3MigrationKey).ExecContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateBlueprintV3(ctx); err != nil {
		t.Fatalf("migrateBlueprintV3() error = %v", err)
	}
	updated, err := store.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DraftDefinition.SchemaVersion != domain.GraphSchemaV3 || updated.Status != domain.PipelineDraft {
		t.Fatalf("migrated pipeline = %#v", updated)
	}
	bindings, err := store.ListTriggers(ctx, domain.TriggerButton)
	if err != nil || len(bindings) != 1 || bindings[0].Enabled || bindings[0].Trusted {
		t.Fatalf("bindings = %#v, %v", bindings, err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "neuropipe-pre-blueprint-v3-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup files = %v, %v", backups, err)
	}
}

func TestBlueprintV3MigrationUnpublishesV2Functions(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	function, err := store.CreateFunction(ctx, "Legacy function", domain.NodePure)
	if err != nil {
		t.Fatalf("CreateFunction() error = %v", err)
	}
	function.DraftDefinition.SchemaVersion = domain.GraphSchemaV2
	function, err = store.PublishFunction(ctx, function)
	if err != nil || function.PublishedRevision != 1 {
		t.Fatalf("PublishFunction() = %#v, %v", function, err)
	}
	if _, err := statements(store.db).Delete("settings").Where("key = ?", blueprintV3MigrationKey).ExecContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateBlueprintV3(ctx); err != nil {
		t.Fatalf("migrateBlueprintV3() error = %v", err)
	}
	updated, err := store.GetFunction(ctx, function.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DraftDefinition.SchemaVersion != domain.GraphSchemaV3 || updated.PublishedRevision != 0 {
		t.Fatalf("migrated function = %#v", updated)
	}
}

func TestCreateLLMToolFunctionPersistsDedicatedToolContract(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	created, err := store.CreateFunctionWithRequest(ctx, domain.CreateFunctionRequest{
		Name:        "Search weather",
		Description: "Looks up a local weather forecast.",
		Kind:        domain.FunctionTool,
		Mode:        domain.NodePure,
	})
	if err != nil {
		t.Fatalf("CreateFunctionWithRequest() error = %v", err)
	}
	if created.Kind != domain.FunctionTool || created.Mode != domain.NodeImpure {
		t.Fatalf("created function = %#v", created)
	}
	reloaded, err := store.GetFunction(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Kind != domain.FunctionTool || reloaded.Description != created.Description {
		t.Fatalf("reloaded function = %#v", reloaded)
	}
	definition := FunctionNodeDefinition(reloaded)
	if definition.Mode != domain.NodeVisual || len(definition.Inputs) != 0 || len(definition.Outputs) != 1 || definition.Outputs[0].Kind != domain.PinTool || definition.Outputs[0].MaxConnections != 0 {
		t.Fatalf("tool node definition = %#v", definition)
	}
}

func TestBlueprintV3MigrationCanonicalizesSafeConfigurationValues(t *testing.T) {
	definition := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		{ID: "number", Type: "data:constant", Data: map[string]any{"config": map[string]any{"type": "number", "value": "7.5"}}},
		{ID: "boolean", Type: "data:constant", Data: map[string]any{"config": map[string]any{"type": "boolean", "value": "true"}}},
		{ID: "bad", Type: "data:constant", Data: map[string]any{"config": map[string]any{"type": "number", "value": "not a number"}}},
		{ID: "date", Type: "date:create", Data: map[string]any{"config": map[string]any{"year": "2026"}}},
	}}
	issues := migrateV2ConfigurationValues(&definition)
	configs := []map[string]any{
		definition.Nodes[0].Data["config"].(map[string]any),
		definition.Nodes[1].Data["config"].(map[string]any),
		definition.Nodes[3].Data["config"].(map[string]any),
	}
	if configs[0]["value"] != 7.5 || configs[1]["value"] != true || configs[2]["year"] != 2026.0 {
		t.Fatalf("canonical configs = %#v", configs)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "bad") {
		t.Fatalf("migration issues = %#v", issues)
	}
}

func TestListExecutionsIncludesPersistedNodeRuns(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Run history", domain.FlowDefinition{Nodes: []domain.FlowNode{{ID: "button", Type: "trigger:button"}}})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	execution, err := store.StartExecution(ctx, pipeline.ID, "draft:button")
	if err != nil {
		t.Fatalf("StartExecution() error = %v", err)
	}
	execution.Status = domain.RunCompleted
	execution.NodeRuns = []domain.NodeRun{{NodeID: "button", NodeType: "trigger:button", Status: domain.RunCompleted, Input: map[string]any{"trigger": "manual"}, Output: map[string]any{"out": "ok"}}}
	if err := store.CompleteExecution(ctx, execution); err != nil {
		t.Fatalf("CompleteExecution() error = %v", err)
	}

	executions, err := store.ListExecutions(ctx, pipeline.ID, 20)
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if len(executions) != 1 || len(executions[0].NodeRuns) != 1 {
		t.Fatalf("ListExecutions() = %#v, want one execution with one node run", executions)
	}
	got := executions[0].NodeRuns[0]
	if got.NodeID != "button" || got.Status != domain.RunCompleted || got.Output.(map[string]any)["out"] != "ok" {
		t.Fatalf("node run = %#v, want persisted node result", got)
	}
}

func TestCreateAndListReportsIncludesPipelineAndExecutionContext(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Daily briefing", domain.FlowDefinition{Nodes: []domain.FlowNode{{ID: "button", Type: "trigger:button"}}})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	execution, err := store.StartExecution(ctx, pipeline.ID, "draft:button")
	if err != nil {
		t.Fatalf("StartExecution() error = %v", err)
	}
	created, err := store.CreateReport(ctx, domain.Report{PipelineID: pipeline.ID, ExecutionID: execution.ID, NodeID: "report", Title: "Morning update", Tags: []string{"Daily", "daily", "Operations"}, Markdown: "# Ready"})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	reports, err := store.ListReports(ctx, 20)
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("report count = %d, want one", len(reports))
	}
	got := reports[0]
	if got.ID != created.ID || got.PipelineName != "Daily briefing" || got.ExecutionStartedAt.IsZero() || got.CreatedAt.IsZero() {
		t.Fatalf("report feed item = %#v", got)
	}
	if want := []string{"Daily", "Operations"}; !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("report tags = %#v, want %#v", got.Tags, want)
	}
	if err := store.DeleteReport(ctx, created.ID); err != nil {
		t.Fatalf("DeleteReport() error = %v", err)
	}
	if reports, err := store.ListReports(ctx, 20); err != nil || len(reports) != 0 {
		t.Fatalf("ListReports() after delete = %#v, %v; want no reports", reports, err)
	}
}

func TestReportTagsColumnMigratesExistingWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	database, err := sql.Open("sqlite3", filepath.Join(root, "neuropipe.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := database.Exec(`CREATE TABLE reports (id TEXT PRIMARY KEY, pipeline_id TEXT NOT NULL, execution_id TEXT NOT NULL, node_id TEXT NOT NULL, title TEXT NOT NULL, markdown TEXT NOT NULL, created_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create legacy reports table: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	var columns int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('reports') WHERE name = 'tags_json'`).Scan(&columns); err != nil {
		t.Fatalf("inspect report columns: %v", err)
	}
	if columns != 1 {
		t.Fatalf("tags_json columns = %d, want 1", columns)
	}
}

func TestLegacyGraphsAreBackedUpPausedAndMarkedReadOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	legacy, err := store.CreatePipeline(context.Background(), "Legacy", domain.FlowDefinition{Nodes: []domain.FlowNode{{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Legacy"}}}}})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = New(root)
	if err != nil {
		t.Fatalf("reopen New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	loaded, err := store.GetPipeline(context.Background(), legacy.ID)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if loaded.Status != domain.PipelineLegacy {
		t.Fatalf("legacy status = %q, want %q", loaded.Status, domain.PipelineLegacy)
	}
	backups, err := filepath.Glob(filepath.Join(root, "neuropipe-pre-blueprint-v2-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup files = %v, %v; want one", backups, err)
	}
}

func TestV3PipelineIsNotMarkedLegacyAndRepairsOldMarker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	store, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	pipeline, err := store.CreatePipeline(ctx, "Current", domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes:         []domain.FlowNode{{ID: "button", Type: "trigger:button"}},
	})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if _, err := statements(store.db).Insert("legacy_graphs").Columns("pipeline_id", "detected_at", "reason").Values(pipeline.ID, "now", "Blueprint v2 rebuild required").ExecContext(ctx); err != nil {
		t.Fatalf("insert invalid legacy marker: %v", err)
	}
	if _, err := statements(store.db).Update("pipelines").Set("status", domain.PipelineLegacy).Where("id = ?", pipeline.ID).ExecContext(ctx); err != nil {
		t.Fatalf("mark pipeline legacy: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = New(root)
	if err != nil {
		t.Fatalf("reopen New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	loaded, err := store.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if loaded.Status != domain.PipelineDraft {
		t.Fatalf("pipeline status = %q, want %q", loaded.Status, domain.PipelineDraft)
	}
	var markers int
	if err := statements(store.db).Select("COUNT(*)").From("legacy_graphs").Where("pipeline_id = ?", pipeline.ID).QueryRowContext(ctx).Scan(&markers); err != nil {
		t.Fatalf("count legacy markers: %v", err)
	}
	if markers != 0 {
		t.Fatalf("legacy markers = %d, want 0", markers)
	}
}

func TestDeletePipelineRemovesPipeline(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	pipeline, err := store.CreatePipeline(context.Background(), "Disposable", domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2})
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if err := store.DeletePipeline(context.Background(), pipeline.ID); err != nil {
		t.Fatalf("DeletePipeline() error = %v", err)
	}
	if _, err := store.GetPipeline(context.Background(), pipeline.ID); err == nil {
		t.Fatal("GetPipeline() error = nil after deletion")
	}
}

func TestChatTranscriptPersistsMessagesRunsAndReplies(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	conversation, err := store.CreateChatConversation(ctx, domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Planning"})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}
	run, err := store.CreateChatRun(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("CreateChatRun() error = %v", err)
	}
	if _, err := store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversation.ID, ChatRunID: run.ID, Role: domain.ChatRoleUser, Content: "Hello"}); err != nil {
		t.Fatalf("CreateChatMessage() error = %v", err)
	}
	if _, err := store.AppendChatReply(ctx, run.ID, "# Ready"); err != nil {
		t.Fatalf("AppendChatReply() error = %v", err)
	}
	if err := store.UpdateChatStatus(ctx, run.ID, "Thinking"); err != nil {
		t.Fatalf("UpdateChatStatus() error = %v", err)
	}
	messages, err := store.ListChatMessages(ctx, conversation.ID, 10)
	if err != nil || len(messages) != 2 {
		t.Fatalf("ListChatMessages() = %#v, %v; want user and assistant", messages, err)
	}
	if messages[1].Role != domain.ChatRoleAssistant || messages[1].Content != "# Ready" {
		t.Fatalf("reply message = %#v", messages[1])
	}
	loaded, err := store.GetChatRun(ctx, run.ID)
	if err != nil || loaded.StatusText != "Thinking" || loaded.Status != domain.RunRunning {
		t.Fatalf("GetChatRun() = %#v, %v", loaded, err)
	}
	events, err := store.ListChatRunEvents(ctx, run.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("ListChatRunEvents() = %#v, %v; want reply and status", events, err)
	}
}

func TestChatToolGrantIsScopedToConversationAndPublishedRevision(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	ctx := context.Background()
	first, err := store.CreateChatConversation(ctx, domain.ChatConversation{Mode: domain.ChatModeModel, Title: "First", ActionPolicy: domain.ChatActionAlways})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}
	second, err := store.CreateChatConversation(ctx, domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Second", ActionPolicy: domain.ChatActionAlways})
	if err != nil {
		t.Fatalf("CreateChatConversation() error = %v", err)
	}
	if err := store.SaveChatToolGrant(ctx, first.ID, "run_pipeline", "pipeline-1", 2); err != nil {
		t.Fatalf("SaveChatToolGrant() error = %v", err)
	}
	matching, err := store.HasChatToolGrant(ctx, first.ID, "run_pipeline", "pipeline-1", 2)
	if err != nil || !matching {
		t.Fatalf("matching grant = %t, %v; want true", matching, err)
	}
	stale, err := store.HasChatToolGrant(ctx, first.ID, "run_pipeline", "pipeline-1", 3)
	if err != nil || stale {
		t.Fatalf("republished grant = %t, %v; want false", stale, err)
	}
	crossConversation, err := store.HasChatToolGrant(ctx, second.ID, "run_pipeline", "pipeline-1", 2)
	if err != nil || crossConversation {
		t.Fatalf("cross-conversation grant = %t, %v; want false", crossConversation, err)
	}
}
