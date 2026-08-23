package executord

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testToken = "test-executor-token"

// fakeVault is an in-memory SecretStore for provider keys.
type fakeVault struct {
	mu      sync.Mutex
	secrets map[string]string
}

func newFakeVault() *fakeVault { return &fakeVault{secrets: map[string]string{}} }

func (f *fakeVault) Get(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.secrets[name]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (f *fakeVault) Put(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secrets[name] = value
	return nil
}

// tokenCredential injects the bearer token expected by the auth interceptor.
type tokenCredential struct{ token string }

func (c tokenCredential) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + c.token}, nil
}

func (c tokenCredential) RequireTransportSecurity() bool { return false }

type testExecutor struct {
	service  *Service
	runner   *Runner
	store    *store
	vault    *fakeVault
	listener *bufconn.Listener
	close    func()
}

func newTestExecutor(t *testing.T) (*grpc.ClientConn, *testExecutor) {
	t.Helper()
	dir := t.TempDir()
	if err := PrepareDataDir(dir); err != nil {
		t.Fatalf("PrepareDataDir() error = %v", err)
	}
	bundles := NewStore(dir)
	runtimeConfig, err := NewRuntimeStore(dir)
	if err != nil {
		t.Fatalf("NewRuntimeStore() error = %v", err)
	}
	globals, err := NewExecutorGlobals(dir)
	if err != nil {
		t.Fatalf("NewExecutorGlobals() error = %v", err)
	}
	vault := newFakeVault()
	tunnel := NewTunnelCaller()
	runner := NewRunner(bundles, catalog.New(), globals, tunnel, nil, runtimeConfig, vault)
	runner.Start()
	schedules := NewCronScheduler(bundles, runner)
	service := NewService("test-version", bundles, runtimeConfig, vault, runner, tunnel, schedules)

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(remoteexec.UnaryAuthInterceptor(testToken)),
		grpc.ChainStreamInterceptor(remoteexec.StreamAuthInterceptor(testToken)),
	)
	executorv1.RegisterExecutorServer(server, service)
	go func() { _ = server.Serve(listener) }()

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(tokenCredential{token: testToken}),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}

	cleanup := func() {
		_ = connection.Close()
		server.Stop()
		runner.Stop()
	}
	t.Cleanup(cleanup)
	return connection, &testExecutor{service: service, runner: runner, store: bundles, vault: vault, listener: listener, close: cleanup}
}

// dialWithoutToken opens an unauthenticated connection for rejection tests.
func (e *testExecutor) dialWithoutToken(t *testing.T) executorv1.ExecutorClient {
	t.Helper()
	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return e.listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return executorv1.NewExecutorClient(connection)
}

func buttonPipelineDefinition(t *testing.T) []byte {
	t.Helper()
	definition := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
		},
	}
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	return data
}

func TestGetStatusRequiresToken(t *testing.T) {
	_, executor := newTestExecutor(t)
	client := executor.dialWithoutToken(t)
	if _, err := client.GetStatus(context.Background(), &executorv1.StatusRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated call status = %v, want Unauthenticated", err)
	}
}

func TestDeployListAndRunLifecycle(t *testing.T) {
	connection, executor := newTestExecutor(t)
	client := executorv1.NewExecutorClient(connection)
	ctx := context.Background()

	statusResponse, err := client.GetStatus(ctx, &executorv1.StatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if statusResponse.ExecutorVersion != "test-version" || statusResponse.BlueprintSchemaVersion != int32(domain.GraphSchemaV3) {
		t.Fatalf("GetStatus() = %+v", statusResponse)
	}

	events := executor.runner.Subscribe()
	defer executor.runner.unsubscribe(events)

	deploy := &executorv1.DeployPipelineRequest{
		PipelineId:             "pipe-1",
		Name:                   "Remote hello",
		Revision:               3,
		BlueprintSchemaVersion: int32(domain.GraphSchemaV3),
		DefinitionJson:         buttonPipelineDefinition(t),
		Capabilities:           []string{},
	}
	if _, err := client.DeployPipeline(ctx, deploy); err != nil {
		t.Fatalf("DeployPipeline() error = %v", err)
	}

	listed, err := client.ListDeployed(ctx, &executorv1.ListDeployedRequest{})
	if err != nil || len(listed.Pipelines) != 1 {
		t.Fatalf("ListDeployed() = %#v, %v; want one deployment", listed, err)
	}

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := client.StartRun(startCtx, &executorv1.StartRunRequest{
		ExecutionId:            "exec-1",
		PipelineId:             "pipe-1",
		Revision:               3,
		TriggerNodeId:          "button",
		Unattended:             true,
		InputJson:              []byte(`{"input":"hello"}`),
		BlueprintSchemaVersion: int32(domain.GraphSchemaV3),
	}); err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	waitCompleted(t, client, "exec-1")
	waitForEvent(t, events, "exec-1", string(domain.RunCompleted))
}

// waitForEvent scans the subscription for one matching run state.
func waitForEvent(t *testing.T, events chan *executorv1.ExecutionEvent, executionID, wantStatus string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-events:
			run := event.GetRun()
			if run.GetExecutionId() == executionID && run.GetStatus() == wantStatus {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s to reach %s", executionID, wantStatus)
		}
	}
}

