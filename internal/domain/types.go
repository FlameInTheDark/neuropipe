// Package domain owns Neuropipe's serialisable business contracts.
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type PipelineStatus string

const (
	PipelineDraft    PipelineStatus = "draft"
	PipelineActive   PipelineStatus = "active"
	PipelineArchived PipelineStatus = "archived"
)

// GraphSchemaV3 is the only supported Blueprint graph schema. It adds
// explicit recursive wire contracts: pin types are enforced before and during
// execution.
const GraphSchemaV3 = 3

// PinKind separates control flow from values. Only exec pins can execute a
// node; data pins are resolved by the active execution frame on demand.
type PinKind string

const (
	PinExec PinKind = "exec"
	PinData PinKind = "data"
	// PinTool connects a published LLM tool function to an AI node. It is
	// declarative: unlike an Exec pin it never participates in graph traversal.
	PinTool PinKind = "tool"
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
	// DataBytes is the display label for the bytes wire contract. A pin
	// declared bytes carries raw binary data ([]byte) and never accepts
	// implicit text conversion; use an explicit encoder node to bridge.
	DataBytes DataType = "bytes"
)

// TypeKind is the JSON-safe subset of Go types available on Blueprint data
// pins. Conversion is never implicit: an int is not a float or string merely
// because the runtime could represent it that way.
type TypeKind string

const (
	TypeAny    TypeKind = "any"
	TypeBool   TypeKind = "bool"
	TypeString TypeKind = "string"
	TypeInt    TypeKind = "int"
	TypeFloat  TypeKind = "float"
	TypeBytes  TypeKind = "bytes"
	TypeList   TypeKind = "list"
	TypeMap    TypeKind = "map"
	TypeRecord TypeKind = "record"
)

// TypeSpec declares the complete contract of a data pin. Name makes a record
// nominal (like a named Go struct); unnamed records are structural contracts.
// Element, Key, Value, and Fields are used only by their matching Kind.
type TypeSpec struct {
	Kind    TypeKind        `json:"kind"`
	Name    string          `json:"name,omitempty"`
	Element *TypeSpec       `json:"element,omitempty"`
	Key     *TypeSpec       `json:"key,omitempty"`
	Value   *TypeSpec       `json:"value,omitempty"`
	Fields  []TypeFieldSpec `json:"fields,omitempty"`
}

// TypeFieldSpec is one field within a structural or named record contract.
type TypeFieldSpec struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     TypeSpec `json:"type"`
	Optional bool     `json:"optional,omitempty"`
}

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
	// TriggerTwitch starts a trusted pipeline from a Twitch EventSub event.
	TriggerTwitch TriggerKind = "twitch"
	// TriggerKV starts a trusted pipeline from a Redis pub/sub message
	// received on a registered KV connection.
	TriggerKV TriggerKind = "kvsubscribe"
	// TriggerDiscord starts a trusted pipeline from a Discord gateway
	// event received by a connected bot identity.
	TriggerDiscord TriggerKind = "discord"
	// TriggerTelegram starts a trusted pipeline from a Telegram Bot API
	// update received by a connected bot identity.
	TriggerTelegram TriggerKind = "telegram"
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
	ProviderAnthropic        ProviderKind = "anthropic"
)

// ValidProviderKinds lists every provider kind the settings UI may configure.
func ValidProviderKinds() []ProviderKind {
	return []ProviderKind{ProviderOllama, ProviderLlamaCPP, ProviderOpenAICompatible, ProviderAnthropic}
}

// IsValidProviderKind reports whether kind is a configurable provider kind.
func IsValidProviderKind(kind ProviderKind) bool {
	switch kind {
	case ProviderOllama, ProviderLlamaCPP, ProviderOpenAICompatible, ProviderAnthropic:
		return true
	default:
		return false
	}
}

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
	// HasUnpublishedChanges distinguishes an editable draft from the immutable
	// revision that triggers currently run. Trust is revision-scoped and must
	// never make the draft read-only.
	HasUnpublishedChanges bool `json:"hasUnpublishedChanges"`
	// ExecutorID targets the pipeline at a remote executor. Empty means the
	// pipeline runs inside this desktop installation.
	ExecutorID string    `json:"executorId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
	ExecutorID        string         `json:"executorId,omitempty"`
	ExecutorName      string         `json:"executorName,omitempty"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type FlowDefinition struct {
	SchemaVersion int        `json:"schemaVersion"`
	Nodes         []FlowNode `json:"nodes"`
	Edges         []FlowEdge `json:"edges"`
	Viewport      Viewport   `json:"viewport"`
	// Groups is renderer-owned presentation metadata (visual frames). The
	// engine never reads it; it only round-trips through save/load so the
	// editor can restore the user's layout aids.
	Groups []NodeGroup `json:"groups,omitempty"`
	// Comments is renderer-owned sticky notes. Purely cosmetic.
	Comments []NodeComment `json:"comments,omitempty"`
}

// NodeGroup is one visual frame grouping a set of nodes on the canvas.
// Purely cosmetic — membership is not enforced by the engine.
type NodeGroup struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Color string  `json:"color"`
}

// NodeComment is a sticky note on the canvas. Purely cosmetic.
type NodeComment struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Color string  `json:"color"`
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
	// Waypoints are editor-only wire layout hints; validation and execution
	// deliberately ignore them.
	Waypoints []Position `json:"waypoints,omitempty"`
}

