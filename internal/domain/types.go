// Package domain owns Neuropipe's serialisable business contracts.
package domain

import (
	"strings"
	"time"
)

type PipelineStatus string

const (
	PipelineDraft    PipelineStatus = "draft"
	PipelineActive   PipelineStatus = "active"
	PipelineArchived PipelineStatus = "archived"
	PipelineLegacy   PipelineStatus = "legacy"
)

// GraphSchemaV2 is the Blueprint-style graph format. V1 definitions are kept
// as read-only migration references and are never executed by the desktop app.
const GraphSchemaV2 = 2

// PinKind separates control flow from values. Only exec pins can execute a
// node; data pins are resolved by the active execution frame on demand.
type PinKind string

const (
	PinExec PinKind = "exec"
	PinData PinKind = "data"
)

// PinDirection identifies which side of a node owns a pin.
type PinDirection string

const (
	PinInput  PinDirection = "input"
	PinOutput PinDirection = "output"
)

// DataType is intentionally small and maps directly to JSON values.
type DataType string

const (
	DataAny     DataType = "any"
	DataText    DataType = "text"
	DataNumber  DataType = "number"
	DataBoolean DataType = "boolean"
	DataObject  DataType = "object"
	DataList    DataType = "list"
)

// NodeExecutionMode determines how a node participates in a Blueprint graph.
type NodeExecutionMode string

const (
	NodeEvent  NodeExecutionMode = "event"
	NodeImpure NodeExecutionMode = "impure"
	NodePure   NodeExecutionMode = "pure"
	NodeVisual NodeExecutionMode = "visual"
)

type TriggerKind string

const (
	TriggerButton TriggerKind = "button"
	TriggerCron   TriggerKind = "cron"
	TriggerFile   TriggerKind = "file"
	TriggerHotkey TriggerKind = "hotkey"
	TriggerHook   TriggerKind = "webhook"
	// TriggerChat starts a published pipeline from a local conversation.
	TriggerChat TriggerKind = "chat"
)

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
	RunSkipped   RunStatus = "skipped"
	RunCancelled RunStatus = "cancelled"
)

type ProviderKind string

const (
	ProviderOllama           ProviderKind = "ollama"
	ProviderLlamaCPP         ProviderKind = "llamacpp"
	ProviderOpenAICompatible ProviderKind = "openai-compatible"
)

type Capability string

const (
	CapabilityNetwork   Capability = "network"
	CapabilityFileRead  Capability = "file-read"
	CapabilityFileWrite Capability = "file-write"
	CapabilityTerminal  Capability = "terminal"
	CapabilityGit       Capability = "git"
	CapabilityDocker    Capability = "docker"
	CapabilityPlugin    Capability = "plugin"
)

// Pipeline is the editor-facing aggregate. Definition remains JSON so React
// Flow can evolve without database migrations for every node setting.
type Pipeline struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Icon              string         `json:"icon"`
	IconColor         string         `json:"iconColor"`
	IconBackground    string         `json:"iconBackground"`
	Status            PipelineStatus `json:"status"`
	DraftDefinition   FlowDefinition `json:"draftDefinition"`
	PublishedRevision int            `json:"publishedRevision"`
	MigrationIssue    string         `json:"migrationIssue,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type PipelineSummary struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Icon              string         `json:"icon"`
	IconColor         string         `json:"iconColor"`
	IconBackground    string         `json:"iconBackground"`
	Status            PipelineStatus `json:"status"`
	PublishedRevision int            `json:"publishedRevision"`
	TriggerCount      int            `json:"triggerCount"`
	MigrationIssue    string         `json:"migrationIssue,omitempty"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type FlowDefinition struct {
	SchemaVersion int        `json:"schemaVersion"`
	Nodes         []FlowNode `json:"nodes"`
	Edges         []FlowEdge `json:"edges"`
	Viewport      Viewport   `json:"viewport"`
}

