// Package pipeline validates and executes published flow definitions.
package pipeline

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// Packet is the JSON data unit routed through a pipeline execution.
type Packet map[string]any

// Result sends zero or more packets through each named output port.
type Result map[string][]Packet

// Executor implements one node type without depending on Wails or persistence.
type Executor interface {
	Execute(ctx context.Context, config map[string]any, input Packet) (Result, error)
}

// ChatRequest is the provider-neutral LLM input used by AI nodes.
type ChatRequest struct {
	Prompt      string
	Model       string
	Schema      map[string]any
	ToolName    string
	ToolChoices []string
	Metrics     domain.LLMMetricContext
	// ToolChoiceDescriptions provides model-facing guidance keyed by a stable
	// choice ID. It is only used to build the provider prompt; the ID remains
	// the constrained value returned by the model.
	ToolChoiceDescriptions map[string]string
}

// ChatResponse is intentionally small so Ollama and OpenAI-compatible APIs can share it.
type ChatResponse struct {
	Content string
	JSON    map[string]any
	Usage   domain.LLMUsage
}

// LLMRunner lets the engine invoke an app-configured provider without importing it.
type LLMRunner interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
}

// AssistantRunner is the optional native-tool extension to LLMRunner. The
// graph host only needs this narrow conversation contract when an Agent has
// one or more LLM tool functions connected to its Tools pin.
type AssistantRunner interface {
	Converse(ctx context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error)
}

// NotificationSender delivers an operating-system notification without
// coupling pipeline execution to Wails or a particular platform API.
type NotificationSender interface {
	Send(ctx context.Context, title, message string) error
}

// ChatWriter persists visible pipeline chat output without coupling Blueprint
// execution to SQLite or the Wails renderer.
type ChatWriter interface {
	AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error)
	UpdateChatStatus(ctx context.Context, chatRunID, statusText string) error
	ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error)
}

// ReportWriter persists Markdown reports without coupling graph execution to
// the SQLite implementation used by the desktop application.
type ReportWriter interface {
	CreateReport(ctx context.Context, report domain.Report) (domain.Report, error)
}

// ReportContext identifies the execution currently allowed to create reports.
type ReportContext struct {
	PipelineID  string
	ExecutionID string
}

// FunctionResolver supplies the latest published global function at execution
// time. It keeps the interpreter independent from SQLite and Wails.
type FunctionResolver interface {
	GetPublishedFunction(ctx context.Context, id string) (domain.CustomFunction, error)
}

// EngineOption extends an engine with an optional infrastructure capability.
type EngineOption func(*Engine)

// WithReportWriter allows report nodes to persist output for one execution.
func WithReportWriter(writer ReportWriter, context ReportContext) EngineOption {
	return func(engine *Engine) {
		engine.reports = writer
		engine.reportContext = context
	}
}

// WithFunctionResolver enables execution of dynamically registered function
// call nodes.
func WithFunctionResolver(resolver FunctionResolver) EngineOption {
	return func(engine *Engine) { engine.functions = resolver }
}

// WithNotificationSender enables the Desktop Notification action for an
// engine. A nil sender keeps the engine usable in headless tests and tools.
func WithNotificationSender(sender NotificationSender) EngineOption {
	return func(engine *Engine) { engine.notifications = sender }
}

// WithChatWriter enables Chat-specific Blueprint nodes for an execution.
func WithChatWriter(writer ChatWriter) EngineOption {
	return func(engine *Engine) { engine.chat = writer }
}

// WithJavaScriptHost makes the explicit np host API available to JavaScript
// nodes for one engine execution. A nil host keeps the engine useful in
// headless tests and makes JavaScript system calls fail explicitly.
func WithJavaScriptHost(host nodes.JavaScriptHost) EngineOption {
	return func(engine *Engine) { engine.javascript = host }
}

// WithTwitchChatSender supplies the sole outbound Twitch capability required
// by the Twitch node module. OAuth, EventSub, and persistence remain outside
// the graph engine.
func WithTwitchChatSender(sender nodes.TwitchChatSender) EngineOption {
	return func(engine *Engine) { engine.twitch = sender }
}

// WithGlobalVariablesStore makes workspace-scoped global variables available to
// running graphs. A nil store keeps the engine usable in headless tests and
// turns reads/writes into clear node errors.
func WithGlobalVariablesStore(store GlobalVariablesStore) EngineOption {
	return func(engine *Engine) { engine.globals = store }
}

// WithSQLExecutor enables registered SQLite database access for SQL nodes.
func WithSQLExecutor(executor nodes.SQLExecutor) EngineOption {
	return func(engine *Engine) { engine.databases = executor }
}

// WithDialogOpener enables Display Message and Display Question nodes by
// exposing a focused native-dialog opener to running graphs. A nil opener
// keeps the engine usable in headless tests and turns dialog calls into
// explicit node errors.
func WithDialogOpener(opener nodes.DialogOpener) EngineOption {
	return func(engine *Engine) { engine.dialogs = opener }
}

// WithInputDialogOpener enables the Display Input Dialog node by exposing the
// styled input-dialog opener to running graphs. A nil opener keeps the engine
// usable in headless tests and turns input-dialog calls into explicit errors.
func WithInputDialogOpener(opener nodes.InputDialogOpener) EngineOption {
	return func(engine *Engine) { engine.inputDialogs = opener }
}

// WithFormDialogOpener enables the Form node by exposing the styled form
// dialog opener to running graphs. A nil opener keeps the engine usable in
// headless tests and turns form-dialog calls into explicit errors.
func WithFormDialogOpener(opener nodes.FormDialogOpener) EngineOption {
	return func(engine *Engine) { engine.formDialogs = opener }
}

// ValidationError explains why a definition cannot be published.
type ValidationError struct {
	Message string `json:"message"`
}

func (e ValidationError) Error() string { return e.Message }

// RunResult is the in-memory execution result returned to the execution service.
type RunResult struct {
	NodeRuns []domain.NodeRun
	Returned bool
	Value    Packet
}