type TriggerBinding struct {
	ID         string      `json:"id"`
	PipelineID string      `json:"pipelineId"`
	NodeID     string      `json:"nodeId"`
	Revision   int         `json:"revision"`
	Kind       TriggerKind `json:"kind"`
	// NodeType and Config are the immutable, canonical trigger-node metadata
	// used by external trigger services. They contain no secret values.
	NodeType      string         `json:"nodeType,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	Label         string         `json:"label"`
	Icon          string         `json:"icon"`
	Color         string         `json:"color"`
	GridPosition  int            `json:"gridPosition"`
	Hotkey        string         `json:"hotkey,omitempty"`
	Cron          string         `json:"cron,omitempty"`
	Timezone      string         `json:"timezone,omitempty"`
	Enabled       bool           `json:"enabled"`
	Trusted       bool           `json:"trusted"`
	NextRunAt     *time.Time     `json:"nextRunAt,omitempty"`
	LastRunAt     *time.Time     `json:"lastRunAt,omitempty"`
	LastRunStatus RunStatus      `json:"lastRunStatus,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
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
	// ExecutorID is set when the run was executed by a remote executor.
	ExecutorID string `json:"executorId,omitempty"`
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
// trigger binding; model conversations use the active Settings provider
// unless they carry an explicit provider/model override. Model conversations
// can also offer published LLM tool functions (ToolIDs) and request a
// reasoning effort; RenamedAt records when the model's one-shot
// rename_conversation tool was used so both the tool and its system-prompt
// rule disappear after the first successful rename.
type ChatConversation struct {
	ID               string           `json:"id"`
	Mode             ChatMode         `json:"mode"`
	Title            string           `json:"title"`
	PipelineID       string           `json:"pipelineId,omitempty"`
	TriggerBindingID string           `json:"triggerBindingId,omitempty"`
	ActionPolicy     ChatActionPolicy `json:"actionPolicy"`
	ProviderID       string           `json:"providerId,omitempty"`
	Model            string           `json:"model,omitempty"`
	Reasoning        string           `json:"reasoning,omitempty"`
	ToolIDs          []string         `json:"toolIds,omitempty"`
	RenamedAt        *time.Time       `json:"renamedAt,omitempty"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// ChatReasoningEffort enumerates the reasoning effort levels selectable per
// conversation. Empty means the provider default (no reasoning field sent).
type ChatReasoningEffort string

const (
	ChatReasoningDefault ChatReasoningEffort = ""
	ChatReasoningNone    ChatReasoningEffort = "none"
	ChatReasoningMinimal ChatReasoningEffort = "minimal"
	ChatReasoningLow     ChatReasoningEffort = "low"
	ChatReasoningMedium  ChatReasoningEffort = "medium"
	ChatReasoningHigh    ChatReasoningEffort = "high"
)

// ValidChatReasoning reports whether a persisted reasoning value can be
// forwarded to a provider adapter.
func ValidChatReasoning(value string) bool {
	switch ChatReasoningEffort(value) {
	case ChatReasoningDefault, ChatReasoningNone, ChatReasoningMinimal, ChatReasoningLow, ChatReasoningMedium, ChatReasoningHigh:
		return true
	}
	return false
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
// turn. Tool support is optional and adapter capability-dependent. Reasoning
// uses the ai-sdk effort vocabulary ("", "none", "minimal", "low", "medium",
// "high"); empty keeps the provider default and sends nothing.
type AssistantChatRequest struct {
	Messages   []ChatMessage        `json:"messages"`
	Tools      []ChatToolDefinition `json:"tools,omitempty"`
	ProviderID string               `json:"providerId,omitempty"`
	Model      string               `json:"model,omitempty"`
	Reasoning  string               `json:"reasoning,omitempty"`
	Metrics    LLMMetricContext     `json:"metrics,omitempty"`
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
	Type        string            `json:"type"`
	Category    string            `json:"category"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Color       string            `json:"color"`
	Mode        NodeExecutionMode `json:"mode"`
	// TriggerKind lets publishing derive bindings from module metadata instead
	// of duplicating node-type switches in application services.
	TriggerKind TriggerKind `json:"triggerKind,omitempty"`
	// PortContractOwned marks definitions whose input and output ports are
	// complete module contracts. The catalog must not append generic
	// payload/result pins to these definitions.
	PortContractOwned bool           `json:"portContractOwned,omitempty"`
	Inputs            []NodePort     `json:"inputs"`
	Outputs           []NodePort     `json:"outputs"`
	Fields            []ConfigField  `json:"fields"`
	Capabilities      []Capability   `json:"capabilities,omitempty"`
	DefaultConfig     map[string]any `json:"defaultConfig"`
	Source            string         `json:"source"`
}

type NodePort struct {
	ID             string       `json:"id"`
	Label          string       `json:"label"`
	Kind           PinKind      `json:"kind"`
	Direction      PinDirection `json:"direction"`
	DataType       DataType     `json:"dataType,omitempty"`
	Type           *TypeSpec    `json:"type,omitempty"`
	Fields         []DataField  `json:"fields,omitempty"`
	Color          string       `json:"color,omitempty"`
	Required       bool         `json:"required,omitempty"`
	Default        any          `json:"default,omitempty"`
	MaxConnections int          `json:"maxConnections,omitempty"`
	// IgnoreConfigFallback leaves a matching inspector field out of runtime
	// inputs so the node can parse that field through its explicit literal path.
	IgnoreConfigFallback bool `json:"-"`
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
// Description is model-facing guidance when the function is exposed as an LLM
// tool. It explains the meaning and constraints of the value without changing
// its strict TypeSpec.
type FunctionPin struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DataType    DataType  `json:"dataType"`
	Type        *TypeSpec `json:"type,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Default     any       `json:"default,omitempty"`
}

// FunctionKind determines how a published function is exposed in the node
// catalogue. Standard functions are callable Blueprint nodes; tool functions
// can only be connected to an LLM node's Tools pin.
type FunctionKind string

const (
	FunctionStandard FunctionKind = "function"
	FunctionTool     FunctionKind = "tool"
)

// CreateFunctionRequest contains the metadata selected before opening a new
// function canvas. Tool functions always execute as an impure subgraph.
type CreateFunctionRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Kind        FunctionKind      `json:"kind"`
	Mode        NodeExecutionMode `json:"mode"`
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
	Kind              FunctionKind      `json:"kind"`
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
	Kind              FunctionKind      `json:"kind"`
	Mode              NodeExecutionMode `json:"mode"`
	PublishedRevision int               `json:"publishedRevision"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

// GlobalVariable is a workspace-scoped, typed, persisted variable. Name is the
// immutable identifier stored inside node configurations; DataType constrains
// writes; DefaultValue is returned when no value has been written yet.
type GlobalVariable struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	DataType     DataType `json:"dataType"`
	DefaultValue any      `json:"defaultValue"`
	// Value is the current in-memory content; it falls back to DefaultValue
	// until the variables service hydrates its SQLite snapshot.
	Value     any       `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SaveGlobalVariableRequest carries only the user-editable fields over the
// Wails boundary. IDs, timestamps, and live values are server-owned, which
// also keeps Wails from unmarshalling empty time strings.
type SaveGlobalVariableRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	DataType     DataType `json:"dataType"`
	DefaultValue any      `json:"defaultValue"`
}

// DatabaseDriver identifies the SQL dialect of a registered database.
type DatabaseDriver string

const (
	DatabaseDriverSQLite   DatabaseDriver = "sqlite"
	DatabaseDriverPostgres DatabaseDriver = "postgres"
	DatabaseDriverMySQL    DatabaseDriver = "mysql"
	DatabaseDriverDuckDB   DatabaseDriver = "duckdb"
	// DatabaseDriverRedis is the Redis-protocol key/value family (Redis,
	// Valkey, KeyDB, Dragonfly). All flavours share the RESP wire protocol and
	// are served by the same go-redis client; the server flavour is reported
	// for display only.
	DatabaseDriverRedis DatabaseDriver = "redis"
	// DatabaseDriverSugarDB is the embedded SugarDB store: an in-process
	// key/value engine that speaks RESP on a loopback listener, so the same
	// go-redis client stack serves it without any external server. Data
	// persists under the configured directory (Path) via AOF and snapshots.
	DatabaseDriverSugarDB DatabaseDriver = "sugardb"
)

// IsKVDriver reports whether the driver belongs to the key/value family served
// by the KV service (remote Redis-protocol servers and the embedded SugarDB
// store). Callers gate routing, node pickers, and browser views on this instead
// of comparing against a single driver constant.
func IsKVDriver(driver DatabaseDriver) bool {
	return driver == DatabaseDriverRedis || driver == DatabaseDriverSugarDB
}

// DatabaseStatus reports the last-known connection state of a database.
type DatabaseStatus string

const (
	DatabaseStatusUnknown    DatabaseStatus = "unknown"
	DatabaseStatusConnected  DatabaseStatus = "connected"
	DatabaseStatusError      DatabaseStatus = "error"
	DatabaseStatusUnverified DatabaseStatus = "unverified"
)

// Database is a user-registered database connection. SQLite uses Path;
// Postgres and MySQL use Host/Port/Database/Username/PasswordRef. Redis-family
// stores use Host/Port/Username/PasswordRef plus the KV-specific fields below;
// Address optionally carries a complete redis:// URL that overrides them.
// The embedded SugarDB driver reuses Path as its persistence directory and
// ignores Host/Port/Address entirely - the engine runs inside the app.
type Database struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Driver      DatabaseDriver `json:"driver"`
	Path        string         `json:"path,omitempty"`
	Host        string         `json:"host,omitempty"`
	Port        int            `json:"port,omitempty"`
	Database    string         `json:"database,omitempty"`
	Username    string         `json:"username,omitempty"`
	PasswordRef string         `json:"passwordRef,omitempty"`
	Schema      string         `json:"schema,omitempty"`
	SSLMode     string         `json:"sslMode,omitempty"`
	Charset     string         `json:"charset,omitempty"`
	Options     string         `json:"options,omitempty"`
	// KV-specific connection settings (drivers "redis" and "sugardb").
	DBIndex    int            `json:"dbIndex,omitempty"`
	UseTLS     bool           `json:"useTLS,omitempty"`
	ClientName string         `json:"clientName,omitempty"`
	Address    string         `json:"address,omitempty"`
	Status     DatabaseStatus `json:"status"`
	LastPingAt *time.Time     `json:"lastPingAt,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// SaveDatabaseRequest carries editable database metadata across Wails.
// Password is write-only (never returned in reads).
type SaveDatabaseRequest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Driver      DatabaseDriver `json:"driver"`
	Path        string         `json:"path,omitempty"`
	Host        string         `json:"host,omitempty"`
	Port        int            `json:"port,omitempty"`
	Database    string         `json:"database,omitempty"`
	Username    string         `json:"username,omitempty"`
	PasswordRef string         `json:"passwordRef,omitempty"`
	Password    string         `json:"password,omitempty"`
	Schema      string         `json:"schema,omitempty"`
	SSLMode     string         `json:"sslMode,omitempty"`
	Charset     string         `json:"charset,omitempty"`
	Options     string         `json:"options,omitempty"`
	// KV-specific connection settings (drivers "redis" and "sugardb").
	DBIndex    int    `json:"dbIndex,omitempty"`
	UseTLS     bool   `json:"useTLS,omitempty"`
	ClientName string `json:"clientName,omitempty"`
	Address    string `json:"address,omitempty"`
}

// DatabaseSchema is the inspectable SQLite catalog exposed to the editor.
type DatabaseSchema struct {
	Tables []DatabaseTable `json:"tables"`
}

type DatabaseTable struct {
	Name    string           `json:"name"`
	Columns []DatabaseColumn `json:"columns"`
	Indexes []DatabaseIndex  `json:"indexes"`
}

type DatabaseColumn struct {
	Name       string  `json:"name"`
	DataType   string  `json:"dataType"`
	Nullable   bool    `json:"nullable"`
	PrimaryKey bool    `json:"primaryKey"`
	Default    *string `json:"default,omitempty"`
}

type DatabaseIndex struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

// SQLParameter is the persisted dynamic input contract of an action:sql node.
type SQLParameter struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     TypeSpec `json:"type"`
	Required bool     `json:"required,omitempty"`
}