type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type FlowNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position Position       `json:"position"`
	Data     map[string]any `json:"data"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type FlowEdge struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	SourceHandle string  `json:"sourceHandle,omitempty"`
	TargetHandle string  `json:"targetHandle,omitempty"`
	Kind         PinKind `json:"kind,omitempty"`
}

type TriggerBinding struct {
	ID            string      `json:"id"`
	PipelineID    string      `json:"pipelineId"`
	NodeID        string      `json:"nodeId"`
	Revision      int         `json:"revision"`
	Kind          TriggerKind `json:"kind"`
	Label         string      `json:"label"`
	Icon          string      `json:"icon"`
	Color         string      `json:"color"`
	GridPosition  int         `json:"gridPosition"`
	Hotkey        string      `json:"hotkey,omitempty"`
	Cron          string      `json:"cron,omitempty"`
	Timezone      string      `json:"timezone,omitempty"`
	Enabled       bool        `json:"enabled"`
	Trusted       bool        `json:"trusted"`
	NextRunAt     *time.Time  `json:"nextRunAt,omitempty"`
	LastRunAt     *time.Time  `json:"lastRunAt,omitempty"`
	LastRunStatus RunStatus   `json:"lastRunStatus,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

type Execution struct {
	ID           string     `json:"id"`
	PipelineID   string     `json:"pipelineId"`
	TriggerID    string     `json:"triggerId,omitempty"`
	Status       RunStatus  `json:"status"`
	StartedAt    time.Time  `json:"startedAt"`
	QueuedAt     *time.Time `json:"queuedAt,omitempty"`
	RunStartedAt *time.Time `json:"runStartedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	Error        string     `json:"error,omitempty"`
	NodeRuns     []NodeRun  `json:"nodeRuns,omitempty"`
}

// Report is a Markdown document emitted by a pipeline execution and kept in
// the local SQLite workspace for later review.
type Report struct {
	ID                 string    `json:"id"`
	PipelineID         string    `json:"pipelineId"`
	PipelineName       string    `json:"pipelineName"`
	ExecutionID        string    `json:"executionId"`
	NodeID             string    `json:"nodeId"`
	Title              string    `json:"title"`
	Tags               []string  `json:"tags"`
	Markdown           string    `json:"markdown"`
	CreatedAt          time.Time `json:"createdAt"`
	ExecutionStartedAt time.Time `json:"executionStartedAt"`
}

// ChatMode identifies whether a conversation is handled directly by the
// configured model or by a published pipeline chat trigger.
type ChatMode string

const (
	ChatModeModel    ChatMode = "model"
	ChatModePipeline ChatMode = "pipeline"
)

// ChatActionPolicy controls confirmation of assistant tool calls for one
// conversation. It is intentionally conversation-scoped, never global.
type ChatActionPolicy string

const (
	ChatActionAsk    ChatActionPolicy = "ask"
	ChatActionAlways ChatActionPolicy = "always"
)

// ChatMessageRole is kept provider-neutral so one transcript can be supplied
// to Ollama and OpenAI-compatible adapters.
type ChatMessageRole string

const (
	ChatRoleUser      ChatMessageRole = "user"
	ChatRoleAssistant ChatMessageRole = "assistant"
	ChatRoleTool      ChatMessageRole = "tool"
	ChatRoleSystem    ChatMessageRole = "system"
)

// ChatConversation is a local conversation. Pipeline conversations pin a
// trigger binding; model conversations use the active Settings provider.
type ChatConversation struct {
	ID               string           `json:"id"`
	Mode             ChatMode         `json:"mode"`
	Title            string           `json:"title"`
	PipelineID       string           `json:"pipelineId,omitempty"`
	TriggerBindingID string           `json:"triggerBindingId,omitempty"`
	ActionPolicy     ChatActionPolicy `json:"actionPolicy"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// ChatToolCall is the structured form returned by a tool-capable provider.
// Arguments are JSON-safe and never contain resolved Neuropipe secrets.
type ChatToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ChatMessage is a durable transcript item. ToolCalls are retained so an
// OpenAI-compatible provider can resume a paused approval turn correctly.
type ChatMessage struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversationId"`
	ChatRunID      string          `json:"chatRunId,omitempty"`
	Role           ChatMessageRole `json:"role"`
	Content        string          `json:"content"`
	ToolCallID     string          `json:"toolCallId,omitempty"`
	ToolName       string          `json:"toolName,omitempty"`
	ToolCalls      []ChatToolCall  `json:"toolCalls,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// ChatRun tracks a single submitted turn. ExecutionID is only set for a
// Pipeline-mode run and remains separate from the chat-visible run ID.
type ChatRun struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	ExecutionID    string    `json:"executionId,omitempty"`
	Status         RunStatus `json:"status"`
	StatusText     string    `json:"statusText"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ChatRunEvent is a compact, expandable activity row in the chat transcript.
type ChatRunEvent struct {
	ID        string    `json:"id"`
	ChatRunID string    `json:"chatRunId"`
	Kind      string    `json:"kind"`
	Summary   string    `json:"summary"`
	Detail    string    `json:"detail,omitempty"`
	Status    RunStatus `json:"status,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChatApproval stores a paused state-changing model tool request. The app
// renders it with its own dialog and resumes work only after a response.
type ChatApproval struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversationId"`
	ChatRunID      string       `json:"chatRunId"`
	ToolCall       ChatToolCall `json:"toolCall"`
	Status         string       `json:"status"`
	CreatedAt      time.Time    `json:"createdAt"`
	ResolvedAt     *time.Time   `json:"resolvedAt,omitempty"`
}

// ChatPipeline is the compact picker projection for published chat triggers.
type ChatPipeline struct {
	BindingID    string `json:"bindingId"`
	PipelineID   string `json:"pipelineId"`
	PipelineName string `json:"pipelineName"`
	Label        string `json:"label"`
	Icon         string `json:"icon"`
	Color        string `json:"color"`
	Revision     int    `json:"revision"`
}

// ChatToolDefinition is shared by provider adapters and the assistant
// coordinator. InputSchema follows the OpenAI JSON-schema subset.
type ChatToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// AssistantChatRequest carries a full persisted transcript to one provider
// turn. Tool support is optional and adapter capability-dependent.
type AssistantChatRequest struct {
	Messages []ChatMessage        `json:"messages"`
	Tools    []ChatToolDefinition `json:"tools,omitempty"`
	Model    string               `json:"model,omitempty"`
	Metrics  LLMMetricContext     `json:"metrics,omitempty"`
}

// AssistantChatResponse contains normal text and/or native tool calls.
type AssistantChatResponse struct {
	Content   string         `json:"content"`
	ToolCalls []ChatToolCall `json:"toolCalls,omitempty"`
	Usage     LLMUsage       `json:"usage,omitempty"`
}

// ParseTags turns a comma, semicolon, or line-separated tag configuration
// into a stable, de-duplicated list suitable for persistence and filtering.
func ParseTags(value string) []string {
	return NormalizeTags(strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	}))
}

