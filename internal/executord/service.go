package executord

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExecutorVersion identifies the daemon build; replaced via ldflags in releases.
var ExecutorVersion = "dev"

// Service implements the executor gRPC contract.
type Service struct {
	executorv1.UnimplementedExecutorServer

	version   string
	store     *store
	runtime   *runtimeStore
	vault     SecretStore
	runner    *Runner
	tunnel    *TunnelCaller
	schedules *CronScheduler
	started   time.Time

	configMu sync.Mutex
}

// NewService composes the gRPC facade over the execution core.
func NewService(version string, store *store, runtime *runtimeStore, vault SecretStore, runner *Runner, tunnel *TunnelCaller, schedules *CronScheduler) *Service {
	return &Service{version: version, store: store, runtime: runtime, vault: vault, runner: runner, tunnel: tunnel, schedules: schedules, started: time.Now().UTC()}
}

// runtimePlatform reports GOOS/GOARCH for the Settings status display.
func runtimePlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func marshalJSON(value any) ([]byte, error)       { return json.Marshal(value) }
func unmarshalJSON(data []byte, target any) error { return json.Unmarshal(data, target) }

func (s *Service) GetStatus(ctx context.Context, _ *executorv1.StatusRequest) (*executorv1.ExecutorStatus, error) {
	return &executorv1.ExecutorStatus{
		ExecutorVersion:        s.version,
		Platform:               runtimePlatform(),
		BlueprintSchemaVersion: domain.GraphSchemaV3,
		UptimeSeconds:          int64(time.Since(s.started).Seconds()),
	}, nil
}

func (s *Service) GetConfig(ctx context.Context, _ *executorv1.GetConfigRequest) (*executorv1.ExecutorConfig, error) {
	config := s.runtime.Get()
	result := &executorv1.ExecutorConfig{
		LlmMode:           string(config.LLMMode),
		DefaultProviderId: config.DefaultProviderID,
		MaxConcurrentRuns: int32(config.MaxConcurrentRuns),
	}
	for _, provider := range config.Providers {
		result.Providers = append(result.Providers, &executorv1.ExecutorProvider{
			Id:        provider.ID,
			Name:      provider.Name,
			Kind:      provider.Kind,
			BaseUrl:   provider.BaseURL,
			Model:     provider.Model,
			Enabled:   provider.Enabled,
			ApiKeySet: provider.APIKeyRef != "",
		})
	}
	return result, nil
}

// UpdateConfig applies runtime settings. Provider API keys are stored in the
// executor vault once and can never be read back over the wire.
func (s *Service) UpdateConfig(ctx context.Context, request *executorv1.UpdateConfigRequest) (*executorv1.ExecutorConfig, error) {
	incoming := request.GetConfig()
	if incoming == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()

	current := s.runtime.Get()
	next := RuntimeConfig{
		LLMMode:           domain.ExecutorLLMMode(incoming.GetLlmMode()),
		DefaultProviderID: incoming.GetDefaultProviderId(),
		MaxConcurrentRuns: int(incoming.GetMaxConcurrentRuns()),
	}
	if next.LLMMode != domain.ExecutorLLMLocal && next.LLMMode != domain.ExecutorLLMProxy {
		return nil, status.Error(codes.InvalidArgument, "llmMode must be \"proxy\" or \"local\"")
	}
	if next.MaxConcurrentRuns < 1 || next.MaxConcurrentRuns > 64 {
		return nil, status.Error(codes.InvalidArgument, "maxConcurrentRuns must be between 1 and 64")
	}
	for _, provider := range incoming.GetProviders() {
		stored := RuntimeProvider{
			ID:      provider.GetId(),
			Name:    provider.GetName(),
			Kind:    provider.GetKind(),
			BaseURL: provider.GetBaseUrl(),
			Model:   provider.GetModel(),
			Enabled: provider.GetEnabled(),
		}
		stored.ID = strings.TrimSpace(stored.ID)
		if stored.ID == "" {
			return nil, status.Error(codes.InvalidArgument, "provider id is required")
		}
		// Keep the previously stored key unless a new one is supplied. An
		// empty api_key never clears an existing credential; removal happens
		// by deleting the provider entry.
		if key := provider.GetApiKey(); key != "" {
			ref := "executor-provider:" + stored.ID
			if err := s.vault.Put(ref, key); err != nil {
				return nil, status.Errorf(codes.Internal, "store provider key: %v", err)
			}
			stored.APIKeyRef = ref
		} else if existing := findProvider(current.Providers, stored.ID); existing != nil {
			stored.APIKeyRef = existing.APIKeyRef
		}
		next.Providers = append(next.Providers, stored)
	}
	if err := s.runtime.Set(next); err != nil {
		return nil, status.Errorf(codes.Internal, "persist config: %v", err)
	}
	return s.GetConfig(ctx, &executorv1.GetConfigRequest{})
}

