// Package app contains the sole Wails-facing façade for Neuropipe.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	chatservice "github.com/FlameInTheDark/neuropipe/internal/chat"
	documentation "github.com/FlameInTheDark/neuropipe/internal/documentation"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/execution"
	"github.com/FlameInTheDark/neuropipe/internal/hotkey"
	"github.com/FlameInTheDark/neuropipe/internal/httpapi"
	"github.com/FlameInTheDark/neuropipe/internal/llm"
	"github.com/FlameInTheDark/neuropipe/internal/localization"
	"github.com/FlameInTheDark/neuropipe/internal/metrics"
	javascriptnode "github.com/FlameInTheDark/neuropipe/internal/nodes/code/javascript"
	"github.com/FlameInTheDark/neuropipe/internal/notifications"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/plugins"
	localruntime "github.com/FlameInTheDark/neuropipe/internal/runtime"
	"github.com/FlameInTheDark/neuropipe/internal/scheduler"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	twitchservice "github.com/FlameInTheDark/neuropipe/internal/twitch"
	"github.com/FlameInTheDark/neuropipe/internal/updatecheck"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	runtimeInstallProgressEvent = "runtime.install.progress"
	modelInstallProgressEvent   = "model.install.progress"
	managedLlamaProviderID      = "llama-managed"
)

// Desktop is the only object bound to the Wails renderer.
type Desktop struct {
	ctx                    context.Context
	dataRoot               string
	store                  *persistence.Store
	registry               *catalog.Registry
	vault                  *security.Vault
	providers              *llm.Manager
	metrics                *metrics.Service
	plugins                *plugins.Manager
	documentation          *documentation.Service
	runs                   *execution.Service
	chat                   *chatservice.Service
	hotkeys                *hotkey.Service
	scheduler              *scheduler.Service
	llama                  *localruntime.LlamaManager
	llamaFiles             *localruntime.LlamaCatalog
	models                 *localruntime.ModelCatalog
	progressMu             sync.RWMutex
	runtimeInstallProgress domain.InstallProgress
	modelInstallProgress   domain.InstallProgress
	api                    *httpapi.Server
	twitch                 *twitchservice.Service
	settingsMu             sync.RWMutex
	settings               domain.Settings
	trayMu                 sync.RWMutex
	trayMenu               trayLabelSink
	updates                UpdateChecker
	updateMu               sync.RWMutex
	update                 domain.UpdateAvailability
	updateCancel           context.CancelFunc
	updateWG               sync.WaitGroup
}

// New composes the desktop application from local dependencies.
func New(version string) (*Desktop, error) {
	root, err := appDataRoot()
	if err != nil {
		return nil, err
	}
	store, err := persistence.New(root)
	if err != nil {
		return nil, err
	}
	vault, err := security.NewVault(root)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	pluginDirectory := filepath.Join(root, "plugins")
	settings, err := store.LoadSettings(context.Background(), pluginDirectory)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := normalizeConfiguredProvider(&settings); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := validateAPISettings(&settings); err != nil {
		_ = store.Close()
		return nil, err
	}
	contentDirectory, err := normalizeContentDirectory(settings.ContentDirectory, root)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	settings.ContentDirectory = contentDirectory
	registry := catalog.New()
	pluginManager := plugins.NewManager(settings.PluginDirectory)
	docs, err := documentation.New(pluginManager)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	desktop := &Desktop{dataRoot: root, store: store, registry: registry, vault: vault, settings: settings, plugins: pluginManager, documentation: docs, updates: updatecheck.NewChecker(updatecheck.NewGitHubSource(nil), version)}
	if err := desktop.refreshFunctionRegistry(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	desktop.llama = localruntime.NewLlamaManager()
	desktop.metrics = metrics.NewService(store, desktop.emit, metrics.NewProcessSampler(), desktop.llama.PID)
	desktop.providers = llm.NewManager(settings, vault, llm.WithUsageRecorder(desktop.metrics))
	desktop.runs = execution.NewService(store, registry, desktop.providers, desktop.emit,
		execution.WithNotificationSender(notifications.NewToastSender("Neuropipe")),
		execution.WithMetricsRecorder(desktop.metrics),
	)
	desktop.runs.SetMaxConcurrentRuns(settings.MaxConcurrentRuns)
	desktop.twitch = twitchservice.New(vault, store, desktop.runs, desktop.saveTwitchIdentity, desktop.emit)
	desktop.twitch.Configure(settings.Twitch)
	desktop.runs.SetTwitchChatSender(desktop.twitch)
	desktop.chat = chatservice.NewService(store, desktop.runs, desktop.providers, desktop.emit)
	desktop.scheduler = scheduler.New(store, desktop.runs)
	desktop.hotkeys = hotkey.New(store, desktop.runs)
	desktop.configureContentDirectory(contentDirectory)
	desktop.api = httpapi.New(desktop, vault)
	return desktop, nil
}

// Startup stores the Wails context and activates registered local triggers.
func (d *Desktop) Startup(ctx context.Context) {
	d.ctx = ctx
	d.startUpdateChecks(ctx)
	d.runs.Start(ctx)
	d.chat.Start(ctx)
	d.metrics.Start(ctx, d.settings.Metrics)
	_, _ = d.plugins.Reload()
	_ = d.refreshFunctionRegistry(ctx)
	_ = d.store.PurgeExecutions(ctx, d.settings.RetentionDays)
	_ = d.metrics.Purge(ctx)
	_ = d.scheduler.Start(ctx)
	if err := d.hotkeys.Start(ctx); err != nil {
		d.emit("hotkeys.status.error", err.Error())
	}
	if err := d.api.Configure(ctx, d.GetSettings().API); err != nil {
		d.emit("api.status.error", err.Error())
	}
	d.twitch.Start(ctx)
	if d.settings.LlamaRuntime.AutoStart {
		_, _ = d.StartLlamaRuntime()
	}
}

// Shutdown stops managed workers before closing persistent state.
func (d *Desktop) Shutdown(context.Context) {
	d.stopUpdateChecks()
	_ = d.api.Stop(context.Background())
	d.hotkeys.Stop()
	d.scheduler.Stop()
	d.twitch.Stop()
	d.chat.Stop()
	d.runs.Stop()
	d.metrics.Stop()
	d.llama.Stop()
	_ = d.store.Close()
}

func (d *Desktop) ListPipelines() ([]domain.PipelineSummary, error) {
	return d.store.ListPipelines(d.context())
}

func (d *Desktop) CreatePipeline(name string) (domain.Pipeline, error) {
	return d.store.CreatePipeline(d.context(), name, defaultDefinition())
}

// DeletePipeline permanently removes a user-selected pipeline and all of its
// local execution data. The UI always asks for confirmation first.
func (d *Desktop) DeletePipeline(id string) error {
	if err := d.store.DeletePipeline(d.context(), id); err != nil {
		return err
	}
	if err := d.scheduler.Reload(d.context()); err != nil {
		return err
	}
	return d.hotkeys.Reload(d.context())
}

// DuplicatePipeline creates an editable draft copy without duplicating live
// trigger bindings, permissions, or execution history.
func (d *Desktop) DuplicatePipeline(id string) (domain.Pipeline, error) {
	original, err := d.store.GetPipeline(d.context(), id)
	if err != nil {
		return domain.Pipeline{}, err
	}
	if !isCurrentBlueprintSchema(original.DraftDefinition.SchemaVersion) {
		return domain.Pipeline{}, fmt.Errorf("legacy pipelines must be rebuilt instead of duplicated")
	}
	copy, err := d.store.CreatePipeline(d.context(), original.Name+" copy", original.DraftDefinition)
	if err != nil {
		return domain.Pipeline{}, err
	}
	copy.Description = original.Description
	copy.Icon = original.Icon
	return d.store.SaveDraft(d.context(), copy)
}

func (d *Desktop) GetPipeline(id string) (domain.Pipeline, error) {
	return d.store.GetPipeline(d.context(), id)
}

func (d *Desktop) SavePipeline(pipeline domain.Pipeline) (domain.Pipeline, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return domain.Pipeline{}, err
	}
	return d.store.SaveDraft(d.context(), pipeline)
}