// NormalizeTags removes empty and duplicate tags while preserving their first
// display spelling and order. Tag matching is intentionally case-insensitive.
func NormalizeTags(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		tag := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		tag = strings.TrimPrefix(tag, "#")
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

type NodeRun struct {
	NodeID           string    `json:"nodeId"`
	NodeType         string    `json:"nodeType"`
	ParentNodeID     string    `json:"parentNodeId,omitempty"`
	FunctionID       string    `json:"functionId,omitempty"`
	FunctionRevision int       `json:"functionRevision,omitempty"`
	Status           RunStatus `json:"status"`
	Input            any       `json:"input,omitempty"`
	Output           any       `json:"output,omitempty"`
	Error            string    `json:"error,omitempty"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt"`
}

type NodeDefinition struct {
	Type          string            `json:"type"`
	Category      string            `json:"category"`
	Label         string            `json:"label"`
	Description   string            `json:"description"`
	Icon          string            `json:"icon"`
	Color         string            `json:"color"`
	Mode          NodeExecutionMode `json:"mode"`
	Inputs        []NodePort        `json:"inputs"`
	Outputs       []NodePort        `json:"outputs"`
	Fields        []ConfigField     `json:"fields"`
	Capabilities  []Capability      `json:"capabilities,omitempty"`
	DefaultConfig map[string]any    `json:"defaultConfig"`
	Source        string            `json:"source"`
}

type NodePort struct {
	ID             string       `json:"id"`
	Label          string       `json:"label"`
	Kind           PinKind      `json:"kind"`
	Direction      PinDirection `json:"direction"`
	DataType       DataType     `json:"dataType,omitempty"`
	Fields         []DataField  `json:"fields,omitempty"`
	Color          string       `json:"color,omitempty"`
	Required       bool         `json:"required,omitempty"`
	Default        any          `json:"default,omitempty"`
	MaxConnections int          `json:"maxConnections,omitempty"`
}

// DataField documents a known field within an object-valued data pin. Plugins
// can use dotted paths for nested values; it is metadata only and never leaks
// execution data between runs.
type DataField struct {
	Path        string   `json:"path"`
	Label       string   `json:"label,omitempty"`
	DataType    DataType `json:"dataType"`
	Description string   `json:"description,omitempty"`
	Optional    bool     `json:"optional,omitempty"`
}

// FunctionPin describes a stable public data contract for a custom function.
type FunctionPin struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	DataType DataType `json:"dataType"`
	Required bool     `json:"required,omitempty"`
	Default  any      `json:"default,omitempty"`
}

