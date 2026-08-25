package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

// jsonRaw decodes an embedded JSON literal into a generic object argument.
func jsonRaw(t *testing.T, literal string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(literal), &value); err != nil {
		t.Fatalf("decode test JSON: %v", err)
	}
	return value
}

// scriptedAssistant replays queued responses in order and records requests.
type scriptedAssistant struct {
	mu        sync.Mutex
	responses []domain.AssistantChatResponse
	requests  []domain.AssistantChatRequest
}

func (a *scriptedAssistant) Converse(_ context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	if len(a.responses) == 0 {
		return domain.AssistantChatResponse{Content: "done"}, nil
	}
	response := a.responses[0]
	a.responses = a.responses[1:]
	return response, nil
}

// stubAuthoring records authoring calls and returns canned results so tests
// can exercise gating without touching package app.
type stubAuthoring struct {
	mu           sync.Mutex
	validateErr  error
	savedDefs    []domain.FlowDefinition
	publishedIDs []string
	deletedIDs   []string
	next         domain.Pipeline
}

func (a *stubAuthoring) ValidatePipeline(def domain.FlowDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.validateErr
}

func (a *stubAuthoring) CreatePipelineDraft(_ context.Context, name, description string, def domain.FlowDefinition) (domain.Pipeline, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.savedDefs = append(a.savedDefs, def)
	a.next = domain.Pipeline{ID: "pipe-new", Name: name, Description: description, Status: domain.PipelineDraft, DraftDefinition: def}
	return a.next, nil
}

func (a *stubAuthoring) SavePipelineDraft(ctx context.Context, id, name, description string, def domain.FlowDefinition) (domain.Pipeline, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.savedDefs = append(a.savedDefs, def)
	a.next = domain.Pipeline{ID: id, Name: name, Description: description, Status: domain.PipelineDraft, DraftDefinition: def}
	return a.next, nil
}

func (a *stubAuthoring) GetPipelineFull(_ context.Context, id string) (domain.Pipeline, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.next.ID == id {
		return a.next, nil
	}
	return domain.Pipeline{ID: id, Name: "Existing", Status: domain.PipelineActive}, nil
}

func (a *stubAuthoring) PublishPipeline(_ context.Context, id string) (domain.Pipeline, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.publishedIDs = append(a.publishedIDs, id)
	return domain.Pipeline{ID: id, Status: domain.PipelineActive, PublishedRevision: 2}, nil
}

func (a *stubAuthoring) DeletePipeline(_ context.Context, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletedIDs = append(a.deletedIDs, id)
	return nil
}

func (a *stubAuthoring) ValidateFunction(domain.CustomFunction) error { return nil }

func (a *stubAuthoring) SaveFunctionDraft(_ context.Context, fn domain.CustomFunction) (domain.CustomFunction, error) {
	fn.ID = "fn-1"
	return fn, nil
}

func (a *stubAuthoring) GetFunction(_ context.Context, id string) (domain.CustomFunction, error) {
	return domain.CustomFunction{ID: id, Name: "Fn"}, nil
}

func (a *stubAuthoring) ListFunctions(context.Context) ([]domain.FunctionSummary, error) {
	return nil, nil
}

func (a *stubAuthoring) PublishFunction(_ context.Context, fn domain.CustomFunction) (domain.CustomFunction, error) {
	fn.PublishedRevision = 1
	return fn, nil
}

func (a *stubAuthoring) DeleteFunction(_ context.Context, id string) error { return nil }

func toolCallResponse(name string, arguments map[string]any) domain.AssistantChatResponse {
	return domain.AssistantChatResponse{ToolCalls: []domain.ChatToolCall{{ID: "call-" + name, Name: name, Arguments: arguments}}}
}