func (d *Desktop) PublishPipeline(pipeline domain.Pipeline) (domain.Pipeline, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return domain.Pipeline{}, err
	}
	if err := pipelineValidate(pipeline.DraftDefinition, d.registry); err != nil {
		return domain.Pipeline{}, err
	}
	bindings := bindingsFor(pipeline, d.registry)
	if err := d.hotkeys.Validate(d.context(), pipeline.ID, bindings); err != nil {
		return domain.Pipeline{}, err
	}
	published, err := d.store.Publish(d.context(), pipeline, bindings)
	if err != nil {
		return domain.Pipeline{}, err
	}
	if err := d.scheduler.Reload(d.context()); err != nil {
		return domain.Pipeline{}, err
	}
	if err := d.hotkeys.Reload(d.context()); err != nil {
		return domain.Pipeline{}, fmt.Errorf("pipeline was published, but global hotkeys could not be registered: %w", err)
	}
	d.twitch.Reconcile()
	return published, nil
}

func (d *Desktop) ListNodeDefinitions() ([]domain.NodeDefinition, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return nil, err
	}
	return d.registry.All(), nil
}

// ValidateJavaScript checks JavaScript source with the embedded Blueprint
// interpreter before the editor stores it in a draft. It never executes code
// or makes host services available.
func (d *Desktop) ValidateJavaScript(code string) error {
	return javascriptnode.Validate(code)
}

// ListDocumentation returns the local documentation tree. Markdown content is
// retrieved separately so navigation stays fast even with plugin bundles.
func (d *Desktop) ListDocumentation(language string) ([]domain.DocumentationEntry, error) {
	return d.documentation.List(language)
}

// GetDocumentation returns one rendered-safe Markdown source by stable ID.
func (d *Desktop) GetDocumentation(language, id string) (domain.DocumentationDocument, error) {
	return d.documentation.Get(language, id)
}

// SearchDocumentation performs local full-text search across embedded and
// enabled-plugin Markdown pages.
func (d *Desktop) SearchDocumentation(language, query string) ([]domain.DocumentationSearchResult, error) {
	return d.documentation.Search(language, query)
}

// GetDocumentationForNode resolves inspector links without exposing a plugin
// bundle path or changing the editor route.
func (d *Desktop) GetDocumentationForNode(nodeType string) (domain.DocumentationReference, error) {
	return d.documentation.ForNode(nodeType)
}

// ListFunctions returns workspace-wide custom functions for the Functions tab.
func (d *Desktop) ListFunctions() ([]domain.FunctionSummary, error) {
	return d.store.ListFunctions(d.context())
}

// CreateFunction creates a typed draft custom function selected in the
// function-creation dialog.
func (d *Desktop) CreateFunction(request domain.CreateFunctionRequest) (domain.CustomFunction, error) {
	return d.store.CreateFunctionWithRequest(d.context(), request)
}

func (d *Desktop) GetFunction(id string) (domain.CustomFunction, error) {
	return d.store.GetFunction(d.context(), id)
}

func (d *Desktop) SaveFunction(function domain.CustomFunction) (domain.CustomFunction, error) {
	return d.store.SaveFunctionDraft(d.context(), function)
}

func (d *Desktop) PublishFunction(function domain.CustomFunction) (domain.CustomFunction, error) {
	if err := validateFunction(function, d.registry); err != nil {
		return domain.CustomFunction{}, err
	}
	if err := d.validateFunctionDependencies(function); err != nil {
		return domain.CustomFunction{}, err
	}
	published, err := d.store.PublishFunction(d.context(), function)
	if err != nil {
		return domain.CustomFunction{}, err
	}
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return domain.CustomFunction{}, err
	}
	return published, nil
}

// validateFunctionDependencies rejects direct and indirect recursion before a
// new function revision is made callable from every pipeline.
func (d *Desktop) validateFunctionDependencies(root domain.CustomFunction) error {
	active := make(map[string]bool)
	var visit func(string, domain.FlowDefinition) error
	visit = func(functionID string, definition domain.FlowDefinition) error {
		if active[functionID] {
			return fmt.Errorf("recursive custom function call %q is not allowed", functionID)
		}
		active[functionID] = true
		defer delete(active, functionID)
		for _, node := range definition.Nodes {
			if !strings.HasPrefix(node.Type, "function:") || strings.HasPrefix(node.Type, "function:entry") || strings.HasPrefix(node.Type, "function:return") || strings.HasPrefix(node.Type, "function:input") || strings.HasPrefix(node.Type, "function:output") {
				continue
			}
			targetID := strings.TrimPrefix(node.Type, "function:")
			if targetID == root.ID {
				return fmt.Errorf("recursive custom function call %q is not allowed", root.Name)
			}
			target, err := d.store.GetPublishedFunction(d.context(), targetID)
			if err != nil {
				return fmt.Errorf("resolve function dependency %q: %w", targetID, err)
			}
			if err := visit(target.ID, target.DraftDefinition); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root.ID, root.DraftDefinition)
}

func (d *Desktop) DeleteFunction(id string) error {
	if err := d.store.DeleteFunction(d.context(), id); err != nil {
		return err
	}
	return d.refreshFunctionRegistry(d.context())
}

func (d *Desktop) ListTriggerButtons() ([]domain.TriggerBinding, error) {
	return d.store.ListTriggers(d.context(), domain.TriggerButton)
}

func (d *Desktop) ListSchedules() ([]domain.TriggerBinding, error) {
	return d.store.ListTriggers(d.context(), domain.TriggerCron)
}

// ListTwitchTriggers exposes credential-free EventSub binding state for the
// explicit trust/enable controls in Twitch settings.
func (d *Desktop) ListTwitchTriggers() ([]domain.TriggerBinding, error) {
	return d.store.ListTriggers(d.context(), domain.TriggerTwitch)
}

func (d *Desktop) SetTwitchTriggerEnabled(id string, enabled bool) error {
	binding, err := d.store.GetTrigger(d.context(), id)
	if err != nil {
		return err
	}
	if binding.Kind != domain.TriggerTwitch {
		return fmt.Errorf("trigger %q is not a Twitch trigger", binding.Label)
	}
	if enabled && !binding.Trusted {
		return fmt.Errorf("trust the published pipeline revision before enabling this Twitch trigger")
	}
	if err := d.store.SetTriggerEnabled(d.context(), id, enabled); err != nil {
		return err
	}
	d.twitch.Reconcile()
	return nil
}

// ListAllTriggers returns every published trigger binding for local API clients.
func (d *Desktop) ListAllTriggers() ([]domain.TriggerBinding, error) {
	return d.store.ListAllTriggers(d.context())
}

func (d *Desktop) SetScheduleEnabled(id string, enabled bool) error {
	binding, err := d.store.GetTrigger(d.context(), id)
	if err != nil {
		return err
	}
	if binding.Kind != domain.TriggerCron {
		return fmt.Errorf("only cron triggers can be changed from Schedules")
	}
	if enabled && !binding.Trusted {
		return fmt.Errorf("trust the active pipeline revision before enabling a schedule")
	}
	if err := d.store.SetTriggerEnabled(d.context(), id, enabled); err != nil {
		return err
	}
	return d.scheduler.Reload(d.context())
}

func (d *Desktop) RunTrigger(id string) (domain.Execution, error) {
	return d.runs.RunBinding(d.context(), id, pipeline.Packet{"trigger": "button"}, false)
}

// RunPipelineDraft performs an explicit, manual editor run using the current
// saved draft. Publishing remains required for trigger-board and scheduled runs.
func (d *Desktop) RunPipelineDraft(pipelineID, triggerNodeID string) (domain.Execution, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return domain.Execution{}, err
	}
	item, err := d.store.GetPipeline(d.context(), pipelineID)
	if err != nil {
		return domain.Execution{}, err
	}
	return d.runs.RunDraft(d.context(), item.ID, triggerNodeID, item.DraftDefinition, pipeline.Packet{"trigger": "manual"})
}