// CustomFunction is a globally reusable, versioned Blueprint function.
type CustomFunction struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Category          string            `json:"category"`
	Icon              string            `json:"icon"`
	IconColor         string            `json:"iconColor"`
	IconBackground    string            `json:"iconBackground"`
	Mode              NodeExecutionMode `json:"mode"`
	Inputs            []FunctionPin     `json:"inputs"`
	Outputs           []FunctionPin     `json:"outputs"`
	DraftDefinition   FlowDefinition    `json:"draftDefinition"`
	PublishedRevision int               `json:"publishedRevision"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

// FunctionSummary is the compact Functions-library card.
type FunctionSummary struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Category          string            `json:"category"`
	Icon              string            `json:"icon"`
	IconColor         string            `json:"iconColor"`
	IconBackground    string            `json:"iconBackground"`
	Mode              NodeExecutionMode `json:"mode"`
	PublishedRevision int               `json:"publishedRevision"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

type ConfigField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []Option `json:"options,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	VisibleWhen string   `json:"visibleWhen,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ProviderConfig struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Kind      ProviderKind `json:"kind"`
	BaseURL   string       `json:"baseUrl"`
	Model     string       `json:"model"`
	APIKeyRef string       `json:"apiKeyRef,omitempty"`
	Enabled   bool         `json:"enabled"`
}

// ModelPriceRate is an optional local estimate for one hosted provider/model.
// Values are expressed in USD per million tokens and are never treated as a
// provider billing record.
type ModelPriceRate struct {
	ProviderID          string  `json:"providerId"`
	Model               string  `json:"model"`
	InputUSDPerMillion  float64 `json:"inputUsdPerMillion"`
	OutputUSDPerMillion float64 `json:"outputUsdPerMillion"`
}

// MetricsSettings controls the local-only retention and sampling policy.
type MetricsSettings struct {
	DetailRetentionDays   int              `json:"detailRetentionDays"`
	RollupRetentionDays   int              `json:"rollupRetentionDays"`
	SampleIntervalSeconds int              `json:"sampleIntervalSeconds"`
	PriceRates            []ModelPriceRate `json:"priceRates"`
}

