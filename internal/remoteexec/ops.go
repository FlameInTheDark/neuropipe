package remoteexec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"google.golang.org/grpc"
)

// DeployTrigger carries one autonomous trigger binding inside a deployment.
type DeployTrigger struct {
	BindingID      string
	NodeID         string
	NodeType       string
	Kind           domain.TriggerKind
	Label          string
	Enabled        bool
	Trusted        bool
	Cron           string
	Timezone       string
	WatchPath      string
	WatchRecursive bool
}

// DeployRequest is the full publication bundle pushed to one executor.
type DeployRequest struct {
	PipelineID             string
	Name                   string
	Description            string
	Icon                   string
	IconColor              string
	IconBackground         string
	Revision               int64
	BlueprintSchemaVersion int32
	Definition             domain.FlowDefinition
	Functions              []domain.CustomFunction
	Triggers               []DeployTrigger
	Capabilities           []domain.Capability
}

// DeployedInfo mirrors one executor-side deployment for reconciliation.
type DeployedInfo struct {
	PipelineID   string
	Name         string
	Revision     int64
	TriggerCount int
	DeployedAt   time.Time
}

// StartSpec describes one dispatched run.
type StartSpec struct {
	ExecutionID            string
	PipelineID             string
	Revision               int64
	TriggerNodeID          string
	TriggerBindingID       string
	ChatRunID              string
	Unattended             bool
	Input                  pipeline.Packet
	EmbeddedDefinition     *domain.FlowDefinition
	BlueprintSchemaVersion int32
}

// PingStatus performs an immediate reachability check and returns fresh
// status data without waiting for the supervisor's next health tick.
func (m *Manager) PingStatus(id string) (domain.RemoteExecutorStatus, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return domain.RemoteExecutorStatus{}, err
	}
	client := connection.currentClient()
	if client == nil {
		return connection.snapshotStatus(), ErrOffline
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	response, err := client.GetStatus(ctx, &executorv1.StatusRequest{})
	if err != nil {
		return domain.RemoteExecutorStatus{Online: false, Message: friendlyError(err)}, err
	}
	status := domain.RemoteExecutorStatus{
		Online:        true,
		Version:       response.GetExecutorVersion(),
		Platform:      response.GetPlatform(),
		ActiveRuns:    int(response.GetActiveRuns()),
		MaxConcurrent: int(response.GetMaxConcurrentRuns()),
	}
	connection.setStatus(status)
	m.notifyStatus(id, status)
	return status, nil
}

// Deploy pushes a published bundle to one executor.
func (m *Manager) Deploy(ctx context.Context, id string, request DeployRequest) error {
	connection, err := m.connFor(id)
	if err != nil {
		return err
	}
	definitionJSON, err := encode(request.Definition)
	if err != nil {
		return err
	}
	functions := make([][]byte, 0, len(request.Functions))
	for _, function := range request.Functions {
		data, err := encode(function)
		if err != nil {
			return fmt.Errorf("encode function %q: %w", function.Name, err)
		}
		functions = append(functions, data)
	}
	protoRequest := &executorv1.DeployPipelineRequest{
		PipelineId:             request.PipelineID,
		Name:                   request.Name,
		Description:            request.Description,
		Icon:                   request.Icon,
		IconColor:              request.IconColor,
		IconBackground:         request.IconBackground,
		Revision:               request.Revision,
		BlueprintSchemaVersion: request.BlueprintSchemaVersion,
		DefinitionJson:         definitionJSON,
		FunctionsJson:          functions,
		Triggers:               make([]*executorv1.DeployedTrigger, 0, len(request.Triggers)),
		Capabilities:           capabilitiesToStrings(request.Capabilities),
	}
	for _, trigger := range request.Triggers {
		protoRequest.Triggers = append(protoRequest.Triggers, &executorv1.DeployedTrigger{
			BindingId:      trigger.BindingID,
			NodeId:         trigger.NodeID,
			NodeType:       trigger.NodeType,
			Kind:           string(trigger.Kind),
			Label:          trigger.Label,
			Enabled:        trigger.Enabled,
			Trusted:        trigger.Trusted,
			Cron:           trigger.Cron,
			Timezone:       trigger.Timezone,
			WatchPath:      trigger.WatchPath,
			WatchRecursive: trigger.WatchRecursive,
		})
	}
	client := connection.currentClient()
	if client == nil {
		return ErrOffline
	}
	_, err = client.DeployPipeline(ctx, protoRequest)
	return err
}