func waitCompleted(t *testing.T, client executorv1.ExecutorClient, executionID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		snapshot, err := client.GetRun(context.Background(), &executorv1.GetRunRequest{ExecutionId: executionID})
		if err == nil {
			last = snapshot.Status + "/" + snapshot.Error
			if snapshot.Status == string(domain.RunCompleted) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("execution %s did not complete in time (last state: %s)", executionID, last)
}

func TestDeployRejectsUnknownSchema(t *testing.T) {
	connection, _ := newTestExecutor(t)
	client := executorv1.NewExecutorClient(connection)
	_, err := client.DeployPipeline(context.Background(), &executorv1.DeployPipelineRequest{
		PipelineId:             "pipe-x",
		Revision:               1,
		BlueprintSchemaVersion: 99,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("DeployPipeline() status = %v, want FailedPrecondition", err)
	}
}

func TestUpdateConfigStoresProviderKeyOnce(t *testing.T) {
	connection, executor := newTestExecutor(t)
	client := executorv1.NewExecutorClient(connection)
	ctx := context.Background()

	response, err := client.UpdateConfig(ctx, &executorv1.UpdateConfigRequest{Config: &executorv1.ExecutorConfig{
		LlmMode:           string(domain.ExecutorLLMLocal),
		DefaultProviderId: "prov-1",
		MaxConcurrentRuns: 7,
		Providers: []*executorv1.ExecutorProvider{{
			Id:      "prov-1",
			Name:    "Hosted",
			Kind:    string(domain.ProviderOpenAICompatible),
			BaseUrl: "https://api.example.com/v1",
			Model:   "demo",
			Enabled: true,
			ApiKey:  "sk-super-secret",
		}},
	}})
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	provider := response.Providers[0]
	if provider.ApiKeySet != true || provider.ApiKey != "" {
		t.Fatalf("provider = %+v, want key stored once and never echoed", provider)
	}
	if got := executor.vault.secrets["executor-provider:prov-1"]; got != "sk-super-secret" {
		t.Fatalf("vault key = %q, want the stored API key", got)
	}

	config, err := client.GetConfig(ctx, &executorv1.GetConfigRequest{})
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if config.LlmMode != string(domain.ExecutorLLMLocal) || config.MaxConcurrentRuns != 7 {
		t.Fatalf("config = %+v", config)
	}
	if len(config.Providers) != 1 || config.Providers[0].ApiKeySet != true || config.Providers[0].ApiKey != "" {
		t.Fatalf("providers after read = %#v, want key material omitted", config.Providers)
	}
}

func TestCancelRunCompletesQueuedExecution(t *testing.T) {
	connection, executor := newTestExecutor(t)
	client := executorv1.NewExecutorClient(connection)
	ctx := context.Background()

	if _, err := client.DeployPipeline(ctx, &executorv1.DeployPipelineRequest{
		PipelineId:             "pipe-c",
		Name:                   "Cancellable",
		Revision:               1,
		BlueprintSchemaVersion: int32(domain.GraphSchemaV3),
		DefinitionJson:         buttonPipelineDefinition(t),
	}); err != nil {
		t.Fatalf("DeployPipeline() error = %v", err)
	}

	events := executor.runner.Subscribe()
	defer executor.runner.unsubscribe(events)

	if _, err := client.CancelRun(ctx, &executorv1.CancelRunRequest{ExecutionId: "missing"}); status.Code(err) != codes.NotFound {
		t.Fatalf("cancel unknown status = %v, want NotFound", err)
	}

	// A queued run cancelled before a worker picks it up completes as cancelled.
	executor.runner.cancelledMu.Lock()
	executor.runner.cancelled["exec-q"] = true
	executor.runner.cancelledMu.Unlock()
	job := runnerJob{
		record: RunRecord{
			ExecutionID:   "exec-q",
			PipelineID:    "pipe-c",
			Name:          "Cancellable",
			TriggerNodeID: "button",
			Status:        domain.RunPending,
			StartedAt:     time.Now().UTC(),
		},
		triggerNodeID: "button",
		unattended:    true,
	}
	if err := executor.runner.enqueueJob(job); err != nil {
		t.Fatalf("enqueueJob() error = %v", err)
	}
	waitForEvent(t, events, "exec-q", string(domain.RunCancelled))
	_ = client
}