type Settings struct {
	Language          string `json:"language"`
	DefaultProviderID string `json:"defaultProviderId"`
	ContentDirectory  string `json:"contentDirectory"`
	RetentionDays     int    `json:"retentionDays"`
	// WebhookPort is retained to migrate pre-API settings. API.Port is the
	// active listener setting and is the only port used by new installations.
	WebhookPort          int                  `json:"webhookPort"`
	PluginDirectory      string               `json:"pluginDirectory"`
	Providers            []ProviderConfig     `json:"providers"`
	MaxConcurrentRuns    int                  `json:"maxConcurrentRuns"`
	MaxConcurrentLLMRuns int                  `json:"maxConcurrentLLMRuns"`
	LlamaRuntime         LlamaRuntimeSettings `json:"llamaRuntime"`
	API                  APISettings          `json:"api"`
	Metrics              MetricsSettings      `json:"metrics"`
}

// LLMMetricContext identifies a call without retaining its prompt, response,
// credentials, input payload, or tool arguments.
type LLMMetricContext struct {
	ExecutionID string `json:"executionId,omitempty"`
	ChatRunID   string `json:"chatRunId,omitempty"`
	PipelineID  string `json:"pipelineId,omitempty"`
	NodeID      string `json:"nodeId,omitempty"`
	NodeType    string `json:"nodeType,omitempty"`
	Origin      string `json:"origin,omitempty"`
}

// LLMUsage is the safe numerical projection returned by a provider adapter.
// A zero token count with TokensReported false means the provider omitted usage.
type LLMUsage struct {
	ProviderID       string           `json:"providerId"`
	ProviderName     string           `json:"providerName"`
	ProviderKind     ProviderKind     `json:"providerKind"`
	Model            string           `json:"model"`
	PromptTokens     int64            `json:"promptTokens"`
	CompletionTokens int64            `json:"completionTokens"`
	TokensReported   bool             `json:"tokensReported"`
	QueueWait        time.Duration    `json:"queueWait"`
	Duration         time.Duration    `json:"duration"`
	Succeeded        bool             `json:"succeeded"`
	EstimatedCostUSD *float64         `json:"estimatedCostUsd,omitempty"`
	Context          LLMMetricContext `json:"context"`
	OccurredAt       time.Time        `json:"occurredAt"`
}

// MetricsFilter limits a local dashboard query. Empty dimensions mean all.
type MetricsFilter struct {
	From         time.Time     `json:"from"`
	To           time.Time     `json:"to"`
	PipelineIDs  []string      `json:"pipelineIds,omitempty"`
	ProviderIDs  []string      `json:"providerIds,omitempty"`
	Models       []string      `json:"models,omitempty"`
	TriggerKinds []TriggerKind `json:"triggerKinds,omitempty"`
	Statuses     []RunStatus   `json:"statuses,omitempty"`
}

// MetricsKPI is a current value together with its comparable prior period.
type MetricsKPI struct {
	Value         float64 `json:"value"`
	PreviousValue float64 `json:"previousValue"`
	Available     bool    `json:"available"`
}

type MetricsPoint struct {
	At     time.Time `json:"at"`
	Value  float64   `json:"value"`
	Value2 float64   `json:"value2,omitempty"`
	Value3 float64   `json:"value3,omitempty"`
}

type MetricsRunPoint struct {
	At        time.Time `json:"at"`
	Completed int       `json:"completed"`
	Failed    int       `json:"failed"`
	Skipped   int       `json:"skipped"`
	Cancelled int       `json:"cancelled"`
}

type MetricsBreakdown struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
	Secondary float64 `json:"secondary,omitempty"`
}

type MetricsPipelineHealth struct {
	PipelineID string    `json:"pipelineId"`
	Name       string    `json:"name"`
	At         time.Time `json:"at"`
	Completed  int       `json:"completed"`
	Failed     int       `json:"failed"`
}

type MetricsResourcePoint struct {
	At         time.Time `json:"at"`
	Process    string    `json:"process"`
	CPUPercent float64   `json:"cpuPercent"`
	WorkingSet int64     `json:"workingSet"`
}