// Undeploy removes one deployed bundle.
func (m *Manager) Undeploy(ctx context.Context, id, pipelineID string) error {
	connection, err := m.connFor(id)
	if err != nil {
		return err
	}
	client := connection.currentClient()
	if client == nil {
		return ErrOffline
	}
	_, err = client.UndeployPipeline(ctx, &executorv1.UndeployPipelineRequest{PipelineId: pipelineID})
	return err
}

// ListDeployed returns executor-side deployment identities.
func (m *Manager) ListDeployed(ctx context.Context, id string) ([]DeployedInfo, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return nil, err
	}
	client := connection.currentClient()
	if client == nil {
		return nil, ErrOffline
	}
	response, err := client.ListDeployed(ctx, &executorv1.ListDeployedRequest{})
	if err != nil {
		return nil, err
	}
	list := make([]DeployedInfo, 0, len(response.GetPipelines()))
	for _, item := range response.GetPipelines() {
		info := DeployedInfo{
			PipelineID:   item.GetPipelineId(),
			Name:         item.GetName(),
			Revision:     item.GetRevision(),
			TriggerCount: int(item.GetTriggerCount()),
		}
		if item.GetDeployedAt() != nil {
			info.DeployedAt = item.GetDeployedAt().AsTime()
		}
		list = append(list, info)
	}
	return list, nil
}

// StartRun dispatches one run. It fails fast when the executor is offline so
// the execution record completes with an explicit, localized error.
func (m *Manager) StartRun(ctx context.Context, id string, spec StartSpec) error {
	connection, err := m.connFor(id)
	if err != nil {
		return err
	}
	inputJSON := []byte("{}")
	if spec.Input != nil {
		if inputJSON, err = encode(spec.Input); err != nil {
			return err
		}
	}
	request := &executorv1.StartRunRequest{
		ExecutionId:            spec.ExecutionID,
		PipelineId:             spec.PipelineID,
		Revision:               spec.Revision,
		TriggerNodeId:          spec.TriggerNodeID,
		TriggerBindingId:       spec.TriggerBindingID,
		Unattended:             spec.Unattended,
		ChatRunId:              spec.ChatRunID,
		InputJson:              inputJSON,
		BlueprintSchemaVersion: spec.BlueprintSchemaVersion,
	}
	if spec.EmbeddedDefinition != nil {
		data, encodeErr := encode(*spec.EmbeddedDefinition)
		if encodeErr != nil {
			return encodeErr
		}
		request.EmbeddedDefinitionJson = data
	}
	client := connection.currentClient()
	if client == nil {
		return ErrOffline
	}
	if _, err = client.StartRun(ctx, request); err != nil {
		return fmt.Errorf("start remote run: %w", err)
	}
	return nil
}

// Cancel cancels one executor-side run.
func (m *Manager) Cancel(ctx context.Context, id, executionID string) error {
	connection, err := m.connFor(id)
	if err != nil {
		return err
	}
	client := connection.currentClient()
	if client == nil {
		return ErrOffline
	}
	_, err = client.CancelRun(ctx, &executorv1.CancelRunRequest{ExecutionId: executionID})
	return err
}

// Run fetches one executor-side execution snapshot.
func (m *Manager) Run(ctx context.Context, id, executionID string) (domain.Execution, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return domain.Execution{}, err
	}
	client := connection.currentClient()
	if client == nil {
		return domain.Execution{}, ErrOffline
	}
	snapshot, err := client.GetRun(ctx, &executorv1.GetRunRequest{ExecutionId: executionID})
	if err != nil {
		return domain.Execution{}, err
	}
	return ExecutionFromSnapshot(snapshot, id), nil
}