// approveNext resolves the first pending approval, mirroring the user
// clicking Allow in the chat UI.
func approveNext(t *testing.T, service *Service, store *persistence.Store, conversationID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		approvals, err := store.ListPendingChatApprovals(context.Background(), conversationID)
		if err == nil && len(approvals) > 0 {
			if err := service.ResolveApproval(context.Background(), approvals[0].ID, true); err != nil {
				t.Fatalf("ResolveApproval() error = %v", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no pending approval appeared")
}
func waitForRunStatus(t *testing.T, store *persistence.Store, runID string, want domain.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last domain.RunStatus
	var lastErr string
	for time.Now().Before(deadline) {
		run, err := store.GetChatRun(context.Background(), runID)
		if err == nil {
			last, lastErr = run.Status, run.Error
			if run.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach %s (status=%s error=%q)", runID, want, last, lastErr)
}

func dumpConversation(t *testing.T, store *persistence.Store, conversationID string) {
	t.Helper()
	messages, err := store.ListChatMessages(context.Background(), conversationID, 100)
	if err != nil {
		t.Fatalf("dump messages error = %v", err)
	}
	for _, m := range messages {
		t.Logf("MSG role=%s tool=%s content=%.80q", m.Role, m.ToolName, m.Content)
	}
}

func lastToolContent(t *testing.T, store *persistence.Store, conversationID string) (string, domain.ChatMessage) {
	t.Helper()
	messages, err := store.ListChatMessages(context.Background(), conversationID, 100)
	if err != nil {
		t.Fatalf("ListChatMessages() error = %v", err)
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == domain.ChatRoleTool {
			return messages[i].Content, messages[i]
		}
	}
	t.Fatal("no tool result message")
	return "", domain.ChatMessage{}
}

const validGraphJSON = `{"schemaVersion":3,"nodes":[{"id":"t","type":"trigger:button","position":{"x":0,"y":0},"data":{"config":{"label":"Go"}}}]}`

func TestGetNodeContractAndGuideTools(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	registry := catalog.New()
	assistant := &scriptedAssistant{
		responses: []domain.AssistantChatResponse{
			toolCallResponse("get_node_contract", map[string]any{"nodeType": "action:http"}),
			toolCallResponse("get_authoring_guide", map[string]any{}),
		},
	}
	service := NewService(store, nil, assistant, nil, WithAuthoring(&stubAuthoring{}), WithNodeCatalog(registry))
	service.Start(context.Background())
	defer service.Stop()

	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Docs"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "How do I call HTTP?")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	content, _ := lastToolContent(t, store, conversation.ID)
	if !strings.Contains(content, `"type": "action:http"`) && !strings.Contains(content, "action:http") {
		t.Fatalf("contract payload missing node type: %s", content)
	}

	messages, err := store.ListChatMessages(context.Background(), conversation.ID, 100)
	if err != nil {
		t.Fatalf("messages error = %v", err)
	}
	foundGuide := false
	for _, m := range messages {
		if m.Role == domain.ChatRoleTool && strings.Contains(m.Content, "Blueprint v3") {
			foundGuide = true
		}
	}
	if !foundGuide {
		t.Fatalf("authoring guide not delivered as tool result")
	}
	if len(assistant.requests) == 0 || !strings.Contains(assistant.requests[0].Messages[0].Content, "AUTHOR") {
		t.Fatalf("system guidance missing from first request")
	}
}

func TestSavePipelineDraftCreatesValidatedDraft(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	authoring := &stubAuthoring{}
	assistant := &scriptedAssistant{
		responses: []domain.AssistantChatResponse{
			toolCallResponse("save_pipeline_draft", map[string]any{
				"name":       "Generated",
				"definition": jsonRaw(t, validGraphJSON),
			}),
		},
	}
	registry := catalog.New()
	service := NewService(store, nil, assistant, nil, WithAuthoring(authoring), WithNodeCatalog(registry))
	service.Start(context.Background())
	defer service.Stop()

	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Author"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Make me a pipeline")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	approveNext(t, service, store, conversation.ID)
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	content, _ := lastToolContent(t, store, conversation.ID)
	if !strings.Contains(content, `"saved": true`) && !strings.Contains(content, `"pipelineId"`) {
		t.Fatalf("save result = %s", content)
	}
	authoring.mu.Lock()
	defer authoring.mu.Unlock()
	if len(authoring.savedDefs) != 1 || authoring.savedDefs[0].SchemaVersion != 3 {
		t.Fatalf("saved definitions = %#v", authoring.savedDefs)
	}
}

func TestSavePipelineDraftSurfacesValidationErrorWithoutSaving(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	authoring := &stubAuthoring{validateErr: context.DeadlineExceeded} // any non-nil sentinel
	assistant := &scriptedAssistant{
		responses: []domain.AssistantChatResponse{
			toolCallResponse("save_pipeline_draft", map[string]any{"name": "Broken", "definition": jsonRaw(t, validGraphJSON)}),
		},
	}
	registry := catalog.New()
	service := NewService(store, nil, assistant, nil, WithAuthoring(authoring), WithNodeCatalog(registry))
	service.Start(context.Background())
	defer service.Stop()

	conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: "Broken"})
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	run, err := service.Send(context.Background(), conversation.ID, "Try to save")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	approveNext(t, service, store, conversation.ID)
	waitForRunStatus(t, store, run.ID, domain.RunCompleted)

	content, _ := lastToolContent(t, store, conversation.ID)
	if !strings.Contains(content, `"validationError"`) || !strings.Contains(content, `"saved":false`) {
		t.Fatalf("validation failure payload = %s", content)
	}
	authoring.mu.Lock()
	defer authoring.mu.Unlock()
	if len(authoring.savedDefs) != 0 {
		t.Fatalf("invalid draft must not be persisted")
	}
}

func TestPublishAndDeletePipelinePauseForApproval(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	for _, name := range []string{"publish_pipeline", "delete_pipeline"} {
		assistant := &scriptedAssistant{
			responses: []domain.AssistantChatResponse{
				toolCallResponse(name, map[string]any{"pipelineId": "pipe-1"}),
			},
		}
		authoring := &stubAuthoring{}
		service := NewService(store, nil, assistant, nil, WithAuthoring(authoring), WithNodeCatalog(catalog.New()))
		service.Start(context.Background())
		conversation, err := service.CreateConversation(context.Background(), domain.ChatConversation{Mode: domain.ChatModeModel, Title: name})
		if err != nil {
			t.Fatalf("CreateConversation() error = %v", err)
		}
		if _, err := service.Send(context.Background(), conversation.ID, name); err != nil {
			t.Fatalf("Send(%s) error = %v", name, err)
		}
		deadline := time.Now().Add(time.Second)
		approved := false
		for time.Now().Before(deadline) {
			approvals, listErr := store.ListPendingChatApprovals(context.Background(), conversation.ID)
			if listErr == nil && len(approvals) > 0 {
				if approvals[0].ToolCall.Name != name {
					t.Fatalf("%s: approval = %#v", name, approvals[0])
				}
				if err := service.ResolveApproval(context.Background(), approvals[0].ID, true); err != nil {
					t.Fatalf("ResolveApproval() error = %v", err)
				}
				approved = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !approved {
			t.Fatalf("%s did not pause for approval", name)
		}
		service.Stop()
		authoring.mu.Lock()
		if name == "publish_pipeline" && len(authoring.publishedIDs) != 1 {
			t.Fatalf("publish not executed after approval")
		}
		if name == "delete_pipeline" && len(authoring.deletedIDs) != 1 {
			t.Fatalf("delete not executed after approval")
		}
		authoring.mu.Unlock()
	}
}