// MetricExecutionEvent is the payload-free detailed fact retained for a
// completed, skipped, or cancelled pipeline run.
type MetricExecutionEvent struct {
	ExecutionID     string      `json:"executionId"`
	PipelineID      string      `json:"pipelineId"`
	PipelineName    string      `json:"pipelineName"`
	TriggerKind     TriggerKind `json:"triggerKind"`
	Status          RunStatus   `json:"status"`
	OccurredAt      time.Time   `json:"occurredAt"`
	DurationMS      float64     `json:"durationMs"`
	QueueWaitMS     float64     `json:"queueWaitMs"`
	NodeCount       int         `json:"nodeCount"`
	FailedNodeCount int         `json:"failedNodeCount"`
}

type MetricNodeEvent struct {
	ExecutionID string    `json:"executionId"`
	PipelineID  string    `json:"pipelineId"`
	NodeType    string    `json:"nodeType"`
	Status      RunStatus `json:"status"`
	OccurredAt  time.Time `json:"occurredAt"`
	DurationMS  float64   `json:"durationMs"`
}

type MetricActivityEvent struct {
	Kind       string    `json:"kind"`
	Outcome    string    `json:"outcome"`
	DurationMS float64   `json:"durationMs"`
	OccurredAt time.Time `json:"occurredAt"`
}

// MetricsOverview is the complete typed response rendered by the Metrics tab.
type MetricsOverview struct {
	Filter            MetricsFilter           `json:"filter"`
	Granularity       string                  `json:"granularity"`
	Runs              MetricsKPI              `json:"runs"`
	SuccessRate       MetricsKPI              `json:"successRate"`
	AverageDurationMS MetricsKPI              `json:"averageDurationMs"`
	P95DurationMS     MetricsKPI              `json:"p95DurationMs"`
	LLMCalls          MetricsKPI              `json:"llmCalls"`
	PromptTokens      MetricsKPI              `json:"promptTokens"`
	CompletionTokens  MetricsKPI              `json:"completionTokens"`
	EstimatedCostUSD  MetricsKPI              `json:"estimatedCostUsd"`
	RunSeries         []MetricsRunPoint       `json:"runSeries"`
	DurationSeries    []MetricsPoint          `json:"durationSeries"`
	LLMSeries         []MetricsPoint          `json:"llmSeries"`
	QueueSeries       []MetricsPoint          `json:"queueSeries"`
	Pipelines         []MetricsPipelineHealth `json:"pipelines"`
	Failures          []MetricsBreakdown      `json:"failures"`
	SlowNodes         []MetricsBreakdown      `json:"slowNodes"`
	Models            []MetricsBreakdown      `json:"models"`
	Triggers          []MetricsBreakdown      `json:"triggers"`
	Activity          []MetricsBreakdown      `json:"activity"`
	Resources         []MetricsResourcePoint  `json:"resources"`
	TokensUnavailable int                     `json:"tokensUnavailable"`
	UnpricedCalls     int                     `json:"unpricedCalls"`
	LocalCalls        int                     `json:"localCalls"`
}

// APIAuthMode determines how callers authenticate to Neuropipe's optional
// local HTTP API. The no-auth mode intentionally cannot enable admin routes.
type APIAuthMode string

const (
	APIAuthToken APIAuthMode = "token"
	APIAuthNone  APIAuthMode = "none"
)

// APISettings controls the embedded Fiber server. TokenRef only identifies a
// DPAPI-protected vault record; the token value is never serialised to React.
type APISettings struct {
	Enabled              bool        `json:"enabled"`
	BindAddress          string      `json:"bindAddress"`
	Port                 int         `json:"port"`
	AuthMode             APIAuthMode `json:"authMode"`
	TokenRef             string      `json:"tokenRef,omitempty"`
	AllowedOrigins       []string    `json:"allowedOrigins"`
	AdminEnabled         bool        `json:"adminEnabled"`
	ExposureAcknowledged bool        `json:"exposureAcknowledged"`
}

// APIStatus is safe to render in Settings and never includes a token or
// connection internals.
type APIStatus struct {
	Running         bool   `json:"running"`
	Endpoint        string `json:"endpoint,omitempty"`
	TokenConfigured bool   `json:"tokenConfigured"`
	Message         string `json:"message,omitempty"`
}