func (d *Desktop) ListExecutions(pipelineID string) ([]domain.Execution, error) {
	return d.store.ListExecutions(d.context(), pipelineID, 20)
}

// ListRecentExecutions returns workspace-wide execution history.
func (d *Desktop) ListRecentExecutions(limit int) ([]domain.Execution, error) {
	return d.store.ListRecentExecutions(d.context(), limit)
}

// GetMetricsOverview returns locally stored numerical observability data.
func (d *Desktop) GetMetricsOverview(filter domain.MetricsFilter) (domain.MetricsOverview, error) {
	return d.metrics.Overview(d.context(), filter)
}

// ClearMetrics removes local metrics without deleting automation history.
func (d *Desktop) ClearMetrics() error {
	return d.metrics.Clear(d.context())
}

// RecordHTTPMetric is used only by the embedded Fiber server to record a
// payload-free route category, outcome, and duration.
func (d *Desktop) RecordHTTPMetric(event domain.MetricActivityEvent) error {
	return d.metrics.RecordActivity(d.context(), event)
}

// GetExecution returns one redacted execution record and its node runs.
func (d *Desktop) GetExecution(id string) (domain.Execution, error) {
	return d.store.GetExecution(d.context(), id)
}

// StartPublishedPipeline queues a published Blueprint-v2 graph for a direct
// authenticated API invocation. It is an explicit run, not an unattended trigger.
func (d *Desktop) StartPublishedPipeline(pipelineID, triggerNodeID string, input map[string]any) (domain.Execution, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return domain.Execution{}, err
	}
	return d.runs.QueuePublished(d.context(), pipelineID, triggerNodeID, pipeline.Packet(input))
}

// ListReports returns the persisted local Markdown report feed.
func (d *Desktop) ListReports() ([]domain.Report, error) {
	return d.store.ListReports(d.context(), 100)
}

// DeleteReport permanently removes one report from the local workspace.
func (d *Desktop) DeleteReport(id string) error {
	return d.store.DeleteReport(d.context(), id)
}

// ListChatConversations returns all locally retained model and pipeline chats.
func (d *Desktop) ListChatConversations() ([]domain.ChatConversation, error) {
	return d.store.ListChatConversations(d.context())
}

// ListChatPipelines returns the published Chat Trigger bindings available to
// the Pipeline chat picker.
func (d *Desktop) ListChatPipelines() ([]domain.ChatPipeline, error) {
	return d.store.ListChatPipelines(d.context())
}

// CreateChatConversation starts either a model transcript or a selected
// pipeline transcript. Pipeline bindings are always resolved server-side.
func (d *Desktop) CreateChatConversation(mode domain.ChatMode, bindingID string) (domain.ChatConversation, error) {
	conversation := domain.ChatConversation{Mode: mode, ActionPolicy: domain.ChatActionAsk}
	if mode == domain.ChatModePipeline {
		binding, err := d.store.GetTrigger(d.context(), bindingID)
		if err != nil {
			return domain.ChatConversation{}, err
		}
		if binding.Kind != domain.TriggerChat || !binding.Enabled {
			return domain.ChatConversation{}, fmt.Errorf("selected trigger is unavailable for chat")
		}
		pipeline, err := d.store.GetPipeline(d.context(), binding.PipelineID)
		if err != nil {
			return domain.ChatConversation{}, err
		}
		conversation.PipelineID, conversation.TriggerBindingID = binding.PipelineID, binding.ID
		conversation.Title = defaultString(binding.Label, pipeline.Name)
	} else if mode != domain.ChatModeModel {
		return domain.ChatConversation{}, fmt.Errorf("invalid chat mode %q", mode)
	}
	return d.chat.CreateConversation(d.context(), conversation)
}

// SaveChatConversation persists a user-renamed title or action policy.
func (d *Desktop) SaveChatConversation(conversation domain.ChatConversation) (domain.ChatConversation, error) {
	return d.store.SaveChatConversation(d.context(), conversation)
}

// DeleteChatConversation permanently removes a local transcript.
func (d *Desktop) DeleteChatConversation(id string) error {
	return d.store.DeleteChatConversation(d.context(), id)
}

// ListChatMessages returns a chronological, local-only conversation transcript.
func (d *Desktop) ListChatMessages(conversationID string) ([]domain.ChatMessage, error) {
	return d.store.ListChatMessages(d.context(), conversationID, 200)
}

// ListChatRuns returns visible work items for a conversation.
func (d *Desktop) ListChatRuns(conversationID string) ([]domain.ChatRun, error) {
	return d.store.ListChatRuns(d.context(), conversationID)
}

// ListChatRunEvents returns compact tool and pipeline activity rows.
func (d *Desktop) ListChatRunEvents(chatRunID string) ([]domain.ChatRunEvent, error) {
	return d.store.ListChatRunEvents(d.context(), chatRunID)
}

// SendChatMessage queues the submitted text for the selected local chat mode.
func (d *Desktop) SendChatMessage(conversationID, text string) (domain.ChatRun, error) {
	return d.chat.Send(d.context(), conversationID, text)
}

// CancelChatRun stops one active local model or pipeline turn.
func (d *Desktop) CancelChatRun(id string) error {
	return d.chat.Cancel(d.context(), id)
}

// ListPendingChatApprovals returns model-tool confirmations awaiting the user.
func (d *Desktop) ListPendingChatApprovals(conversationID string) ([]domain.ChatApproval, error) {
	return d.store.ListPendingChatApprovals(d.context(), conversationID)
}

// ResolveChatApproval resumes a paused model turn after the styled UI dialog.
func (d *Desktop) ResolveChatApproval(id string, approved bool) error {
	return d.chat.ResolveApproval(d.context(), id, approved)
}