// SQLArgument is one named, safely-bound query value.
type SQLArgument struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// SQLRequest is the narrow execution contract shared by the node and service.
type SQLRequest struct {
	DatabaseID string        `json:"databaseId"`
	SQL        string        `json:"sql"`
	Parameters []SQLArgument `json:"parameters"`
	MaxRows    int           `json:"maxRows,omitempty"`
}

// SQLResult contains only JSON-safe values suitable for Blueprint packets.
type SQLResult struct {
	Columns      []string         `json:"columns"`
	Rows         []map[string]any `json:"rows"`
	RowsAffected int64            `json:"rowsAffected"`
	LastInsertID *int64           `json:"lastInsertId,omitempty"`
	Truncated    bool             `json:"truncated"`
}

// SQLDebugRequest is the Wails-facing query preview contract.
type SQLDebugRequest struct {
	DatabaseID string        `json:"databaseId"`
	SQL        string        `json:"sql"`
	Parameters []SQLArgument `json:"parameters"`
	MaxRows    int           `json:"maxRows,omitempty"`
}

/* ------------------------------------------------------------------ */
/* Key/value (Redis protocol) contracts                                */
/* ------------------------------------------------------------------ */

// KVArgument is the persisted dynamic input contract of an action:kv_command
// node. It mirrors SQLParameter so the editor reuses the same structured
// parameter UI; Name is only a label because Redis arguments are positional.
type KVArgument struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     TypeSpec `json:"type"`
	Required bool     `json:"required,omitempty"`
}

// KVCommandRequest is the narrow execution contract shared by KV nodes and
// the KV service. Args carries the command's positional arguments as strings;
// typed pin values are converted by the node before crossing this boundary.
type KVCommandRequest struct {
	DatabaseID     string   `json:"databaseId"`
	Command        string   `json:"command"`
	Args           []string `json:"args"`
	MaxResults     int      `json:"maxResults,omitempty"`
	AllowDangerous bool     `json:"allowDangerous,omitempty"`
}

// KVCommandResult contains only JSON-safe values suitable for Blueprint
// packets. IsNil reports a Redis nil reply (missing key), which is a valid
// outcome rather than an error.
type KVCommandResult struct {
	Value     any  `json:"value"`
	IsNil     bool `json:"isNil"`
	Truncated bool `json:"truncated,omitempty"`
}

// KVScanRequest pages through keys with cursor-based SCAN.
type KVScanRequest struct {
	Cursor uint64 `json:"cursor"`
	Match  string `json:"match,omitempty"`
	Type   string `json:"type,omitempty"`
	Count  int    `json:"count,omitempty"`
}

// KVKeyPage is one SCAN page enriched with per-key type, TTL, and size.
type KVKeyPage struct {
	Keys       []KVKey `json:"keys"`
	NextCursor uint64  `json:"nextCursor"`
	TotalSeen  int     `json:"totalSeen"`
}