// UpdateAvailability is safe release metadata for the desktop title bar. It
// never includes the request details or any locally stored data.
type UpdateAvailability struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	URL       string `json:"url,omitempty"`
}

type RuntimeMode string

const (
	RuntimeAuto   RuntimeMode = "auto"
	RuntimeCPU    RuntimeMode = "cpu"
	RuntimeCUDA   RuntimeMode = "cuda"
	RuntimeVulkan RuntimeMode = "vulkan"
	RuntimeHIP    RuntimeMode = "hip"
)

// LlamaRuntimeSettings identifies the locally managed llama.cpp executable and model.
type LlamaRuntimeSettings struct {
	BinaryPath     string      `json:"binaryPath"`
	ModelPath      string      `json:"modelPath"`
	RuntimeVersion string      `json:"runtimeVersion,omitempty"`
	Mode           RuntimeMode `json:"mode"`
	ContextSize    int         `json:"contextSize"`
	AutoStart      bool        `json:"autoStart"`
}

// LlamaRuntimeStatus is safe to expose to the renderer and never includes process internals.
type LlamaRuntimeStatus struct {
	Running  bool        `json:"running"`
	Endpoint string      `json:"endpoint"`
	Mode     RuntimeMode `json:"mode"`
	Model    string      `json:"model"`
	Message  string      `json:"message"`
}

// RuntimeArtifact identifies a verified file offered by an official release.
type RuntimeArtifact struct {
	URL    string `json:"url,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// InstallProgress describes the current state of a managed runtime or model install.
// It intentionally contains no local paths or credentials, so it is safe to emit to React.
type InstallProgress struct {
	Kind            string  `json:"kind"`
	Stage           string  `json:"stage"`
	Label           string  `json:"label"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	BytesPerSecond  float64 `json:"bytesPerSecond"`
	Percentage      int     `json:"percentage"`
}

// LlamaRuntimeRelease is a compatible official llama.cpp Windows release.
type LlamaRuntimeRelease struct {
	Version     string          `json:"version"`
	PublishedAt string          `json:"publishedAt,omitempty"`
	CPU         RuntimeArtifact `json:"cpu"`
	CUDA        RuntimeArtifact `json:"cuda"`
	Vulkan      RuntimeArtifact `json:"vulkan"`
	HIP         RuntimeArtifact `json:"hip"`
}

// LlamaRuntimeInstallRequest selects one official runtime build to install.
type LlamaRuntimeInstallRequest struct {
	Version string      `json:"version"`
	Mode    RuntimeMode `json:"mode"`
}

// InstalledLlamaRuntime reports the available acceleration builds per release.
type InstalledLlamaRuntime struct {
	Version         string `json:"version"`
	CPUInstalled    bool   `json:"cpuInstalled"`
	CUDAInstalled   bool   `json:"cudaInstalled"`
	VulkanInstalled bool   `json:"vulkanInstalled"`
	HIPInstalled    bool   `json:"hipInstalled"`
}

// LlamaRuntimeCatalogStatus is the safe state of user-owned runtime files.
type LlamaRuntimeCatalogStatus struct {
	Root            string                  `json:"root"`
	SelectedVersion string                  `json:"selectedVersion,omitempty"`
	Installed       []InstalledLlamaRuntime `json:"installed"`
}

// ModelSearchResult is a public, non-gated GGUF repository returned by Hugging Face.
type ModelSearchResult struct {
	ID           string   `json:"id"`
	Author       string   `json:"author,omitempty"`
	AvatarURL    string   `json:"avatarUrl,omitempty"`
	Downloads    int64    `json:"downloads"`
	Likes        int      `json:"likes"`
	LastModified string   `json:"lastModified,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ModelSearchRequest drives a GGUF-only Hugging Face discovery query.
type ModelSearchRequest struct {
	Query string `json:"query"`
	Sort  string `json:"sort"`
}

// ModelFile is one downloadable GGUF file, including size and LFS checksum when supplied.
type ModelFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256,omitempty"`
	Quantization string `json:"quantization,omitempty"`
	Recommended  bool   `json:"recommended,omitempty"`
}

