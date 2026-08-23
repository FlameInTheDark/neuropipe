// Executor management for the desktop façade: registration CRUD, connection
// lifecycle, deployment of published pipelines, and reconciliation. Secrets
// stay in the desktop vault; only vault references are persisted or shown.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/execution"
	"github.com/FlameInTheDark/neuropipe/internal/llm"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	remoteexec "github.com/FlameInTheDark/neuropipe/internal/remoteexec"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/google/uuid"
)

const executorTokenPrefix = "executor-token:"

// executorBridge answers reverse host-service calls from remote executors
// with the same local services a desktop-local run would use, so provider
// keys, database credentials, Twitch OAuth, reports, and conversations all
// remain on the user's machine.
type executorBridge struct {
	providers *llm.Manager
	store     *persistence.Store
	databases nodes.SQLExecutor
	twitch    nodes.TwitchChatSender
	emit      func(event string)
}

func (b *executorBridge) LLMChat(ctx context.Context, request pipeline.ChatRequest) (pipeline.ChatResponse, error) {
	if b.providers == nil {
		return pipeline.ChatResponse{}, fmt.Errorf("no LLM provider is configured in Neuropipe")
	}
	return b.providers.Chat(ctx, request)
}

func (b *executorBridge) LLMConverse(ctx context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	if b.providers == nil {
		return domain.AssistantChatResponse{}, fmt.Errorf("no LLM provider is configured in Neuropipe")
	}
	return b.providers.Converse(ctx, request)
}

func (b *executorBridge) CreateReport(ctx context.Context, report domain.Report) (domain.Report, error) {
	created, err := b.store.CreateReport(ctx, report)
	if err == nil && b.emit != nil {
		b.emit("reports.updated")
	}
	return created, err
}

func (b *executorBridge) GetReport(ctx context.Context, id string) (domain.Report, error) {
	return b.store.GetReport(ctx, id)
}

func (b *executorBridge) ListReports(ctx context.Context, limit int) ([]domain.Report, error) {
	return b.store.ListReports(ctx, limit)
}

func (b *executorBridge) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	return b.store.AppendChatReply(ctx, chatRunID, content)
}

func (b *executorBridge) UpdateChatStatus(ctx context.Context, chatRunID, status string) error {
	return b.store.UpdateChatStatus(ctx, chatRunID, status)
}

func (b *executorBridge) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	return b.store.ReadChatHistory(ctx, chatID, limit)
}

func (b *executorBridge) ExecuteSQL(ctx context.Context, request domain.SQLRequest) (domain.SQLResult, error) {
	if b.databases == nil {
		return domain.SQLResult{}, fmt.Errorf("no databases are registered in Neuropipe")
	}
	return b.databases.ExecuteSQL(ctx, request)
}

func (b *executorBridge) SendTwitchChatMessage(ctx context.Context, request domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error) {
	if b.twitch == nil {
		return domain.TwitchChatMessageResult{}, fmt.Errorf("Twitch is not connected in Neuropipe")
	}
	return b.twitch.SendTwitchChatMessage(ctx, request)
}

var _ remoteexec.HostBridge = (*executorBridge)(nil)

// remoteDispatcher adapts the connection manager to the execution service's
// narrow dispatch contract.
type executorDispatch struct{ manager *remoteexec.Manager }

func (e executorDispatch) Dispatch(ctx context.Context, executorID string, run execution.RemoteDispatch) error {
	return e.manager.StartRun(ctx, executorID, remoteexec.StartSpec{
		ExecutionID:            run.ExecutionID,
		PipelineID:             run.PipelineID,
		TriggerNodeID:          run.TriggerNodeID,
		TriggerBindingID:       run.TriggerBindingID,
		ChatRunID:              run.ChatRunID,
		Unattended:             run.Unattended,
		Input:                  run.Input,
		EmbeddedDefinition:     run.EmbeddedDefinition,
		BlueprintSchemaVersion: domain.GraphSchemaV3,
	})
}

func (e executorDispatch) CancelRun(ctx context.Context, executorID, executionID string) error {
	return e.manager.Cancel(ctx, executorID, executionID)
}

// ListRemoteExecutors returns every registered executor with its cached
// connectivity status.
func (d *Desktop) ListRemoteExecutors() ([]domain.RemoteExecutorListItem, error) {
	executors, err := d.store.ListRemoteExecutors(d.context())
	if err != nil {
		return nil, err
	}
	result := make([]domain.RemoteExecutorListItem, 0, len(executors))
	for _, item := range executors {
		entry := domain.RemoteExecutorListItem{RemoteExecutor: item, Status: d.remote.CachedStatus(item.ID)}
		result = append(result, entry)
	}
	return result, nil
}