// KVKey describes one key in the browser. TTL uses the Redis convention:
// -1 means no expiry, -2 means the key no longer exists.
type KVKey struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	TTL      int64  `json:"ttl"`
	Encoding string `json:"encoding,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// KVKeyValue is the value viewer payload for one key. Value's shape depends
// on Type: string, map[string]string (hash), []string (list, set),
// []map[string]any with member/score (zset), or []map[string]any with
// id/fields (stream).
type KVKeyValue struct {
	Type      string `json:"type"`
	Value     any    `json:"value"`
	TTL       int64  `json:"ttl"`
	Truncated bool   `json:"truncated,omitempty"`
}

// KVServerInfo is the display-only server summary shown on the browser's
// Info tab. Fields missing from a flavour's INFO output stay zero-valued.
type KVServerInfo struct {
	Flavor           string           `json:"flavor"`
	Version          string           `json:"version"`
	UptimeSeconds    int64            `json:"uptimeSeconds"`
	ConnectedClients int64            `json:"connectedClients"`
	UsedMemory       int64            `json:"usedMemory"`
	UsedMemoryHuman  string           `json:"usedMemoryHuman"`
	TotalKeys        int64            `json:"totalKeys"`
	Databases        []KVDatabaseInfo `json:"databases"`
}

// KVDatabaseInfo is the key count of one logical Redis database index.
type KVDatabaseInfo struct {
	Index int   `json:"index"`
	Keys  int64 `json:"keys"`
}

// GlobalVariableSummary is the compact Variables-library card. Value carries
// the current content so the list view can preview it without a follow-up call.
type GlobalVariableSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DataType    DataType  `json:"dataType"`
	Value       any       `json:"value"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ValidateGlobalVariableName enforces the same identifier rule as local
// execution variables so node configs and database keys remain stable.
var globalVariableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidateGlobalVariableName(name string) error {
	if !globalVariableNamePattern.MatchString(name) {
		return fmt.Errorf("variable name must start with a letter or underscore and contain only letters, numbers, or underscores")
	}
	return nil
}