// RecentRuns lists recent executor-side runs for reconnect reconciliation.
func (m *Manager) RecentRuns(ctx context.Context, id string, limit int) ([]domain.Execution, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return nil, err
	}
	client := connection.currentClient()
	if client == nil {
		return nil, ErrOffline
	}
	response, err := client.ListRuns(ctx, &executorv1.ListRunsRequest{Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	runs := make([]domain.Execution, 0, len(response.GetRuns()))
	for _, snapshot := range response.GetRuns() {
		runs = append(runs, ExecutionFromSnapshot(snapshot, id))
	}
	return runs, nil
}

// Config reads the RPC-managed runtime configuration. Provider API keys are
// never included; only whether one has been set.
func (m *Manager) Config(ctx context.Context, id string) (domain.RemoteExecutorConfig, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return domain.RemoteExecutorConfig{}, err
	}
	client := connection.currentClient()
	if client == nil {
		return domain.RemoteExecutorConfig{}, ErrOffline
	}
	response, err := client.GetConfig(ctx, &executorv1.GetConfigRequest{})
	if err != nil {
		return domain.RemoteExecutorConfig{}, err
	}
	return configFromProto(response), nil
}

// UpdateConfig applies runtime configuration. Provider API keys present in
// the payload are stored once in the executor's vault and cannot be read back.
func (m *Manager) UpdateConfig(ctx context.Context, id string, config domain.RemoteExecutorConfig) (domain.RemoteExecutorConfig, error) {
	connection, err := m.connFor(id)
	if err != nil {
		return domain.RemoteExecutorConfig{}, err
	}
	client := connection.currentClient()
	if client == nil {
		return domain.RemoteExecutorConfig{}, ErrOffline
	}
	response, err := client.UpdateConfig(ctx, &executorv1.UpdateConfigRequest{Config: configToProto(config)})
	if err != nil {
		return domain.RemoteExecutorConfig{}, err
	}
	return configFromProto(response), nil
}

// ExecutorProbe is a one-shot reachability result for an endpoint that is
// not registered yet (Settings "test connection").
type ExecutorProbe struct {
	Version       string
	Platform      string
	ActiveRuns    int
	MaxConcurrent int
}

// ProbeExecutor dials an executor exactly once without creating a supervised
// connection. Callers own the context timeout.
func ProbeExecutor(ctx context.Context, address, token string, useTLS bool) (*ExecutorProbe, error) {
	connection, err := grpc.NewClient(address, dialOptions(token, useTLS, "")...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = connection.Close() }()
	client := executorv1.NewExecutorClient(connection)
	response, err := client.GetStatus(ctx, &executorv1.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return &ExecutorProbe{
		Version:       response.GetExecutorVersion(),
		Platform:      response.GetPlatform(),
		ActiveRuns:    int(response.GetActiveRuns()),
		MaxConcurrent: int(response.GetMaxConcurrentRuns()),
	}, nil
}

func configToProto(config domain.RemoteExecutorConfig) *executorv1.ExecutorConfig {
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
			ApiKey:    provider.APIKey,
			ApiKeySet: provider.APIKeySet || provider.APIKey != "",
		})
	}
	return result
}

func configFromProto(config *executorv1.ExecutorConfig) domain.RemoteExecutorConfig {
	result := domain.RemoteExecutorConfig{
		LLMMode:           domain.ExecutorLLMMode(config.GetLlmMode()),
		DefaultProviderID: config.GetDefaultProviderId(),
		MaxConcurrentRuns: int(config.GetMaxConcurrentRuns()),
	}
	if result.LLMMode == "" {
		result.LLMMode = domain.ExecutorLLMProxy
	}
	for _, provider := range config.GetProviders() {
		result.Providers = append(result.Providers, domain.RemoteExecutorProvider{
			ID:        provider.GetId(),
			Name:      provider.GetName(),
			Kind:      provider.GetKind(),
			BaseURL:   provider.GetBaseUrl(),
			Model:     provider.GetModel(),
			Enabled:   provider.GetEnabled(),
			APIKeySet: provider.GetApiKeySet(),
		})
	}
	return result
}

func capabilitiesToStrings(capabilities []domain.Capability) []string {
	values := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		values = append(values, string(capability))
	}
	return values
}

var errClosed = errors.New("connection closed")

// currentClient snapshots the RPC client under the status lock; nil means
// the supervisor has not completed a successful open yet.
func (c *conn) currentClient() executorv1.ExecutorClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}