// AddRemoteExecutor registers an executor. When no token is supplied one is
// generated; the returned Token is displayed exactly once.
func (d *Desktop) AddRemoteExecutor(request domain.SaveRemoteExecutorRequest) (domain.ExecutorCreateResult, error) {
	name := strings.TrimSpace(request.Name)
	address := strings.TrimSpace(request.Address)
	if name == "" || address == "" {
		return domain.ExecutorCreateResult{}, fmt.Errorf("an executor name and address are required")
	}
	token := strings.TrimSpace(request.Token)
	if token == "" {
		generated, err := generateExecutorToken()
		if err != nil {
			return domain.ExecutorCreateResult{}, err
		}
		token = generated
	}
	now := time.Now().UTC()
	item := domain.RemoteExecutor{
		ID:        uuid.NewString(),
		Name:      name,
		Address:   address,
		TokenRef:  executorTokenPrefix + uuid.NewString(),
		UseTLS:    request.UseTLS,
		LLMMode:   domain.ExecutorLLMProxy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := d.vault.Put(item.TokenRef, token); err != nil {
		return domain.ExecutorCreateResult{}, fmt.Errorf("store executor token: %w", err)
	}
	if err := d.store.SaveRemoteExecutor(d.context(), item); err != nil {
		_ = d.vault.Delete(item.TokenRef)
		return domain.ExecutorCreateResult{}, err
	}
	if err := d.remote.Ensure(remoteexec.Target{ID: item.ID, Address: item.Address, TokenRef: item.TokenRef, UseTLS: item.UseTLS}); err != nil {
		d.emit("executor.status.updated", map[string]any{"id": item.ID, "status": domain.RemoteExecutorStatus{Online: false, Message: err.Error()}})
	}
	return domain.ExecutorCreateResult{Executor: item, Token: token}, nil
}

// UpdateRemoteExecutor changes name/address/TLS settings and reopens the
// supervised connection.
func (d *Desktop) UpdateRemoteExecutor(request domain.SaveRemoteExecutorRequest) (domain.RemoteExecutor, error) {
	item, err := d.store.GetRemoteExecutor(d.context(), request.ID)
	if err != nil {
		return domain.RemoteExecutor{}, err
	}
	if name := strings.TrimSpace(request.Name); name != "" {
		item.Name = name
	}
	if address := strings.TrimSpace(request.Address); address != "" {
		item.Address = address
	}
	item.UseTLS = request.UseTLS
	if err := d.store.SaveRemoteExecutor(d.context(), item); err != nil {
		return domain.RemoteExecutor{}, err
	}
	if err := d.remote.Ensure(remoteexec.Target{ID: item.ID, Address: item.Address, TokenRef: item.TokenRef, UseTLS: item.UseTLS}); err != nil {
		return item, err
	}
	return item, nil
}

// RemoveRemoteExecutor detaches its pipelines, forgets the registration, and
// deletes the shared secret from the vault.
func (d *Desktop) RemoveRemoteExecutor(id string) error {
	if _, err := d.store.GetRemoteExecutor(d.context(), id); err != nil {
		return err
	}
	if err := d.store.DetachExecutorFromPipelines(d.context(), id); err != nil {
		return err
	}
	item, _ := d.store.GetRemoteExecutor(d.context(), id)
	d.remote.Remove(id)
	if err := d.store.DeleteRemoteExecutor(d.context(), id); err != nil {
		return err
	}
	if item.TokenRef != "" {
		_ = d.vault.Delete(item.TokenRef)
	}
	return nil
}

// RotateExecutorToken replaces the shared secret in the vault and returns
// the new value once so it can be installed on the executor machine.
func (d *Desktop) RotateExecutorToken(id string) (string, error) {
	item, err := d.store.GetRemoteExecutor(d.context(), id)
	if err != nil {
		return "", err
	}
	token, err := generateExecutorToken()
	if err != nil {
		return "", err
	}
	if err := d.vault.Put(item.TokenRef, token); err != nil {
		return "", fmt.Errorf("store executor token: %w", err)
	}
	return token, nil
}

// GetRemoteExecutorStatus performs an immediate reachability check.
func (d *Desktop) GetRemoteExecutorStatus(id string) (domain.RemoteExecutorStatus, error) {
	status, err := d.remote.PingStatus(id)
	if err != nil && status.Message == "" {
		status.Message = err.Error()
	}
	return status, nil
}

// GetRemoteExecutorConfig reads the RPC-managed runtime configuration.
func (d *Desktop) GetRemoteExecutorConfig(id string) (domain.RemoteExecutorConfig, error) {
	return d.remote.Config(d.context(), id)
}

// UpdateRemoteExecutorConfig applies runtime configuration on the executor.
// The chosen LLM mode is also mirrored into the local registration row so
// the pipelines list can render consistent badges offline.
func (d *Desktop) UpdateRemoteExecutorConfig(id string, config domain.RemoteExecutorConfig) (domain.RemoteExecutorConfig, error) {
	applied, err := d.remote.UpdateConfig(d.context(), id, config)
	if err != nil {
		return domain.RemoteExecutorConfig{}, err
	}
	if item, getErr := d.store.GetRemoteExecutor(d.context(), id); getErr == nil && item.LLMMode != applied.LLMMode {
		item.LLMMode = applied.LLMMode
		_ = d.store.SaveRemoteExecutor(d.context(), item)
	}
	return applied, nil
}

// SyncPipelineToExecutor deploys the pipeline's current published revision
// immediately. It is the retry action for a failed publish-time deploy.
func (d *Desktop) SyncPipelineToExecutor(pipelineID string) error {
	return d.DeployPipelineToExecutor(pipelineID)
}

// DeployPipelineToExecutor pushes the published revision bundle to the
// pipeline's target executor.
func (d *Desktop) DeployPipelineToExecutor(pipelineID string) error {
	ctx := d.context()
	item, err := d.store.GetPipeline(ctx, pipelineID)
	if err != nil {
		return err
	}
	if item.ExecutorID == "" {
		return fmt.Errorf("pipeline %q does not target a remote executor", item.Name)
	}
	if item.Status != domain.PipelineActive || item.PublishedRevision < 1 {
		return fmt.Errorf("pipeline %q has no published revision to deploy", item.Name)
	}
	request, err := d.buildDeployRequest(ctx, item, int64(item.PublishedRevision))
	if err != nil {
		return err
	}
	if err := d.remote.Deploy(ctx, item.ExecutorID, request); err != nil {
		return fmt.Errorf("deploy to executor: %w", err)
	}
	return nil
}

// buildDeployRequest assembles the full bundle: definition, recursively
// resolved custom functions, autonomous trigger bindings with their trust
// state, and the capability allowlist captured at publish time.
func (d *Desktop) buildDeployRequest(ctx context.Context, item domain.Pipeline, revision int64) (remoteexec.DeployRequest, error) {
	definition, err := d.store.PublishedDefinition(ctx, item.ID, int(revision))
	if err != nil {
		return remoteexec.DeployRequest{}, err
	}
	functions, err := d.resolveFunctionBundle(ctx, definition)
	if err != nil {
		return remoteexec.DeployRequest{}, err
	}
	bindings, err := d.store.ListTriggersByPipeline(ctx, item.ID)
	if err != nil {
		return remoteexec.DeployRequest{}, err
	}
	triggers := make([]remoteexec.DeployTrigger, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Revision != int(revision) {
			continue
		}
		switch binding.Kind {
		case domain.TriggerCron, domain.TriggerFile:
			triggers = append(triggers, remoteexec.DeployTrigger{
				BindingID: binding.ID,
				NodeID:    binding.NodeID,
				NodeType:  binding.NodeType,
				Kind:      binding.Kind,
				Label:     binding.Label,
				Enabled:   binding.Enabled,
				Trusted:   binding.Trusted,
				Cron:      binding.Cron,
				Timezone:  binding.Timezone,
			})
		default:
			// Button, hotkey, webhook, chat, and Twitch triggers stay
			// desktop-hosted; each run is dispatched over gRPC instead.
		}
	}
	return remoteexec.DeployRequest{
		PipelineID:             item.ID,
		Name:                   item.Name,
		Description:            item.Description,
		Icon:                   item.Icon,
		Revision:               revision,
		BlueprintSchemaVersion: domain.GraphSchemaV3,
		Definition:             definition,
		Functions:              functions,
		Triggers:               triggers,
		Capabilities:           security.RequiredCapabilities(definition, d.registry),
	}, nil
}

// resolveFunctionBundle collects every published custom function referenced
// by the definition, following nested function calls transitively.
func (d *Desktop) resolveFunctionBundle(ctx context.Context, definition domain.FlowDefinition) ([]domain.CustomFunction, error) {
	resolved := make(map[string]domain.CustomFunction)
	var visit func(definition domain.FlowDefinition) error
	visit = func(definition domain.FlowDefinition) error {
		for _, node := range definition.Nodes {
			if !strings.HasPrefix(node.Type, "function:") || isFunctionBoundary(node.Type) {
				continue
			}
			functionID := strings.TrimPrefix(node.Type, "function:")
			if _, done := resolved[functionID]; done {
				continue
			}
			function, err := d.store.GetPublishedFunction(ctx, functionID)
			if err != nil {
				return fmt.Errorf("resolve function %q for deployment: %w", functionID, err)
			}
			resolved[functionID] = function
			if err := visit(function.DraftDefinition); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(definition); err != nil {
		return nil, err
	}
	functions := make([]domain.CustomFunction, 0, len(resolved))
	for _, function := range resolved {
		functions = append(functions, function)
	}
	return functions, nil
}

// reconcileExecutor syncs deployments after an executor reconnects: missing
// or outdated bundles are re-deployed, orphaned bundles removed, and runs
// that finished while disconnected are adopted into local history.
func (d *Desktop) reconcileExecutor(executorID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pipelines, err := d.store.ListPipelines(ctx)
	if err != nil {
		return
	}
	targeted := make([]domain.PipelineSummary, 0)
	for _, item := range pipelines {
		if item.ExecutorID == executorID && item.Status == domain.PipelineActive && item.PublishedRevision > 0 {
			targeted = append(targeted, item)
		}
	}
	deployed, err := d.remote.ListDeployed(ctx, executorID)
	if err != nil {
		return
	}
	deployedRevisions := make(map[string]int64, len(deployed))
	for _, info := range deployed {
		deployedRevisions[info.PipelineID] = info.Revision
	}
	for _, item := range targeted {
		want := int64(item.PublishedRevision)
		if have, exists := deployedRevisions[item.ID]; exists && have == want {
			continue
		}
		full, err := d.store.GetPipeline(ctx, item.ID)
		if err != nil {
			continue
		}
		request, err := d.buildDeployRequest(ctx, full, want)
		if err != nil {
			continue
		}
		if err := d.remote.Deploy(ctx, executorID, request); err != nil {
			return // connection dropped again; next reconnect retries
		}
	}
	for _, info := range deployed {
		found := false
		for _, item := range targeted {
			if item.ID == info.PipelineID {
				found = true
				break
			}
		}
		if !found {
			_ = d.remote.Undeploy(ctx, executorID, info.PipelineID)
		}
	}
	runs, err := d.remote.RecentRuns(ctx, executorID, recentRunsReconcileLimit)
	if err != nil {
		return
	}
	for _, run := range runs {
		stored, err := d.store.GetExecution(ctx, run.ID)
		if err != nil {
			// Unknown to this workspace: an autonomous schedule fire that
			// happened entirely on the executor. Adopt it into history.
			_ = d.store.AdoptRemoteExecution(ctx, run)
			continue
		}
		// A dispatched run may have finished while the connection was down;
		// refresh it so it never stays "running" locally forever.
		if !isRunTerminal(stored.Status) && isRunTerminal(run.Status) {
			d.runs.ApplyRemoteRunUpdate(run)
		}
	}
}

// isRunTerminal reports whether an execution reached a final state.
func isRunTerminal(status domain.RunStatus) bool {
	switch status {
	case domain.RunCompleted, domain.RunFailed, domain.RunCancelled, domain.RunSkipped:
		return true
	default:
		return false
	}
}

const recentRunsReconcileLimit = 100

// generateExecutorToken creates a 256-bit shared secret.
func generateExecutorToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate executor token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// TestRemoteExecutor dials an unregistered endpoint once to verify the
// address and token before saving anything.
func (d *Desktop) TestRemoteExecutor(address, token string, useTLS bool) (domain.RemoteExecutorStatus, error) {
	ctx, cancel := context.WithTimeout(d.context(), 10*time.Second)
	defer cancel()
	probe, err := remoteexec.ProbeExecutor(ctx, strings.TrimSpace(address), strings.TrimSpace(token), useTLS)
	if err != nil {
		return domain.RemoteExecutorStatus{Online: false}, err
	}
	return domain.RemoteExecutorStatus{
		Online:        true,
		Version:       probe.Version,
		Platform:      probe.Platform,
		ActiveRuns:    probe.ActiveRuns,
		MaxConcurrent: probe.MaxConcurrent,
	}, nil
}