// ValidDataType reports whether the value is a recognised pinned data type.
func ValidDataType(dataType DataType) bool {
	switch dataType {
	case DataAny, DataText, DataNumber, DataBoolean, DataObject, DataList, DataBytes:
		return true
	default:
		return false
	}
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
	// Dynamic names an asynchronous option source rendered by the inspector
	// ("pipelines"); it overrides static Options when set.
	Dynamic string `json:"dynamic,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ModelConfig is one manually configured or discovered model of a provider.
// ID is the provider-facing model key; Name is the display title shown in
// pickers. A model with an empty Name falls back to showing its ID.
type ModelConfig struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Parameters override the provider-level generation values for this
	// model. Unset fields keep inheriting the provider values.
	Parameters *GenerationParameters `json:"parameters,omitempty"`
}

// GenerationParameters are optional per-request generation overrides. Nil
// fields mean "not configured": Neuropipe then omits them from the wire so
// endpoints keep their own defaults. Model entries override provider-level
// values field by field, so a provider can set a temperature baseline while
// one long-context model raises only its context size.
type GenerationParameters struct {
	// Temperature scales sampling randomness, typically 0..2.
	Temperature *float64 `json:"temperature,omitempty"`
	// TopK limits sampling to the K most likely tokens.
	TopK *int `json:"topK,omitempty"`
	// TopP enables nucleus sampling (0..1).
	TopP *float64 `json:"topP,omitempty"`
	// MaxTokens caps the generated completion (OpenAI max_tokens, Anthropic
	// max_tokens, Ollama num_predict) when no cap was discovered.
	MaxTokens *int `json:"maxTokens,omitempty"`
	// ContextSize widens the prompt window (Ollama num_ctx, the context the
	// managed llama.cpp server is launched with) for models whose window was
	// not discovered from the provider.
	ContextSize *int `json:"contextSize,omitempty"`
}

// EffectiveParameters resolves the generation parameters for one request to
// the given model: the model entry's overrides win over the provider-level
// values field by field. Matching is case-insensitive on the model key, and a
// nil model resolves to the provider-level values alone.
func (p ProviderConfig) EffectiveParameters(model string) GenerationParameters {
	result := GenerationParameters{}
	if p.Parameters != nil {
		result = *p.Parameters
	}
	model = strings.TrimSpace(model)
	var override *GenerationParameters
	for index := range p.Models {
		if strings.EqualFold(strings.TrimSpace(p.Models[index].ID), model) {
			override = p.Models[index].Parameters
			break
		}
	}
	if override == nil {
		return result
	}
	if override.Temperature != nil {
		result.Temperature = override.Temperature
	}
	if override.TopK != nil {
		result.TopK = override.TopK
	}
	if override.TopP != nil {
		result.TopP = override.TopP
	}
	if override.MaxTokens != nil {
		result.MaxTokens = override.MaxTokens
	}
	if override.ContextSize != nil {
		result.ContextSize = override.ContextSize
	}
	return result
}

type ProviderConfig struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Kind    ProviderKind `json:"kind"`
	BaseURL string       `json:"baseUrl"`
	// Model is the provider's default model: AI nodes without an explicit
	// model selection resolve to it at execution time.
	Model     string        `json:"model"`
	Models    []ModelConfig `json:"models,omitempty"`
	APIKeyRef string        `json:"apiKeyRef,omitempty"`
	Enabled   bool          `json:"enabled"`
	// Parameters are provider-level generation defaults applied to every
	// request unless the selected model overrides a field.
	Parameters *GenerationParameters `json:"parameters,omitempty"`
}

// ModelOptions returns the configured model list, ensuring the provider's
// default model is present so a saved default never disappears from pickers.
func (p ProviderConfig) ModelOptions() []ModelConfig {
	options := make([]ModelConfig, 0, len(p.Models)+1)
	seen := make(map[string]struct{}, len(p.Models)+1)
	if model := strings.TrimSpace(p.Model); model != "" {
		options = append(options, ModelConfig{ID: model})
		seen[model] = struct{}{}
	}
	for _, item := range p.Models {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		options = append(options, ModelConfig{ID: id, Name: strings.TrimSpace(item.Name), Parameters: item.Parameters})
		seen[id] = struct{}{}
	}
	return options
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
	HideToTrayOnClose bool   `json:"hideToTrayOnClose"`
	DefaultProviderID string `json:"defaultProviderId"`
	ContentDirectory  string `json:"contentDirectory"`
	RetentionDays     int    `json:"retentionDays"`
	// WebhookPort is retained to migrate pre-API settings. API.Port is the
	// active listener setting and is the only port used by new installations.
	WebhookPort     int              `json:"webhookPort"`
	PluginDirectory string           `json:"pluginDirectory"`
	Providers       []ProviderConfig `json:"providers"`
	// ManagedLlamaRemoved records the user's explicit removal of the managed
	// llama.cpp provider so the settings sync does not materialize it again
	// while local models exist. Adding the provider back clears it.
	ManagedLlamaRemoved  bool                 `json:"managedLlamaRemoved,omitempty"`
	MaxConcurrentRuns    int                  `json:"maxConcurrentRuns"`
	MaxConcurrentLLMRuns int                  `json:"maxConcurrentLLMRuns"`
	LlamaRuntime         LlamaRuntimeSettings `json:"llamaRuntime"`
	API                  APISettings          `json:"api"`
	Metrics              MetricsSettings      `json:"metrics"`
	Twitch               TwitchSettings       `json:"twitch"`
	Discord              DiscordSettings      `json:"discord"`
	Telegram             TelegramSettings     `json:"telegram"`
}

// TwitchSettings contains public configuration only. OAuth credentials remain
// in the DPAPI-backed vault and must never be returned through Wails.
type TwitchSettings struct {
	ClientID             string           `json:"clientId"`
	DefaultBotIdentityID string           `json:"defaultBotIdentityId,omitempty"`
	Identities           []TwitchIdentity `json:"identities"`
}

type TwitchIdentityStatus string

const (
	TwitchIdentityConnected         TwitchIdentityStatus = "connected"
	TwitchIdentityExpired           TwitchIdentityStatus = "expired"
	TwitchIdentityReconnectRequired TwitchIdentityStatus = "reconnect-required"
	TwitchIdentityRevoked           TwitchIdentityStatus = "revoked"
)

type TwitchConnectionMethod string

const (
	TwitchConnectionDeviceCode TwitchConnectionMethod = "device-code"
	TwitchConnectionManual     TwitchConnectionMethod = "manual"
)

// TwitchIdentity is safe to persist in settings and expose to the editor. It
// deliberately omits both access and refresh tokens and their vault keys.
type TwitchIdentity struct {
	ID        string                 `json:"id"`
	Label     string                 `json:"label"`
	UserID    string                 `json:"userId"`
	Login     string                 `json:"login"`
	Scopes    []string               `json:"scopes"`
	ExpiresAt *time.Time             `json:"expiresAt,omitempty"`
	Status    TwitchIdentityStatus   `json:"status"`
	Method    TwitchConnectionMethod `json:"method"`
}

// TwitchEventConditionField describes one EventSub condition without leaking
// OAuth implementation details into node packages or the renderer.
type TwitchEventConditionField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// TwitchEventDescriptor is the catalog contract shared by settings and the
// dynamic trigger node. EventType is a current EventSub subscription type.
type TwitchEventDescriptor struct {
	Type           string                      `json:"type"`
	Version        string                      `json:"version"`
	Label          string                      `json:"label"`
	Description    string                      `json:"description"`
	RequiredScopes []string                    `json:"requiredScopes"`
	Conditions     []TwitchEventConditionField `json:"conditions"`
	EventType      TypeSpec                    `json:"eventType"`
	ChatMessage    bool                        `json:"chatMessage"`
}

type TwitchStatus struct {
	Connected           bool   `json:"connected"`
	ConnectionState     string `json:"connectionState"`
	ActiveSubscriptions int    `json:"activeSubscriptions"`
	LastError           string `json:"lastError,omitempty"`
}

type TwitchDeviceAuthorizationRequest struct {
	// IdentityID reconnects an existing public identity with a new scope set.
	// An empty ID creates a new connected identity once authorization completes.
	IdentityID string   `json:"identityId,omitempty"`
	Label      string   `json:"label"`
	Scopes     []string `json:"scopes"`
}

// TwitchDeviceAuthorization is the short-lived public half of an OAuth
// device-code flow. The internal polling state and all tokens stay private.
type TwitchDeviceAuthorization struct {
	ID              string    `json:"id"`
	UserCode        string    `json:"userCode"`
	VerificationURI string    `json:"verificationUri"`
	ExpiresAt       time.Time `json:"expiresAt"`
	IntervalSeconds int       `json:"intervalSeconds"`
}

type TwitchManualIdentityRequest struct {
	Label       string `json:"label"`
	AccessToken string `json:"accessToken"`
}

type TwitchChatMessageRequest struct {
	IdentityID    string `json:"identityId,omitempty"`
	Channel       string `json:"channel"`
	Message       string `json:"message"`
	ReplyParentID string `json:"replyParentMessageId,omitempty"`
}

type TwitchChatMessageResult struct {
	MessageID string `json:"messageId,omitempty"`
	Sent      bool   `json:"sent"`
	Reason    string `json:"reason,omitempty"`
}

// DiscordSettings contains public configuration only. Bot tokens remain in the
// DPAPI-backed vault and must never be returned through Wails.
type DiscordSettings struct {
	DefaultBotIdentityID string            `json:"defaultBotIdentityId,omitempty"`
	Identities           []DiscordIdentity `json:"identities"`
}

type DiscordIdentityStatus string

const (
	DiscordIdentityConnected DiscordIdentityStatus = "connected"
	DiscordIdentityInvalid   DiscordIdentityStatus = "invalid"
	DiscordIdentityRevoked   DiscordIdentityStatus = "revoked"
)

// DiscordIdentity is safe to persist in settings and expose to the editor. It
// deliberately omits the bot token and its vault key.
type DiscordIdentity struct {
	ID        string                `json:"id"`
	Label     string                `json:"label"`
	BotUserID string                `json:"botUserId"`
	Username  string                `json:"username"`
	Status    DiscordIdentityStatus `json:"status"`
}

type DiscordStatus struct {
	Connected           bool   `json:"connected"`
	ConnectionState     string `json:"connectionState"`
	ActiveSubscriptions int    `json:"activeSubscriptions"`
	LastError           string `json:"lastError,omitempty"`
}

type DiscordManualIdentityRequest struct {
	Label string `json:"label"`
	Token string `json:"token"` // bot token; moved to the vault, never persisted here
}

// DiscordEventConditionField describes one client-side filter without leaking
// gateway implementation details into node packages or the renderer.
type DiscordEventConditionField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// DiscordEventDescriptor is the catalog contract shared by settings and the
// dynamic trigger node. Intents is the bitmask union required to receive the
// gateway event; Privileged marks descriptors whose intents need a Developer
// Portal toggle.
type DiscordEventDescriptor struct {
	Type         string                       `json:"type"`
	GatewayEvent string                       `json:"gatewayEvent"`
	Label        string                       `json:"label"`
	Description  string                       `json:"description"`
	Intents      int                          `json:"intents"`
	Privileged   bool                         `json:"privileged"`
	ChatMessage  bool                         `json:"chatMessage"`
	Conditions   []DiscordEventConditionField `json:"conditions"`
}

type DiscordMessageRequest struct {
	IdentityID  string              `json:"identityId,omitempty"`
	ChannelID   string              `json:"channelId"`
	Message     string              `json:"message"`
	ReplyToID   string              `json:"replyToMessageId,omitempty"`
	Embeds      []*DiscordEmbed     `json:"embeds,omitempty"`
	Attachments []DiscordAttachment `json:"attachments,omitempty"`
}

// DiscordEmbed mirrors Discord's embed object using the API's own snake_case
// wire format, so raw embed JSON pasted into the embedsJson pin parses without
// translation. It is converted to the transport library's type inside the
// Discord service; nodes never depend on discordgo.
type DiscordEmbed struct {
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	URL         string               `json:"url,omitempty"`
	Timestamp   string               `json:"timestamp,omitempty"`
	Color       int                  `json:"color,omitempty"`
	Footer      *DiscordEmbedFooter  `json:"footer,omitempty"`
	Image       *DiscordEmbedMedia   `json:"image,omitempty"`
	Thumbnail   *DiscordEmbedMedia   `json:"thumbnail,omitempty"`
	Author      *DiscordEmbedAuthor  `json:"author,omitempty"`
	Fields      []*DiscordEmbedField `json:"fields,omitempty"`
}

type DiscordEmbedFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type DiscordEmbedMedia struct {
	URL string `json:"url,omitempty"`
}

type DiscordEmbedAuthor struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordAttachment is one pre-loaded file payload attached to a message.
type DiscordAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
	Data        []byte `json:"-"`
}

type DiscordDMRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	UserID     string `json:"userId"`
	Message    string `json:"message"`
}

type DiscordReactionRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChannelID  string `json:"channelId"`
	MessageID  string `json:"messageId"`
	Emoji      string `json:"emoji"`
}

type DiscordEditRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChannelID  string `json:"channelId"`
	MessageID  string `json:"messageId"`
	Message    string `json:"message"`
}

type DiscordDeleteRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChannelID  string `json:"channelId"`
	MessageID  string `json:"messageId"`
}

type DiscordMessageResult struct {
	MessageID string `json:"messageId,omitempty"`
	Sent      bool   `json:"sent"`
	Reason    string `json:"reason,omitempty"`
}

/* ------------------------------------------------------------------ */
/* Discord application commands                                        */
/* ------------------------------------------------------------------ */

// Discord command types, mirroring Discord's application command type
// values: chat input (slash), user context menu, message context menu.
const (
	DiscordCommandChatInput int = 1
	DiscordCommandUser      int = 2
	DiscordCommandMessage   int = 3
)

// DiscordApplicationCommand mirrors Discord's application command object
// using the API's own snake_case wire format, so a command fetched from
// Discord round-trips through the editor without translation.
type DiscordApplicationCommand struct {
	ID                      string                            `json:"id,omitempty"`
	ApplicationID           string                            `json:"application_id,omitempty"`
	GuildID                 string                            `json:"guild_id,omitempty"`
	Version                 string                            `json:"version,omitempty"`
	Type                    int                               `json:"type,omitempty"`
	Name                    string                            `json:"name"`
	Description             string                            `json:"description,omitempty"`
	Options                 []DiscordApplicationCommandOption `json:"options,omitempty"`
	DefaultMemberPermission *int64                            `json:"default_member_permissions,string,omitempty"`
	DMPermission            *bool                             `json:"dm_permission,omitempty"`
	NSFW                    bool                              `json:"nsfw,omitempty"`
	Contexts                []int                             `json:"contexts,omitempty"`
}

// DiscordApplicationCommandOption mirrors one slash-command option.
// Type uses Discord's option type values: 1 subcommand, 2 subcommand
// group, 3 string, 4 integer, 5 boolean, 6 user, 7 channel, 8 role,
// 9 mentionable, 10 number, 11 attachment.
type DiscordApplicationCommandOption struct {
	Type         int                               `json:"type"`
	Name         string                            `json:"name"`
	Description  string                            `json:"description,omitempty"`
	Required     bool                              `json:"required,omitempty"`
	Choices      []DiscordApplicationCommandChoice `json:"choices,omitempty"`
	ChannelTypes []int                             `json:"channel_types,omitempty"`
	MinValue     *float64                          `json:"min_value,omitempty"`
	MaxValue     *float64                          `json:"max_value,omitempty"`
	MinLength    *int                              `json:"min_length,omitempty"`
	MaxLength    int                               `json:"max_length,omitempty"`
	Autocomplete bool                              `json:"autocomplete,omitempty"`
	Options      []DiscordApplicationCommandOption `json:"options,omitempty"`
}

// DiscordApplicationCommandChoice is one selectable value for STRING,
// INTEGER, and NUMBER options.
type DiscordApplicationCommandChoice struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// DiscordCommandRequest creates or updates one application command on a
// bot identity. GuildID selects the scope: empty means global.
type DiscordCommandRequest struct {
	IdentityID string                    `json:"identityId,omitempty"`
	GuildID    string                    `json:"guildId,omitempty"`
	Command    DiscordApplicationCommand `json:"command"`
}

// DiscordGuildLite is one guild of a bot, reduced to what the command
// scope picker needs.
type DiscordGuildLite struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// DiscordInteractionRef is the handoff object between the application
// command trigger and the reply nodes. The token lives in pipeline
// memory only, expires after 15 minutes, and never reaches settings or
// the renderer.
type DiscordInteractionRef struct {
	InteractionID string `json:"interactionId"`
	ApplicationID string `json:"applicationId"`
	Token         string `json:"token"`
	CommandName   string `json:"commandName,omitempty"`
	CommandID     string `json:"commandId,omitempty"`
	Deferred      bool   `json:"deferred"`
}

// DiscordCommandReplyRequest answers one interaction: manual mode sends
// the initial callback, deferred mode edits the loading placeholder.
type DiscordCommandReplyRequest struct {
	IdentityID  string                `json:"identityId,omitempty"`
	Interaction DiscordInteractionRef `json:"interaction"`
	Message     string                `json:"message"`
	Embeds      []*DiscordEmbed       `json:"embeds,omitempty"`
	Ephemeral   bool                  `json:"ephemeral,omitempty"`
}

// DiscordFollowupRequest sends one additional followup message while the
// interaction token is still valid.
type DiscordFollowupRequest struct {
	IdentityID  string                `json:"identityId,omitempty"`
	Interaction DiscordInteractionRef `json:"interaction"`
	Message     string                `json:"message"`
	Embeds      []*DiscordEmbed       `json:"embeds,omitempty"`
}

// DiscordCommandEditRequest edits the original interaction reply (empty
// MessageID) or a followup message by id.
type DiscordCommandEditRequest struct {
	IdentityID  string                `json:"identityId,omitempty"`
	Interaction DiscordInteractionRef `json:"interaction"`
	MessageID   string                `json:"messageId,omitempty"`
	Message     string                `json:"message"`
	Embeds      []*DiscordEmbed       `json:"embeds,omitempty"`
}

type DiscordActionResult struct {
	Done   bool   `json:"done"`
	Reason string `json:"reason,omitempty"`
}

// TelegramSettings contains public configuration only. Bot tokens remain in
// the DPAPI-backed vault and must never be returned through Wails.
type TelegramSettings struct {
	DefaultBotIdentityID string             `json:"defaultBotIdentityId,omitempty"`
	Identities           []TelegramIdentity `json:"identities"`
}

type TelegramIdentityStatus string

const (
	TelegramIdentityConnected TelegramIdentityStatus = "connected"
	TelegramIdentityInvalid   TelegramIdentityStatus = "invalid"
	TelegramIdentityRevoked   TelegramIdentityStatus = "revoked"
)

// TelegramIdentity is safe to persist in settings and expose to the editor. It
// deliberately omits the bot token and its vault key.
type TelegramIdentity struct {
	ID        string                 `json:"id"`
	Label     string                 `json:"label"`
	BotUserID string                 `json:"botUserId"`
	Username  string                 `json:"username"` // without @
	Status    TelegramIdentityStatus `json:"status"`
}

type TelegramStatus struct {
	Connected           bool   `json:"connected"`
	ConnectionState     string `json:"connectionState"`
	ActiveSubscriptions int    `json:"activeSubscriptions"`
	LastError           string `json:"lastError,omitempty"`
}

type TelegramManualIdentityRequest struct {
	Label string `json:"label"`
	Token string `json:"token"`
}

type TelegramEventConditionField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// TelegramEventDescriptor is the catalog contract shared by settings and the
// dynamic trigger node. Type is the Bot API update field name.
type TelegramEventDescriptor struct {
	Type        string                        `json:"type"`
	Label       string                        `json:"label"`
	Description string                        `json:"description"`
	ChatMessage bool                          `json:"chatMessage"`
	Callback    bool                          `json:"callback"`
	Conditions  []TelegramEventConditionField `json:"conditions"`
}

type TelegramMessageRequest struct {
	IdentityID          string `json:"identityId,omitempty"`
	ChatID              string `json:"chatId"`
	Message             string `json:"message"`
	ParseMode           string `json:"parseMode,omitempty"` // "" | HTML | MarkdownV2
	ReplyToMessageID    string `json:"replyToMessageId,omitempty"`
	DisableNotification bool   `json:"disableNotification,omitempty"`
}

// TelegramPhotoRequest sends one photo. Either PhotoURL is set (Telegram
// fetches it server-side) or Data carries the raw image bytes uploaded as
// multipart by the service.
type TelegramPhotoRequest struct {
	IdentityID  string `json:"identityId,omitempty"`
	ChatID      string `json:"chatId"`
	PhotoURL    string `json:"photoUrl"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Data        []byte `json:"-"`
	Caption     string `json:"caption,omitempty"`
	ParseMode   string `json:"parseMode,omitempty"`
}