// ModelDetail contains public Hub metadata and a bounded Markdown model card.
// It has no credentials or local installation paths.
type ModelDetail struct {
	ID           string      `json:"id"`
	Author       string      `json:"author,omitempty"`
	AvatarURL    string      `json:"avatarUrl,omitempty"`
	Downloads    int64       `json:"downloads"`
	Likes        int         `json:"likes"`
	LastModified string      `json:"lastModified,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	Readme       string      `json:"readme,omitempty"`
	Files        []ModelFile `json:"files"`
}

// InstalledModelMetadata is the durable Hugging Face identity recorded beside
// a completed GGUF. It deliberately excludes the model card so installed-model
// browsing stays fast and does not duplicate a potentially large README.
type InstalledModelMetadata struct {
	SchemaVersion int      `json:"schemaVersion"`
	Repository    string   `json:"repository"`
	File          string   `json:"file"`
	Quantization  string   `json:"quantization,omitempty"`
	SHA256        string   `json:"sha256,omitempty"`
	Author        string   `json:"author,omitempty"`
	AvatarURL     string   `json:"avatarUrl,omitempty"`
	Downloads     int64    `json:"downloads"`
	Likes         int      `json:"likes"`
	LastModified  string   `json:"lastModified,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	InstalledAt   string   `json:"installedAt"`
}

// ModelInstallRequest selects a public Hugging Face GGUF file.
type ModelInstallRequest struct {
	Repository string `json:"repository"`
	File       string `json:"file"`
}

// LocalModel is an installed GGUF file safe to expose to the renderer.
type LocalModel struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Size         int64    `json:"size"`
	Repository   string   `json:"repository,omitempty"`
	Author       string   `json:"author,omitempty"`
	AvatarURL    string   `json:"avatarUrl,omitempty"`
	Downloads    int64    `json:"downloads"`
	Likes        int      `json:"likes"`
	LastModified string   `json:"lastModified,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Quantization string   `json:"quantization,omitempty"`
	SHA256       string   `json:"sha256,omitempty"`
	InstalledAt  string   `json:"installedAt,omitempty"`
}

type PluginStatus struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Version            string `json:"version"`
	Path               string `json:"path"`
	Enabled            bool   `json:"enabled"`
	Healthy            bool   `json:"healthy"`
	NodeCount          int    `json:"nodeCount"`
	Description        string `json:"description"`
	Error              string `json:"error,omitempty"`
	DocumentationError string `json:"documentationError,omitempty"`
}

// DocumentationEntry is the navigation metadata for a local Markdown page.
// Category is an ordered path, allowing the client to build a tree without
// knowing whether the document was supplied by the core app or a plugin.
type DocumentationEntry struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary,omitempty"`
	Category  []string `json:"category"`
	NodeTypes []string `json:"nodeTypes,omitempty"`
	Source    string   `json:"source"`
	PluginID  string   `json:"pluginId,omitempty"`
}

// DocumentationDocument is a complete local Markdown document. Markdown is
// deliberately supplied only through the Desktop facade; the renderer never
// reads local files or plugin bundles directly.
type DocumentationDocument struct {
	DocumentationEntry
	Markdown string `json:"markdown"`
}

// DocumentationSearchResult contains a concise local match preview. It never
// includes raw filesystem paths or a plugin bundle path.
type DocumentationSearchResult struct {
	Document DocumentationEntry `json:"document"`
	Excerpt  string             `json:"excerpt"`
}

// DocumentationReference resolves a node type to a document and optionally a
// stable heading anchor within it.
type DocumentationReference struct {
	DocumentID string `json:"documentId"`
	Anchor     string `json:"anchor,omitempty"`
}

type PermissionGrant struct {
	PipelineID string     `json:"pipelineId"`
	Revision   int        `json:"revision"`
	Capability Capability `json:"capability"`
	Scope      string     `json:"scope"`
	GrantedAt  time.Time  `json:"grantedAt"`
}