func (d *Desktop) GetRequiredCapabilities(pipelineID string) ([]domain.Capability, error) {
	if err := d.refreshFunctionRegistry(d.context()); err != nil {
		return nil, err
	}
	pipeline, err := d.store.GetPipeline(d.context(), pipelineID)
	if err != nil {
		return nil, err
	}
	return security.RequiredCapabilities(pipeline.DraftDefinition, d.registry), nil
}

func (d *Desktop) TrustPipelineRevision(pipelineID string, revision int) error {
	definition, err := d.store.PublishedDefinition(d.context(), pipelineID, revision)
	if err != nil {
		return err
	}
	for _, capability := range security.RequiredCapabilities(definition, d.registry) {
		if err := d.store.Grant(d.context(), domain.PermissionGrant{
			PipelineID: pipelineID,
			Revision:   revision,
			Capability: capability,
			Scope:      "*",
		}); err != nil {
			return err
		}
	}
	if err := d.store.TrustRevision(d.context(), pipelineID, revision); err != nil {
		return err
	}
	d.twitch.Reconcile()
	return nil
}

func (d *Desktop) GrantCapability(grant domain.PermissionGrant) error {
	return d.store.Grant(d.context(), grant)
}

// GetTwitchStatus exposes lifecycle state without exposing OAuth credentials.
func (d *Desktop) GetTwitchStatus() domain.TwitchStatus { return d.twitch.Status() }

// ListTwitchEventCatalog supplies the strict EventSub metadata used by the
// trigger editor and settings UI.
func (d *Desktop) ListTwitchEventCatalog() []domain.TwitchEventDescriptor { return d.twitch.Catalog() }

func (d *Desktop) StartTwitchDeviceAuthorization(request domain.TwitchDeviceAuthorizationRequest) (domain.TwitchDeviceAuthorization, error) {
	return d.twitch.BeginDeviceAuthorization(d.context(), request)
}

func (d *Desktop) CancelTwitchDeviceAuthorization(id string) { d.twitch.CancelDeviceAuthorization(id) }

func (d *Desktop) AddTwitchManualIdentity(request domain.TwitchManualIdentityRequest) (domain.TwitchIdentity, error) {
	return d.twitch.AddManualIdentity(d.context(), request)
}

func (d *Desktop) RemoveTwitchIdentity(id string) error {
	return d.twitch.RemoveIdentity(d.context(), id)
}

func (d *Desktop) TrustTwitchTrigger(id string) error {
	binding, err := d.store.GetTrigger(d.context(), id)
	if err != nil {
		return err
	}
	if binding.Kind != domain.TriggerTwitch {
		return fmt.Errorf("trigger %q is not a Twitch trigger", binding.Label)
	}
	return d.TrustPipelineRevision(binding.PipelineID, binding.Revision)
}

// ResolveNodeDefinition keeps editor pin rendering in lockstep with the
// module-owned dynamic port resolver.
func (d *Desktop) ResolveNodeDefinition(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition, found := d.registry.Get(node.Type)
	if !found {
		return domain.NodeDefinition{}, fmt.Errorf("node type %q is unavailable", node.Type)
	}
	if module, found := d.registry.Node(node.Type); found {
		return module.Resolve(node)
	}
	return definition, nil
}

func (d *Desktop) saveTwitchIdentity(ctx context.Context, identity domain.TwitchIdentity) error {
	d.settingsMu.Lock()
	defer d.settingsMu.Unlock()
	identities := d.settings.Twitch.Identities[:0]
	found := false
	for _, item := range d.settings.Twitch.Identities {
		if item.ID != identity.ID {
			identities = append(identities, item)
			continue
		}
		found = true
		if identity.Status != domain.TwitchIdentityRevoked {
			identities = append(identities, identity)
		}
	}
	if !found && identity.Status != domain.TwitchIdentityRevoked {
		identities = append(identities, identity)
	}
	d.settings.Twitch.Identities = identities
	if d.settings.Twitch.DefaultBotIdentityID == identity.ID && identity.Status == domain.TwitchIdentityRevoked {
		d.settings.Twitch.DefaultBotIdentityID = ""
	}
	if d.settings.Twitch.DefaultBotIdentityID == "" && identity.Status == domain.TwitchIdentityConnected {
		d.settings.Twitch.DefaultBotIdentityID = identity.ID
	}
	return d.store.SaveSettings(ctx, d.settings)
}

func (d *Desktop) GetSettings() domain.Settings {
	d.settingsMu.RLock()
	defer d.settingsMu.RUnlock()
	return d.settings
}

func (d *Desktop) SaveSettings(settings domain.Settings) error {
	d.settingsMu.Lock()
	defer d.settingsMu.Unlock()
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
	if err := normalizeMetricsSettings(&settings.Metrics); err != nil {
		return err
	}
	if settings.LlamaRuntime.Mode == "" {
		settings.LlamaRuntime.Mode = domain.RuntimeAuto
	}
	if settings.LlamaRuntime.ContextSize < 1024 {
		settings.LlamaRuntime.ContextSize = 8192
	}
	if err := normalizeConfiguredProvider(&settings); err != nil {
		return err
	}
	if err := validateAPISettings(&settings); err != nil {
		return err
	}
	if settings.API.Enabled && settings.API.AuthMode == domain.APIAuthToken {
		if _, err := d.vault.Get(settings.API.TokenRef); err != nil {
			return fmt.Errorf("resolve API token: %w", err)
		}
	}
	if strings.TrimSpace(settings.PluginDirectory) == "" {
		settings.PluginDirectory = filepath.Join(filepath.Dir(d.plugins.Directory()), "plugins")
	}
	contentDirectory, err := normalizeContentDirectory(settings.ContentDirectory, d.dataRoot)
	if err != nil {
		return err
	}
	settings.ContentDirectory = contentDirectory
	if err := d.store.SaveSettings(d.context(), settings); err != nil {
		return err
	}
	d.settings = settings
	if d.twitch != nil {
		d.twitch.Configure(settings.Twitch)
	}
	d.providers.Configure(settings)
	d.metrics.UpdateSettings(settings.Metrics)
	_ = d.metrics.Purge(d.context())
	d.runs.SetMaxConcurrentRuns(settings.MaxConcurrentRuns)
	d.plugins.SetDirectory(settings.PluginDirectory)
	d.configureContentDirectory(settings.ContentDirectory)
	_, _ = d.plugins.Reload()
	if d.api != nil {
		if err := d.api.Configure(d.context(), settings.API); err != nil {
			return err
		}
	}
	return nil
}

// ChooseContentDirectory opens the native directory picker. The selected path is
// only persisted when the renderer subsequently saves Settings.
func (d *Desktop) ChooseContentDirectory() (string, error) {
	current, err := normalizeContentDirectory(d.settings.ContentDirectory, d.dataRoot)
	if err != nil {
		return "", err
	}
	selected, err := wailsruntime.OpenDirectoryDialog(d.context(), wailsruntime.OpenDialogOptions{
		Title:            "Select Neuropipe content folder",
		DefaultDirectory: current,
	})
	if err != nil || strings.TrimSpace(selected) == "" {
		return selected, err
	}
	return normalizeContentDirectory(selected, d.dataRoot)
}

// GetLlamaRuntimeStatus returns safe state for the managed loopback runtime.
func (d *Desktop) GetLlamaRuntimeStatus() domain.LlamaRuntimeStatus { return d.llama.Status() }