func findProvider(providers []RuntimeProvider, id string) *RuntimeProvider {
	for index := range providers {
		if providers[index].ID == id {
			return &providers[index]
		}
	}
	return nil
}

func (s *Service) DeployPipeline(ctx context.Context, request *executorv1.DeployPipelineRequest) (*executorv1.DeployPipelineResponse, error) {
	if request.GetPipelineId() == "" {
		return nil, status.Error(codes.InvalidArgument, "pipelineId is required")
	}
	bundle := DeployedPipeline{
		PipelineID:             request.GetPipelineId(),
		Name:                   request.GetName(),
		Description:            request.GetDescription(),
		Icon:                   request.GetIcon(),
		Revision:               request.GetRevision(),
		BlueprintSchemaVersion: request.GetBlueprintSchemaVersion(),
		Capabilities:           append([]string(nil), request.GetCapabilities()...),
	}
	if bundle.BlueprintSchemaVersion != domain.GraphSchemaV3 {
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported Blueprint schema version %d", bundle.BlueprintSchemaVersion)
	}
	if err := unmarshalJSON(request.GetDefinitionJson(), &bundle.Definition); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "decode definition: %v", err)
	}
	for _, data := range request.GetFunctionsJson() {
		var function domain.CustomFunction
		if err := unmarshalJSON(data, &function); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "decode function: %v", err)
		}
		bundle.Functions = append(bundle.Functions, function)
	}
	for _, trigger := range request.GetTriggers() {
		bundle.Triggers = append(bundle.Triggers, DeployedTriggerSnapshot{
			BindingID:      trigger.GetBindingId(),
			NodeID:         trigger.GetNodeId(),
			NodeType:       trigger.GetNodeType(),
			Kind:           trigger.GetKind(),
			Label:          trigger.GetLabel(),
			Enabled:        trigger.GetEnabled(),
			Trusted:        trigger.GetTrusted(),
			Cron:           trigger.GetCron(),
			Timezone:       trigger.GetTimezone(),
			WatchPath:      trigger.GetWatchPath(),
			WatchRecursive: trigger.GetWatchRecursive(),
		})
	}
	if err := s.store.Deploy(bundle); err != nil {
		return nil, status.Errorf(codes.Internal, "persist deployment: %v", err)
	}
	if s.schedules != nil {
		if err := s.schedules.Reload(); err != nil {
			return nil, status.Errorf(codes.Internal, "reload schedules: %v", err)
		}
	}
	return &executorv1.DeployPipelineResponse{}, nil
}

func (s *Service) UndeployPipeline(ctx context.Context, request *executorv1.UndeployPipelineRequest) (*executorv1.UndeployPipelineResponse, error) {
	if request.GetPipelineId() == "" {
		return nil, status.Error(codes.InvalidArgument, "pipelineId is required")
	}
	if err := s.store.Undeploy(request.GetPipelineId()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove deployment: %v", err)
	}
	if s.schedules != nil {
		if err := s.schedules.Reload(); err != nil {
			return nil, status.Errorf(codes.Internal, "reload schedules: %v", err)
		}
	}
	return &executorv1.UndeployPipelineResponse{}, nil
}

func (s *Service) ListDeployed(ctx context.Context, _ *executorv1.ListDeployedRequest) (*executorv1.ListDeployedResponse, error) {
	response := &executorv1.ListDeployedResponse{}
	for _, bundle := range s.store.ListBundles() {
		info := &executorv1.DeployedPipelineInfo{
			PipelineId:   bundle.PipelineID,
			Name:         bundle.Name,
			Revision:     bundle.Revision,
			TriggerCount: int32(len(bundle.Triggers)),
			DeployedAt:   remoteexec.TimestampOrNull(bundle.DeployedAt),
		}
		response.Pipelines = append(response.Pipelines, info)
	}
	return response, nil
}