// TelegramDocumentRequest sends one general file. Either DocumentURL is set
// (Telegram fetches it server-side) or Data carries the raw bytes uploaded as
// multipart by the service.
type TelegramDocumentRequest struct {
	IdentityID          string `json:"identityId,omitempty"`
	ChatID              string `json:"chatId"`
	DocumentURL         string `json:"documentUrl,omitempty"`
	FileName            string `json:"fileName,omitempty"`
	ContentType         string `json:"contentType,omitempty"`
	Data                []byte `json:"-"`
	Caption             string `json:"caption,omitempty"`
	ParseMode           string `json:"parseMode,omitempty"`
	ReplyToMessageID    string `json:"replyToMessageId,omitempty"`
	DisableNotification bool   `json:"disableNotification,omitempty"`
}

type TelegramEditRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChatID     string `json:"chatId"`
	MessageID  string `json:"messageId"`
	Message    string `json:"message"`
	ParseMode  string `json:"parseMode,omitempty"`
}

type TelegramDeleteRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChatID     string `json:"chatId"`
	MessageID  string `json:"messageId"`
}

type TelegramCallbackAnswerRequest struct {
	IdentityID      string `json:"identityId,omitempty"`
	CallbackQueryID string `json:"callbackQueryId"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"showAlert,omitempty"`
}

type TelegramChatActionRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChatID     string `json:"chatId"`
	Action     string `json:"action"`
}

type TelegramPinRequest struct {
	IdentityID string `json:"identityId,omitempty"`
	ChatID     string `json:"chatId"`
	MessageID  string `json:"messageId"`
	Notify     bool   `json:"notify,omitempty"`
	Unpin      bool   `json:"unpin,omitempty"`
}

type TelegramMessageResult struct {
	MessageID string `json:"messageId,omitempty"`
	Sent      bool   `json:"sent"`
	Reason    string `json:"reason,omitempty"`
}

type TelegramActionResult struct {
	Done   bool   `json:"done"`
	Reason string `json:"reason,omitempty"`
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

// LlamaRuntimeReleaseList is a release listing together with the source that
// served it. Source is "github-api" (live REST API), "github-web" (live
// releases page, used when the API is rate-limited or blocked), or "cache"
// (the last successful listing); Notice explains any fallback in a
// user-readable way and is empty for a plain API listing.
type LlamaRuntimeReleaseList struct {
	Releases  []LlamaRuntimeRelease `json:"releases"`
	Source    string                `json:"source"`
	FetchedAt time.Time             `json:"fetchedAt,omitempty"`
	Notice    string                `json:"notice,omitempty"`
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

// ExecutorLLMMode selects where AI nodes resolve their provider when running
// on a remote executor. ProxyForwards calls back through the desktop session;
// Local uses providers configured on the executor itself.
type ExecutorLLMMode string

const (
	ExecutorLLMProxy ExecutorLLMMode = "proxy"
	ExecutorLLMLocal ExecutorLLMMode = "local"
)

// RemoteExecutor is a user-registered remote pipeline executor. TokenRef only
// identifies a vault record; the shared secret is never serialised to React.
type RemoteExecutor struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Address   string          `json:"address"`
	TokenRef  string          `json:"-"`
	UseTLS    bool            `json:"useTLS"`
	LLMMode   ExecutorLLMMode `json:"llmMode"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// SaveRemoteExecutorRequest creates or updates an executor registration.
// A non-empty Token stores the shared secret once; it is never returned.
type SaveRemoteExecutorRequest struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Token   string `json:"token,omitempty"`
	UseTLS  bool   `json:"useTLS"`
}

// RemoteExecutorStatus is safe to render in Settings and never includes the
// token, address credentials, or connection internals beyond reachability.
type RemoteExecutorStatus struct {
	Online        bool   `json:"online"`
	Version       string `json:"version,omitempty"`
	Platform      string `json:"platform,omitempty"`
	Message       string `json:"message,omitempty"`
	ActiveRuns    int    `json:"activeRuns"`
	MaxConcurrent int    `json:"maxConcurrent"`
}

// RemoteExecutorProvider mirrors a provider configured on the executor for
// local LLM mode. The API key is write-only and surfaces as APIKeySet.
type RemoteExecutorProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	APIKeySet bool   `json:"apiKeySet"`
}

// RemoteExecutorConfig is the RPC-managed runtime configuration of one
// executor. Boot settings (listen address, TLS, auth token) stay in the
// executor's local config file by design.
type RemoteExecutorConfig struct {
	LLMMode           ExecutorLLMMode          `json:"llmMode"`
	Providers         []RemoteExecutorProvider `json:"providers"`
	DefaultProviderID string                   `json:"defaultProviderId"`
	MaxConcurrentRuns int                      `json:"maxConcurrentRuns"`
}

// RemoteExecutorListItem pairs a registration with its cached status.
type RemoteExecutorListItem struct {
	RemoteExecutor
	Status RemoteExecutorStatus `json:"status"`
}

// ExecutorCreateResult returns the registration plus the shared secret,
// which the UI displays exactly once.
type ExecutorCreateResult struct {
	Executor RemoteExecutor `json:"executor"`
	Token    string         `json:"token"`
}

/* ------------------------------------------------------------------ */
/* Storage (S3 / FTP) contracts                                        */
/* ------------------------------------------------------------------ */

// StorageDriver identifies a supported remote storage connection type.
type StorageDriver string

const (
	StorageDriverS3  StorageDriver = "s3"
	StorageDriverFTP StorageDriver = "ftp"
)

// StorageTLSMode selects how an FTP connection protects its traffic.
type StorageTLSMode string

const (
	StorageTLSNone     StorageTLSMode = "none"
	StorageTLSExplicit StorageTLSMode = "explicit"
	StorageTLSImplicit StorageTLSMode = "implicit"
)

// ValidStorageTLSMode reports whether the FTP TLS mode is recognised.
func ValidStorageTLSMode(mode StorageTLSMode) bool {
	switch mode {
	case StorageTLSNone, StorageTLSExplicit, StorageTLSImplicit:
		return true
	default:
		return false
	}
}

// Storage is one registered remote storage connection. S3 connections fill
// Endpoint/Region/Bucket/AccessKey/SecretRef; FTP connections fill Host/Port/
// Username/PasswordRef/TLSMode/BaseDir. Secrets never leave the local vault;
// only their opaque references are persisted. PublicBaseURL optionally maps
// the storage root to an HTTP address (CDN in front of a bucket, or a web
// server serving the FTP tree) so public URLs can be built without a network
// round-trip.
type Storage struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Driver StorageDriver `json:"driver"`
	// S3-compatible settings (driver "s3").
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretRef string `json:"secretRef,omitempty"`
	// Secure enables HTTPS for custom endpoints; the AWS endpoint is always
	// TLS. Local MinIO instances served over plain HTTP turn this off.
	Secure *bool `json:"secure,omitempty"`
	// FTP settings (driver "ftp").
	Host        string         `json:"host,omitempty"`
	Port        int            `json:"port,omitempty"`
	Username    string         `json:"username,omitempty"`
	PasswordRef string         `json:"passwordRef,omitempty"`
	TLSMode     StorageTLSMode `json:"tlsMode,omitempty"`
	BaseDir     string         `json:"baseDir,omitempty"`
	// PublicBaseURL is an optional HTTP(S) base that serves the storage root
	// (path "" as shown in the browser). Shared by both drivers.
	PublicBaseURL string `json:"publicBaseUrl,omitempty"`
	// Shared bookkeeping.
	Status     DatabaseStatus `json:"status"`
	LastPingAt *time.Time     `json:"lastPingAt,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// SaveStorageRequest carries editable storage metadata across Wails. Secret
// and Password are write-only (never returned in reads).
type SaveStorageRequest struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Driver        StorageDriver  `json:"driver"`
	Endpoint      string         `json:"endpoint,omitempty"`
	Region        string         `json:"region,omitempty"`
	Bucket        string         `json:"bucket,omitempty"`
	AccessKey     string         `json:"accessKey,omitempty"`
	SecretRef     string         `json:"secretRef,omitempty"`
	Secret        string         `json:"secret,omitempty"`
	Secure        *bool          `json:"secure,omitempty"`
	Host          string         `json:"host,omitempty"`
	Port          int            `json:"port,omitempty"`
	Username      string         `json:"username,omitempty"`
	PasswordRef   string         `json:"passwordRef,omitempty"`
	Password      string         `json:"password,omitempty"`
	TLSMode       StorageTLSMode `json:"tlsMode,omitempty"`
	BaseDir       string         `json:"baseDir,omitempty"`
	PublicBaseURL string         `json:"publicBaseUrl,omitempty"`
}

// StorageEntry is one browsable file or folder inside a storage. Path is the
// full remote path without a trailing slash; root listings use "".
type StorageEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// StorageListRequest lists the direct children of one remote directory.
// Empty Path lists the root. MaxEntries caps folder recursion when counting
// deletable objects (0 means unlimited).
type StorageListRequest struct {
	StorageID string `json:"storageId"`
	Path      string `json:"path"`
}

// StorageListResult is the folder-first listing of one remote directory.
type StorageListResult struct {
	Path    string         `json:"path"`
	Entries []StorageEntry `json:"entries"`
}

// StorageUploadFileRequest streams one local file into the storage. When
// RemotePath ends with "/" the original file name is appended.
type StorageUploadFileRequest struct {
	StorageID   string `json:"storageId"`
	LocalPath   string `json:"localPath"`
	RemotePath  string `json:"remotePath"`
	ContentType string `json:"contentType,omitempty"`
}

// StorageUploadDataRequest writes in-memory bytes (Draw Image output, HTTP
// downloads) into the storage.
type StorageUploadDataRequest struct {
	StorageID   string `json:"storageId"`
	Data        []byte `json:"data"`
	RemotePath  string `json:"remotePath"`
	ContentType string `json:"contentType,omitempty"`
}

// StorageUploadResult reports the stored key and the number of bytes written.
type StorageUploadResult struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	Driver string `json:"driver"`
}