// StartLlamaRuntime starts the executable configured in Settings and routes
// llama.cpp providers to the bound loopback endpoint for this app session.
func (d *Desktop) StartLlamaRuntime() (domain.LlamaRuntimeStatus, error) {
	ctx, cancel := context.WithTimeout(d.context(), 50*time.Second)
	defer cancel()
	runtimeSettings, err := d.effectiveLlamaSettings()
	if err != nil {
		return domain.LlamaRuntimeStatus{}, err
	}
	status, err := d.llama.Start(ctx, runtimeSettings)
	if err != nil {
		_ = d.metrics.RecordActivity(d.context(), domain.MetricActivityEvent{Kind: "runtime.start", Outcome: "failed", OccurredAt: time.Now().UTC()})
		return domain.LlamaRuntimeStatus{}, err
	}

	// Starting the managed runtime is an explicit choice to use it for pipeline
	// AI nodes. Keep the persisted provider configuration and the active manager
	// in lockstep so execution cannot fall back to the Ollama default endpoint.
	settings := d.settings
	settings.LlamaRuntime = runtimeSettings
	activateManagedLlamaProvider(&settings, filepath.Base(runtimeSettings.ModelPath), status.Endpoint)
	if err := d.SaveSettings(settings); err != nil {
		d.llama.Stop()
		_ = d.metrics.RecordActivity(d.context(), domain.MetricActivityEvent{Kind: "runtime.start", Outcome: "failed", OccurredAt: time.Now().UTC()})
		return domain.LlamaRuntimeStatus{}, fmt.Errorf("save managed llama.cpp provider selection: %w", err)
	}
	_ = d.metrics.RecordActivity(d.context(), domain.MetricActivityEvent{Kind: "runtime.start", Outcome: "ready", OccurredAt: time.Now().UTC()})
	return status, nil
}

// StopLlamaRuntime stops only the child process Neuropipe itself launched.
func (d *Desktop) StopLlamaRuntime() domain.LlamaRuntimeStatus {
	d.llama.Stop()
	_ = d.metrics.RecordActivity(d.context(), domain.MetricActivityEvent{Kind: "runtime.stop", OccurredAt: time.Now().UTC()})
	return d.llama.Status()
}

// ListLlamaRuntimeReleases lists compatible official llama.cpp Windows releases.
func (d *Desktop) ListLlamaRuntimeReleases() ([]domain.LlamaRuntimeRelease, error) {
	ctx, cancel := context.WithTimeout(d.context(), 20*time.Second)
	defer cancel()
	return d.llamaFiles.List(ctx)
}

// GetLlamaRuntimeCatalogStatus lists user-owned managed runtime installations.
func (d *Desktop) GetLlamaRuntimeCatalogStatus() (domain.LlamaRuntimeCatalogStatus, error) {
	return d.llamaFiles.Status(d.settings.LlamaRuntime.RuntimeVersion)
}

// GetInstallProgress returns the latest safe installer progress snapshot.
// The renderer polls this only while an install is active as a fallback for event delivery.
func (d *Desktop) GetInstallProgress(kind string) (domain.InstallProgress, error) {
	d.progressMu.RLock()
	defer d.progressMu.RUnlock()
	switch kind {
	case "runtime":
		return d.runtimeInstallProgress, nil
	case "model":
		return d.modelInstallProgress, nil
	default:
		return domain.InstallProgress{}, fmt.Errorf("unknown install progress kind %q", kind)
	}
}

// InstallLlamaRuntime downloads a verified official archive and selects it for this app.
func (d *Desktop) InstallLlamaRuntime(request domain.LlamaRuntimeInstallRequest) (domain.LlamaRuntimeCatalogStatus, error) {
	d.llama.Stop()
	d.reportInstallProgress(runtimeInstallProgressEvent, domain.InstallProgress{Kind: "runtime", Stage: "preparing", Label: "Preparing official runtime"})
	if err := d.llamaFiles.InstallWithProgress(d.context(), request, func(progress domain.InstallProgress) {
		d.reportInstallProgress(runtimeInstallProgressEvent, progress)
	}); err != nil {
		d.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "failed", "Runtime installation failed")
		return domain.LlamaRuntimeCatalogStatus{}, err
	}
	settings := d.settings
	settings.LlamaRuntime.RuntimeVersion = request.Version
	settings.LlamaRuntime.Mode = request.Mode
	settings.LlamaRuntime.BinaryPath = d.llamaFiles.RuntimeBinary(request.Version, request.Mode)
	d.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "saving", "Saving runtime selection")
	if err := d.SaveSettings(settings); err != nil {
		d.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "failed", "Runtime installation failed")
		return domain.LlamaRuntimeCatalogStatus{}, err
	}
	status, err := d.llamaFiles.Status(request.Version)
	if err != nil {
		d.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "failed", "Runtime installation failed")
		return domain.LlamaRuntimeCatalogStatus{}, err
	}
	d.advanceInstallProgress(runtimeInstallProgressEvent, "runtime", "complete", "Runtime ready")
	return status, nil
}

// SearchLlamaModels searches public, non-gated GGUF repositories.
func (d *Desktop) SearchLlamaModels(request domain.ModelSearchRequest) ([]domain.ModelSearchResult, error) {
	ctx, cancel := context.WithTimeout(d.context(), 30*time.Second)
	defer cancel()
	return d.models.Search(ctx, request)
}

// GetLlamaModelDetail returns a public model card and verifiable GGUF options.
func (d *Desktop) GetLlamaModelDetail(repository string) (domain.ModelDetail, error) {
	ctx, cancel := context.WithTimeout(d.context(), 30*time.Second)
	defer cancel()
	return d.models.Detail(ctx, repository)
}

// ListLlamaModelFiles remains available to older renderer builds.
func (d *Desktop) ListLlamaModelFiles(repository string) ([]domain.ModelFile, error) {
	detail, err := d.GetLlamaModelDetail(repository)
	if err != nil {
		return nil, err
	}
	return detail.Files, nil
}

// ListInstalledLlamaModels returns completed GGUF files from the active content
// folder for selection by the managed llama.cpp runtime.
func (d *Desktop) ListInstalledLlamaModels() ([]domain.LocalModel, error) {
	ctx, cancel := context.WithTimeout(d.context(), 10*time.Second)
	defer cancel()
	return d.models.Installed(ctx)
}

// InstallLlamaModel downloads a verified GGUF model and selects it for llama.cpp.
func (d *Desktop) InstallLlamaModel(request domain.ModelInstallRequest) (domain.LocalModel, error) {
	d.llama.Stop()
	d.reportInstallProgress(modelInstallProgressEvent, domain.InstallProgress{Kind: "model", Stage: "preparing", Label: "Preparing GGUF model"})
	model, err := d.models.InstallWithProgress(d.context(), request, func(progress domain.InstallProgress) {
		d.reportInstallProgress(modelInstallProgressEvent, progress)
	})
	if err != nil {
		d.advanceInstallProgress(modelInstallProgressEvent, "model", "failed", "Model installation failed")
		return domain.LocalModel{}, err
	}
	settings := d.settings
	settings.LlamaRuntime.ModelPath = model.Path
	activateManagedLlamaProvider(&settings, filepath.Base(model.Path), "")
	d.advanceInstallProgress(modelInstallProgressEvent, "model", "saving", "Saving model selection")
	if err := d.SaveSettings(settings); err != nil {
		d.advanceInstallProgress(modelInstallProgressEvent, "model", "failed", "Model installation failed")
		return domain.LocalModel{}, err
	}
	d.advanceInstallProgress(modelInstallProgressEvent, "model", "complete", "Model ready")
	return model, nil
}