// StartRun begins one asynchronous run and returns immediately.
func (s *Service) StartRun(ctx context.Context, request *executorv1.StartRunRequest) (*executorv1.StartRunResponse, error) {
	if request.GetExecutionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "executionId is required")
	}
	var definition domain.FlowDefinition
	capabilities := make([]string, 0)
	name := ""
	var revision int64
	if len(request.GetEmbeddedDefinitionJson()) > 0 {
		// Draft runs execute ephemerally and are never persisted as bundles.
		if err := unmarshalJSON(request.GetEmbeddedDefinitionJson(), &definition); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "decode embedded definition: %v", err)
		}
	} else {
		bundle, ok := s.store.GetBundle(request.GetPipelineId())
		if !ok {
			return nil, status.Errorf(codes.NotFound, "pipeline %q is not deployed on this executor", request.GetPipelineId())
		}
		if request.GetRevision() != 0 && request.GetRevision() != bundle.Revision {
			return nil, status.Errorf(codes.FailedPrecondition, "deployed revision is v%d; requested v%d", bundle.Revision, request.GetRevision())
		}
		definition = bundle.Definition
		capabilities = bundle.Capabilities
		name = bundle.Name
		revision = bundle.Revision
	}
	var input pipeline.Packet
	if len(request.GetInputJson()) > 0 {
		if err := unmarshalJSON(request.GetInputJson(), &input); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "decode input: %v", err)
		}
	}
	record := RunRecord{
		ExecutionID:      request.GetExecutionId(),
		PipelineID:       request.GetPipelineId(),
		Name:             name,
		Revision:         revision,
		TriggerNodeID:    request.GetTriggerNodeId(),
		TriggerBindingID: request.GetTriggerBindingId(),
		ChatRunID:        request.GetChatRunId(),
		Status:           domain.RunPending,
		StartedAt:        time.Now().UTC(),
	}
	if err := s.runner.Enqueue(record, definition, request.GetTriggerNodeId(), input, request.GetUnattended(), capabilities); err != nil {
		return nil, status.Errorf(codes.ResourceExhausted, "%v", err)
	}
	return &executorv1.StartRunResponse{}, nil
}

func (s *Service) CancelRun(ctx context.Context, request *executorv1.CancelRunRequest) (*executorv1.CancelRunResponse, error) {
	record, ok := s.store.GetRun(request.GetExecutionId())
	if !ok {
		return nil, status.Error(codes.NotFound, "execution not found")
	}
	switch record.Status {
	case domain.RunCompleted, domain.RunFailed, domain.RunCancelled:
		return &executorv1.CancelRunResponse{}, nil
	default:
		record.Status = domain.RunCancelled
		record.Error = "Cancelled by user"
		finished := time.Now().UTC()
		record.FinishedAt = &finished
		_ = s.store.SaveRun(record)
		s.runner.publish(runEventFromRecord(record))
		s.runner.CancelExecution(request.GetExecutionId())
		return &executorv1.CancelRunResponse{}, nil
	}
}

func (s *Service) GetRun(ctx context.Context, request *executorv1.GetRunRequest) (*executorv1.RunSnapshot, error) {
	record, ok := s.store.GetRun(request.GetExecutionId())
	if !ok {
		return nil, status.Error(codes.NotFound, "execution not found")
	}
	return runSnapshotFromRecord(record), nil
}

const defaultRecentRunsLimit = 50

func (s *Service) ListRuns(ctx context.Context, request *executorv1.ListRunsRequest) (*executorv1.ListRunsResponse, error) {
	limit := int(request.GetLimit())
	if limit <= 0 {
		limit = defaultRecentRunsLimit
	}
	response := &executorv1.ListRunsResponse{}
	for _, record := range s.store.RecentRuns(limit) {
		response.Runs = append(response.Runs, runSnapshotFromRecord(record))
	}
	return response, nil
}

// StreamEvents pushes lifecycle events until the client goes away.
func (s *Service) StreamEvents(_ *executorv1.StreamEventsRequest, stream executorv1.Executor_StreamEventsServer) error {
	events := s.runner.Subscribe()
	defer s.runner.unsubscribe(events)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// Tunnel serves the reverse host-service channel opened by the desktop.
func (s *Service) Tunnel(stream executorv1.Executor_TunnelServer) error {
	done := make(chan error, 1)
	go func() {
		s.tunnel.Attach(stream)
		done <- nil
	}()
	// Attach returns when Recv fails (session end). Block on either the
	// attach loop finishing or the stream context closing.
	select {
	case <-done:
		return nil
	case <-stream.Context().Done():
		return nil
	}
}