// StorageDownloadRequest streams one remote file to a local path.
type StorageDownloadRequest struct {
	StorageID  string `json:"storageId"`
	RemotePath string `json:"remotePath"`
	LocalPath  string `json:"localPath"`
}

// StorageDownloadResult reports the local destination and byte count.
type StorageDownloadResult struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// StorageDeleteRequest removes one remote file, or one folder with everything
// inside it when Recursive is set.
type StorageDeleteRequest struct {
	StorageID string `json:"storageId"`
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

// StorageDeleteResult reports how many remote entries were removed.
type StorageDeleteResult struct {
	Deleted bool  `json:"deleted"`
	Count   int64 `json:"count"`
}

// StorageMakeDirRequest creates one remote folder (S3: zero-byte marker).
type StorageMakeDirRequest struct {
	StorageID string `json:"storageId"`
	Path      string `json:"path"`
}

// StorageMakeDirResult reports the created folder path.
type StorageMakeDirResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

// StorageMoveRequest renames or moves one remote file or folder (S3: copy
// plus delete; FTP: server-side rename).
type StorageMoveRequest struct {
	StorageID string `json:"storageId"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// StorageMoveResult reports the source and destination paths.
type StorageMoveResult struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Moved bool   `json:"moved"`
}

// StoragePresignRequest generates a temporary signed URL for one S3 object.
// Method is one of GET, PUT, HEAD, or DELETE. ExpiresSeconds uses the SigV4
// window 1..604800 (0 falls back to one hour). Headers are signed into the
// URL — whoever uses the URL must send exactly these headers. Params become
// signed query parameters (response-* overrides, versionId, …).
type StoragePresignRequest struct {
	StorageID      string            `json:"storageId"`
	Path           string            `json:"path"`
	Method         string            `json:"method"`
	ExpiresSeconds int64             `json:"expiresSeconds,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Params         map[string]string `json:"params,omitempty"`
}

// StoragePresignResult reports the signed URL and the constraints attached
// to it. Headers echoes the canonicalized header names and values the caller
// must send for the signature to validate.
type StoragePresignResult struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	ExpiresInSeconds int64             `json:"expiresInSeconds"`
	ExpiresAt        string            `json:"expiresAt"`
	Headers          map[string]string `json:"headers,omitempty"`
	Params           map[string]string `json:"params,omitempty"`
}

// StoragePublicURLRequest builds the public address of one remote file or
// folder. Construction is pure metadata — no network round-trip — so URLs can
// be produced for objects that are not uploaded yet.
type StoragePublicURLRequest struct {
	StorageID string `json:"storageId"`
	Path      string `json:"path"`
}

// StoragePublicURLResult reports the constructed URL and where it came from:
// "public-base" (the connection's public base URL), "s3" (the direct object
// address), or "ftp" (a best-effort protocol URL).
type StoragePublicURLResult struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
}