// SelectInstalledLlamaModel makes an already installed model active without
// exposing arbitrary filesystem access to API callers.
func (d *Desktop) SelectInstalledLlamaModel(path string) error {
	path = strings.TrimSpace(path)
	models, err := d.ListInstalledLlamaModels()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(models, func(model domain.LocalModel) bool { return strings.EqualFold(model.Path, path) }) {
		return fmt.Errorf("selected model is not installed in Neuropipe's content folder")
	}
	d.llama.Stop()
	settings := d.GetSettings()
	settings.LlamaRuntime.ModelPath = path
	activateManagedLlamaProvider(&settings, filepath.Base(path), "")
	return d.SaveSettings(settings)
}

// DeleteInstalledLlamaModel removes one verified local GGUF after the custom
// renderer confirmation. The active runtime is stopped before its model file
// can be removed.
func (d *Desktop) DeleteInstalledLlamaModel(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("an installed model path is required")
	}
	if strings.EqualFold(d.GetSettings().LlamaRuntime.ModelPath, path) {
		d.llama.Stop()
		settings := d.GetSettings()
		settings.LlamaRuntime.ModelPath = ""
		if err := d.SaveSettings(settings); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(d.context(), 10*time.Second)
	defer cancel()
	return d.models.RemoveInstalled(ctx, path)
}

// GetAPIStatus reports whether the optional Fiber listener is active.
func (d *Desktop) GetAPIStatus() domain.APIStatus {
	if d.api == nil {
		return domain.APIStatus{Message: "API is unavailable"}
	}
	return d.api.Status()
}

// RotateAPIToken creates a new DPAPI-protected bearer token and returns it once
// so the renderer can show it in an in-app confirmation dialog.
func (d *Desktop) RotateAPIToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate API token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	settings := d.GetSettings()
	if strings.TrimSpace(settings.API.TokenRef) == "" {
		settings.API.TokenRef = "neuropipe-api-token"
	}
	if err := d.vault.Put(settings.API.TokenRef, token); err != nil {
		return "", err
	}
	if err := d.SaveSettings(settings); err != nil {
		return "", err
	}
	return token, nil
}

// HandleWebhook verifies a trigger-specific HMAC signature before queueing a
// trusted published webhook binding for unattended execution.
func (d *Desktop) HandleWebhook(path string, body []byte, signature string) (domain.Execution, error) {
	path = normalizeWebhookPath(path)
	if path == "" {
		return domain.Execution{}, fmt.Errorf("webhook path is required")
	}
	bindings, err := d.store.ListTriggers(d.context(), domain.TriggerHook)
	if err != nil {
		return domain.Execution{}, err
	}
	for _, binding := range bindings {
		definition, err := d.store.PublishedDefinition(d.context(), binding.PipelineID, binding.Revision)
		if err != nil {
			continue
		}
		for _, node := range definition.Nodes {
			if node.ID != binding.NodeID || node.Type != "trigger:webhook" {
				continue
			}
			config := node.Data
			if nested, ok := config["config"].(map[string]any); ok {
				config = nested
			}
			configuredPath, _ := config["path"].(string)
			if normalizeWebhookPath(configuredPath) != path {
				continue
			}
			secretRef, _ := config["secret"].(string)
			if strings.TrimSpace(secretRef) == "" {
				return domain.Execution{}, fmt.Errorf("webhook %q has no signing secret", path)
			}
			secret, err := d.vault.Get(secretRef)
			if err != nil {
				return domain.Execution{}, fmt.Errorf("resolve webhook signing secret: %w", err)
			}
			if !validWebhookSignature(body, signature, secret) {
				return domain.Execution{}, fmt.Errorf("webhook signature is invalid")
			}
			input := pipeline.Packet{"trigger": "webhook", "body": string(body)}
			var jsonBody any
			if json.Unmarshal(body, &jsonBody) == nil {
				input["json"] = jsonBody
			}
			return d.runs.QueueBinding(d.context(), binding.ID, input, true)
		}
	}
	return domain.Execution{}, fmt.Errorf("webhook path %q was not found", path)
}

func (d *Desktop) ListProviderModels(providerID string) ([]llm.ModelInfo, error) {
	return d.providers.ListModels(d.context(), providerID)
}

func (d *Desktop) ListSecrets() ([]security.SecretMetadata, error) { return d.vault.List() }

func (d *Desktop) SaveSecret(name, value string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
		return fmt.Errorf("a secret name and value are required")
	}
	return d.vault.Put(strings.TrimSpace(name), value)
}

func (d *Desktop) DeleteSecret(name string) error { return d.vault.Delete(name) }

func (d *Desktop) ListPlugins() []domain.PluginStatus { return d.plugins.Status() }

func (d *Desktop) RediscoverPlugins() ([]domain.PluginStatus, error) { return d.plugins.Reload() }

func (d *Desktop) context() context.Context {
	if d.ctx != nil {
		return d.ctx
	}
	return context.Background()
}

func (d *Desktop) emit(event string, payload any) {
	if d.ctx != nil {
		wailsruntime.EventsEmit(d.ctx, event, payload)
	}
}

func (d *Desktop) reportInstallProgress(event string, progress domain.InstallProgress) {
	d.progressMu.Lock()
	if progress.Kind == "runtime" {
		d.runtimeInstallProgress = progress
	} else {
		d.modelInstallProgress = progress
	}
	d.progressMu.Unlock()
	d.emit(event, progress)
}

func (d *Desktop) advanceInstallProgress(event, kind, stage, label string) {
	d.progressMu.RLock()
	progress := d.modelInstallProgress
	if kind == "runtime" {
		progress = d.runtimeInstallProgress
	}
	d.progressMu.RUnlock()
	progress.Kind = kind
	progress.Stage = stage
	progress.Label = label
	if progress.TotalBytes > 0 && (stage == "saving" || stage == "complete") {
		progress.DownloadedBytes = progress.TotalBytes
		progress.Percentage = 100
	}
	d.reportInstallProgress(event, progress)
}

// activateManagedLlamaProvider makes Neuropipe's owned llama.cpp server the
// one configured LLM provider for future pipeline node executions.
func activateManagedLlamaProvider(settings *domain.Settings, model, endpoint string) {
	settings.Providers = []domain.ProviderConfig{{
		ID:      managedLlamaProviderID,
		Name:    "Managed llama.cpp",
		Kind:    domain.ProviderLlamaCPP,
		BaseURL: endpoint,
		Model:   model,
		Enabled: true,
	}}
	settings.DefaultProviderID = managedLlamaProviderID
}

// normalizeConfiguredProvider keeps the persisted provider contract simple:
// Neuropipe has one active LLM provider at a time. The slice remains in the
// domain type for a stable SDK boundary, but contains exactly one item.
func normalizeConfiguredProvider(settings *domain.Settings) error {
	provider := domain.ProviderConfig{
		ID:      "ollama-local",
		Name:    "Local Ollama",
		Kind:    domain.ProviderOllama,
		BaseURL: "http://127.0.0.1:11434",
		Enabled: true,
	}
	for _, item := range settings.Providers {
		if item.ID == settings.DefaultProviderID {
			provider = item
			break
		}
	}
	if len(settings.Providers) > 0 && provider.ID == "ollama-local" && settings.DefaultProviderID != "ollama-local" {
		provider = settings.Providers[0]
	}

	switch provider.Kind {
	case "", domain.ProviderOllama:
		provider.ID = "ollama-local"
		provider.Name = "Local Ollama"
		provider.Kind = domain.ProviderOllama
		provider.APIKeyRef = ""
		if strings.TrimSpace(provider.BaseURL) == "" {
			provider.BaseURL = "http://127.0.0.1:11434"
		}
	case domain.ProviderLlamaCPP:
		provider.ID = managedLlamaProviderID
		provider.Name = "Managed llama.cpp"
		provider.APIKeyRef = ""
	case domain.ProviderOpenAICompatible:
		provider.ID = "openai-compatible"
		provider.Name = "OpenAI-compatible"
		if strings.TrimSpace(provider.BaseURL) == "" {
			return fmt.Errorf("an OpenAI-compatible provider requires a base URL")
		}
	default:
		return fmt.Errorf("unsupported LLM provider kind %q", provider.Kind)
	}
	provider.Enabled = true
	settings.Providers = []domain.ProviderConfig{provider}
	settings.DefaultProviderID = provider.ID
	return nil
}

func (d *Desktop) effectiveLlamaSettings() (domain.LlamaRuntimeSettings, error) {
	settings := d.settings.LlamaRuntime
	if strings.TrimSpace(settings.RuntimeVersion) == "" {
		return settings, nil
	}
	mode, binary, err := d.llamaFiles.ResolveBinary(settings.RuntimeVersion, settings.Mode)
	if err != nil {
		return domain.LlamaRuntimeSettings{}, err
	}
	settings.Mode, settings.BinaryPath = mode, binary
	return settings, nil
}

// configureContentDirectory keeps downloaded runtimes and GGUF files together
// while leaving databases, secrets, and execution records in app data.
func (d *Desktop) configureContentDirectory(contentDirectory string) {
	d.llamaFiles = localruntime.NewLlamaCatalog(filepath.Join(contentDirectory, "runtimes", "llama.cpp"))
	d.models = localruntime.NewModelCatalog(filepath.Join(contentDirectory, "models"))
}

func normalizeContentDirectory(value, dataRoot string) (string, error) {
	contentDirectory := strings.TrimSpace(os.ExpandEnv(value))
	if contentDirectory == "" {
		contentDirectory = filepath.Join(dataRoot, "content")
	}
	if !filepath.IsAbs(contentDirectory) {
		return "", fmt.Errorf("content folder must be an absolute path")
	}
	contentDirectory = filepath.Clean(contentDirectory)
	if err := os.MkdirAll(contentDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create content folder: %w", err)
	}
	info, err := os.Stat(contentDirectory)
	if err != nil {
		return "", fmt.Errorf("inspect content folder: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("content folder must be a directory")
	}
	return contentDirectory, nil
}

func normalizeMetricsSettings(settings *domain.MetricsSettings) error {
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
	seen := make(map[string]struct{}, len(settings.PriceRates))
	valid := make([]domain.ModelPriceRate, 0, len(settings.PriceRates))
	for _, rate := range settings.PriceRates {
		rate.ProviderID, rate.Model = strings.TrimSpace(rate.ProviderID), strings.TrimSpace(rate.Model)
		if rate.ProviderID == "" || rate.Model == "" {
			return fmt.Errorf("price rates need a provider and model")
		}
		if rate.InputUSDPerMillion < 0 || rate.OutputUSDPerMillion < 0 {
			return fmt.Errorf("price rates cannot be negative")
		}
		key := rate.ProviderID + "\x00" + strings.ToLower(rate.Model)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("price rate for %s is duplicated", rate.Model)
		}
		seen[key] = struct{}{}
		valid = append(valid, rate)
	}
	settings.PriceRates = valid
	return nil
}

func validateAPISettings(settings *domain.Settings) error {
	api := &settings.API
	if api.BindAddress == "" {
		api.BindAddress = "127.0.0.1"
	}
	if net.ParseIP(api.BindAddress) == nil {
		return fmt.Errorf("API bind address must be an IP address")
	}
	if api.Port == 0 {
		api.Port = settings.WebhookPort
	}
	if api.Port < 1024 || api.Port > 65535 {
		return fmt.Errorf("choose an API port between 1024 and 65535")
	}
	// Keep the old field in sync for pre-API renderer compatibility.
	settings.WebhookPort = api.Port
	if api.AuthMode == "" {
		api.AuthMode = domain.APIAuthToken
	}
	if api.AuthMode != domain.APIAuthToken && api.AuthMode != domain.APIAuthNone {
		return fmt.Errorf("unsupported API authentication mode %q", api.AuthMode)
	}
	loopback := net.ParseIP(api.BindAddress).IsLoopback()
	if api.Enabled && !loopback && !api.ExposureAcknowledged {
		return fmt.Errorf("confirm non-loopback API exposure before enabling the listener")
	}
	if api.AuthMode == domain.APIAuthNone {
		api.AdminEnabled = false
	}
	if api.Enabled && api.AuthMode == domain.APIAuthToken && strings.TrimSpace(api.TokenRef) == "" {
		return fmt.Errorf("create an API token before enabling token authentication")
	}
	origins := make([]string, 0, len(api.AllowedOrigins))
	seen := make(map[string]struct{}, len(api.AllowedOrigins))
	for _, value := range api.AllowedOrigins {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" {
			return fmt.Errorf("invalid CORS origin %q", origin)
		}
		if _, exists := seen[origin]; !exists {
			seen[origin] = struct{}{}
			origins = append(origins, origin)
		}
	}
	api.AllowedOrigins = origins
	return nil
}

func normalizeWebhookPath(value string) string {
	path := "/" + strings.Trim(strings.TrimSpace(value), "/")
	if path == "/" {
		return ""
	}
	return path
}

func validWebhookSignature(body []byte, header, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	presented, err := hex.DecodeString(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(presented, expected)
}

func appDataRoot() (string, error) {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "Neuropipe"), nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find application data directory: %w", err)
	}
	return filepath.Join(root, "Neuropipe"), nil
}

func defaultDefinition() domain.FlowDefinition {
	return domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		{ID: "button-trigger", Type: "trigger:button", Position: domain.Position{X: 100, Y: 180}, Data: map[string]any{"config": map[string]any{"label": "Run pipeline", "icon": "play", "color": "#fafafa", "gridPosition": 0}}},
		{ID: "notification", Type: "action:notification", Position: domain.Position{X: 420, Y: 180}, Data: map[string]any{"config": map[string]any{"title": "Neuropipe", "message": "Blueprint ready"}}},
	}, Edges: []domain.FlowEdge{{ID: "button-to-notification", Source: "button-trigger", SourceHandle: "out", Target: "notification", TargetHandle: "in", Kind: domain.PinExec}}, Viewport: domain.Viewport{X: 0, Y: 0, Zoom: 1}}
}

func (d *Desktop) refreshFunctionRegistry(ctx context.Context) error {
	definitions, err := d.store.PublishedFunctionDefinitions(ctx)
	if err != nil {
		return err
	}
	for index := range definitions {
		functionID := strings.TrimPrefix(definitions[index].Type, "function:")
		capabilities, err := d.functionCapabilities(ctx, functionID, make(map[string]bool))
		if err != nil {
			return err
		}
		definitions[index].Capabilities = capabilities
	}
	d.registry.ReplaceDynamic(definitions)
	return nil
}

func (d *Desktop) functionCapabilities(ctx context.Context, functionID string, active map[string]bool) ([]domain.Capability, error) {
	if active[functionID] {
		return nil, fmt.Errorf("recursive custom function %q cannot be resolved", functionID)
	}
	active[functionID] = true
	defer delete(active, functionID)
	function, err := d.store.GetPublishedFunction(ctx, functionID)
	if err != nil {
		return nil, err
	}
	set := make(map[domain.Capability]struct{})
	for _, node := range function.DraftDefinition.Nodes {
		if strings.HasPrefix(node.Type, "function:") && !isFunctionBoundary(node.Type) {
			capabilities, err := d.functionCapabilities(ctx, strings.TrimPrefix(node.Type, "function:"), active)
			if err != nil {
				return nil, err
			}
			for _, capability := range capabilities {
				set[capability] = struct{}{}
			}
			continue
		}
		if definition, exists := d.registry.Get(node.Type); exists {
			for _, capability := range definition.Capabilities {
				set[capability] = struct{}{}
			}
		}
	}
	result := make([]domain.Capability, 0, len(set))
	for capability := range set {
		result = append(result, capability)
	}
	slices.Sort(result)
	return result, nil
}

func isFunctionBoundary(nodeType string) bool {
	return nodeType == "function:entry" || nodeType == "function:return" || nodeType == "function:input" || nodeType == "function:output"
}

func validateFunction(function domain.CustomFunction, registry *catalog.Registry) error {
	if strings.TrimSpace(function.Name) == "" {
		return fmt.Errorf("function name is required")
	}
	if !isCurrentBlueprintSchema(function.DraftDefinition.SchemaVersion) {
		return fmt.Errorf("function must use Blueprint v3")
	}
	if function.Kind == "" {
		function.Kind = domain.FunctionStandard
	}
	if function.Kind != domain.FunctionStandard && function.Kind != domain.FunctionTool {
		return fmt.Errorf("function has an invalid kind")
	}
	if function.Kind == domain.FunctionTool && function.Mode != domain.NodeImpure {
		return fmt.Errorf("an LLM tool function must be impure")
	}
	if function.Kind == domain.FunctionTool {
		if err := pipeline.ValidateToolFunction(function); err != nil {
			return err
		}
	}
	if err := validateFunctionPins("input", function.Inputs); err != nil {
		return err
	}
	if err := validateFunctionPins("output", function.Outputs); err != nil {
		return err
	}
	entries, returns, inputs, outputs := 0, 0, 0, 0
	entryID, returnID := "", ""
	for _, node := range function.DraftDefinition.Nodes {
		switch node.Type {
		case "function:entry":
			entries++
			entryID = node.ID
		case "function:return":
			returns++
			returnID = node.ID
		case "function:input":
			inputs++
		case "function:output":
			outputs++
		}
		definition, exists := registry.Get(node.Type)
		if !exists {
			return fmt.Errorf("function uses unavailable node %q", node.Type)
		}
		if function.Mode == domain.NodePure && (definition.Mode == domain.NodeImpure || definition.Mode == domain.NodeEvent) {
			return fmt.Errorf("pure function cannot contain %s", definition.Label)
		}
		if function.Mode == domain.NodeImpure && definition.Mode == domain.NodeEvent && node.Type != "function:entry" {
			return fmt.Errorf("impure function cannot contain event node %s", definition.Label)
		}
	}
	if function.Mode == domain.NodeImpure {
		if entries != 1 || returns != 1 || inputs != 0 || outputs != 0 {
			return fmt.Errorf("an impure function needs exactly one Function Entry and Function Return")
		}
		if !functionReturnReachable(function.DraftDefinition, entryID, returnID) {
			return fmt.Errorf("function return must be reachable from Function Entry")
		}
	} else if entries != 0 || returns != 0 || inputs != 1 || outputs != 1 {
		return fmt.Errorf("a pure function needs exactly one Function Inputs and Function Outputs node")
	}
	return nil
}

func isCurrentBlueprintSchema(version int) bool {
	return version == domain.GraphSchemaV2 || version == domain.GraphSchemaV3
}

func validateFunctionPins(side string, pins []domain.FunctionPin) error {
	seen := make(map[string]struct{}, len(pins))
	for _, pin := range pins {
		if strings.TrimSpace(pin.ID) == "" || strings.TrimSpace(pin.Name) == "" {
			return fmt.Errorf("function %s pins need an ID and name", side)
		}
		if _, exists := seen[pin.ID]; exists {
			return fmt.Errorf("function %s pin ID %q is duplicated", side, pin.ID)
		}
		seen[pin.ID] = struct{}{}
		switch pin.DataType {
		case domain.DataAny, domain.DataText, domain.DataNumber, domain.DataBoolean, domain.DataObject, domain.DataList:
		default:
			return fmt.Errorf("function %s pin %q has an invalid data type", side, pin.Name)
		}
	}
	return nil
}

func functionReturnReachable(definition domain.FlowDefinition, entryID, returnID string) bool {
	queue := []string{entryID}
	seen := map[string]bool{entryID: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == returnID {
			return true
		}
		for _, edge := range definition.Edges {
			if edge.Source != current || (edge.Kind != "" && edge.Kind != domain.PinExec) || seen[edge.Target] {
				continue
			}
			seen[edge.Target] = true
			queue = append(queue, edge.Target)
		}
	}
	return false
}

func pipelineValidate(definition domain.FlowDefinition, registry *catalog.Registry) error {
	return pipeline.Validate(definition, registry)
}

func bindingsFor(pipeline domain.Pipeline, registry *catalog.Registry) []domain.TriggerBinding {
	bindings := make([]domain.TriggerBinding, 0)
	for _, node := range pipeline.DraftDefinition.Nodes {
		definition, ok := registry.Get(node.Type)
		if !ok || definition.TriggerKind == "" {
			continue
		}
		config := node.Data
		if nested, ok := node.Data["config"].(map[string]any); ok {
			config = nested
		}
		label, _ := config["label"].(string)
		if strings.TrimSpace(label) == "" {
			label = pipeline.Name
		}
		icon, _ := config["icon"].(string)
		color, _ := config["color"].(string)
		cron, _ := config["cron"].(string)
		timezone, _ := config["timezone"].(string)
		hotkey, _ := config["hotkey"].(string)
		gridPosition := 0
		switch value := config["gridPosition"].(type) {
		case float64:
			gridPosition = int(value)
		case int:
			gridPosition = value
		}
		bindings = append(bindings, domain.TriggerBinding{NodeID: node.ID, NodeType: node.Type, Config: cloneConfig(config), Kind: definition.TriggerKind, Label: label, Icon: defaultString(icon, "play"), Color: defaultString(color, "#fafafa"), GridPosition: gridPosition, Hotkey: hotkey, Cron: cron, Timezone: defaultString(timezone, "Local")})
	}
	return bindings
}

func cloneConfig(config map[string]any) map[string]any {
	encoded, err := json.Marshal(config)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return map[string]any{}
	}
	return cloned
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
