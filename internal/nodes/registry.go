// Package nodes provides the composable registry used by first-party Blueprint
// nodes. It deliberately contains no Wails, persistence, or graph-engine
// dependency.
package nodes

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Node is the sole Blueprint node extension contract. A module owns its
// immutable metadata, configuration-dependent ports, and execution semantics;
// the graph host owns traversal, cancellation, and lifecycle state.
type Node interface {
	Definition() domain.NodeDefinition
	Resolve(domain.FlowNode) (domain.NodeDefinition, error)
	Execute(context.Context, Invocation, Runtime) (ExecutionResult, error)
}

// Registrar is the minimal extension surface given to modules at startup.
type Registrar interface{ Register(Node) error }

// Implementation is an embeddable implementation of Node. First-party node
// packages use it with their own metadata, resolver, and executor so every
// module has the same public shape without inheriting engine dependencies.
//
// The Execute function always returns ExecutionResult, regardless of whether
// the node is pure or impure. Pure nodes simply leave Ports and Loop empty.
type Implementation struct {
	Metadata domain.NodeDefinition
	Resolver func(domain.FlowNode) (domain.NodeDefinition, error)
	Executor func(context.Context, Invocation, Runtime) (ExecutionResult, error)
}

// Outputs adapts a value-producing operation to the uniform node executor.
// It is a convenience only; registry dispatch still has exactly one Node
// interface and one Execute method.
func Outputs(evaluate func(context.Context, Invocation, Runtime) (map[string]any, error)) func(context.Context, Invocation, Runtime) (ExecutionResult, error) {
	return func(ctx context.Context, invocation Invocation, runtime Runtime) (ExecutionResult, error) {
		outputs, err := evaluate(ctx, invocation, runtime)
		if err != nil {
			return ExecutionResult{}, err
		}
		return ExecutionResult{Outputs: outputs}, nil
	}
}

// Definition returns the immutable node metadata registered by the module.
func (implementation Implementation) Definition() domain.NodeDefinition {
	return cloneDefinition(implementation.Metadata)
}

// Resolve returns the module's dynamic port contract, or its static metadata.
func (implementation Implementation) Resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	if implementation.Resolver != nil {
		definition, err := implementation.Resolver(node)
		return cloneDefinition(definition), err
	}
	return cloneDefinition(implementation.Metadata), nil
}

// Execute invokes the module-owned operation.
func (implementation Implementation) Execute(ctx context.Context, invocation Invocation, runtime Runtime) (ExecutionResult, error) {
	if implementation.Executor == nil {
		return ExecutionResult{}, fmt.Errorf("node %q has no executor", implementation.Metadata.Type)
	}
	return implementation.Executor(ctx, invocation, runtime)
}

func cloneDefinition(definition domain.NodeDefinition) domain.NodeDefinition {
	definition.Inputs = clonePorts(definition.Inputs)
	definition.Outputs = clonePorts(definition.Outputs)
	definition.Fields = append([]domain.ConfigField(nil), definition.Fields...)
	for index := range definition.Fields {
		definition.Fields[index].Options = append([]domain.Option(nil), definition.Fields[index].Options...)
	}
	definition.Capabilities = append([]domain.Capability(nil), definition.Capabilities...)
	definition.DefaultConfig = cloneValues(definition.DefaultConfig)
	return definition
}

func clonePorts(ports []domain.NodePort) []domain.NodePort {
	cloned := append([]domain.NodePort(nil), ports...)
	for index := range cloned {
		cloned[index].Fields = append([]domain.DataField(nil), cloned[index].Fields...)
		cloned[index].Default = cloneValue(cloned[index].Default)
		if cloned[index].Type != nil {
			typeSpec := cloneTypeSpec(*cloned[index].Type)
			cloned[index].Type = &typeSpec
		}
	}
	return cloned
}

func cloneTypeSpec(typeSpec domain.TypeSpec) domain.TypeSpec {
	if typeSpec.Element != nil {
		element := cloneTypeSpec(*typeSpec.Element)
		typeSpec.Element = &element
	}
	if typeSpec.Key != nil {
		key := cloneTypeSpec(*typeSpec.Key)
		typeSpec.Key = &key
	}
	if typeSpec.Value != nil {
		value := cloneTypeSpec(*typeSpec.Value)
		typeSpec.Value = &value
	}
	typeSpec.Fields = append([]domain.TypeFieldSpec(nil), typeSpec.Fields...)
	for index := range typeSpec.Fields {
		typeSpec.Fields[index].Type = cloneTypeSpec(typeSpec.Fields[index].Type)
	}
	return typeSpec
}

func cloneValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneValues(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneValue(item)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

// Invocation is the stable data passed to executable node modules.
type Invocation struct {
	Node          domain.FlowNode
	Definition    domain.NodeDefinition
	SchemaVersion int
	ExecInput     string
	Config        map[string]any
	Inputs        map[string]any
	// ConnectedInputs distinguishes values supplied by graph edges from
	// inspector configuration copied into a matching input pin.
	ConnectedInputs map[string]bool
}

// ExecutionResult is the uniform output of every node execution. Pure nodes
// return only Outputs; impure nodes may additionally select ports or a loop.
type ExecutionResult struct {
	Outputs map[string]any
	Ports   []string
	Loop    *LoopPlan
}

// LoopPlan describes node-specific loop work while leaving traversal,
// cancellation, activation frames, and iteration limits to the graph host.
type LoopPlan struct {
	Iterations    []map[string]any
	Continue      func(map[string]any) (bool, error)
	ReportedCount int
}

// Runtime is a marker implemented by the graph host. Node modules must depend
// on one of the focused service interfaces below rather than a concrete
// interpreter, Wails service, or persistence implementation.
type Runtime interface{}

// VariableReader is the only host capability required by Get Variable.
type VariableReader interface {
	LookupVariable(name string) (any, bool)
}

// ChatHistoryReader is the only host capability required by Read Chat
// History. It keeps nodes independent from chat persistence.
type ChatHistoryReader interface {
	ReadChatHistory(context.Context, string, int) ([]domain.ChatMessage, error)
}

// OnceStore owns per-node Do Once state for the lifetime of an execution.
type OnceStore interface {
	ClaimOnce(nodeID string) bool
	ResetOnce(nodeID string)
}

// GateStore owns the mutable open state of Gate nodes.
type GateStore interface {
	GateOpen(nodeID string) (open bool, configured bool)
	SetGateOpen(nodeID string, open bool)
}

// FlipFlopStore records the next output state for a FlipFlop node.
type FlipFlopStore interface {
	NextFlipFlop(nodeID string) bool
}

// MultiGateStore records a MultiGate's next output index.
type MultiGateStore interface {
	MultiGateIndex(nodeID string) int
	SetMultiGateIndex(nodeID string, index int)
}

// LoopController exposes only Break's control-flow request, not graph
// traversal or frame internals.
type LoopController interface {
	InLoop() bool
	RequestBreak()
}

// VariableWriter is the narrow mutable-variable capability used by Set
// Variable. Variable reads remain a separate VariableReader abstraction.
type VariableWriter interface {
	StoreVariable(name string, value any)
}

// ReturnSignaler lets Return finish the current function or pipeline without
// exposing interpreter state to node modules.
type ReturnSignaler interface {
	Return(map[string]any)
}

// VariableStore is the per-execution mutable-variable contract used by the
// JavaScript host API. Values never leave the current Blueprint execution
// unless another node explicitly routes them onward.
type VariableStore interface {
	VariableReader
	VariableWriter
	DeleteVariable(name string)
}

// GlobalVariableReader is the narrow capability used by Get Global Variable.
// The host resolves names to workspace declarations and returns lived values.
// A read miss (variable deleted mid-run) yields false, never a synthetic stub.
type GlobalVariableReader interface {
	ReadGlobalVariable(name string) (any, bool)
}

// GlobalVariableWriter is the narrow mutable capability used by Set Global
// Variable. All operations are atomic under the host's lock: IncrementGlobal
// reads and adds in one step, AppendGlobal mutates a list in place, so two
// concurrent pipelines cannot lose an update.
type GlobalVariableWriter interface {
	WriteGlobalVariable(name string, value any) error
	IncrementGlobalVariable(name string, delta float64) (float64, error)
	AppendGlobalVariable(name string, item any) ([]any, error)
}

// SQLExecutorProvider supplies the registered-database boundary to SQL nodes.
type SQLExecutorProvider interface {
	SQLExecutor() SQLExecutor
}

// SQLExecutor is the only database operation available to Blueprint nodes.
type SQLExecutor interface {
	ExecuteSQL(context.Context, domain.SQLRequest) (domain.SQLResult, error)
}

// KVExecutorProvider supplies the registered KV boundary to KV nodes.
type KVExecutorProvider interface {
	KVExecutor() KVExecutor
}

// KVExecutor is the only key/value operation available to Blueprint nodes.
// It deliberately mirrors SQLExecutor's one-method shape so the graph host
// stays free of go-redis dependencies.
type KVExecutor interface {
	ExecuteCommand(context.Context, domain.KVCommandRequest) (domain.KVCommandResult, error)
}

// StorageExecutorProvider supplies the registered-storage boundary to
// storage nodes.
type StorageExecutorProvider interface {
	StorageExecutor() StorageExecutor
}

// StorageExecutor is the only remote-storage surface available to Blueprint
// nodes. Nodes never import S3 or FTP clients; they pass normalized domain
// requests and receive JSON-safe results.
type StorageExecutor interface {
	StorageListFiles(ctx context.Context, request domain.StorageListRequest) (domain.StorageListResult, error)
	StorageUploadFile(ctx context.Context, request domain.StorageUploadFileRequest) (domain.StorageUploadResult, error)
	StorageUploadData(ctx context.Context, request domain.StorageUploadDataRequest) (domain.StorageUploadResult, error)
	StorageDownloadFile(ctx context.Context, request domain.StorageDownloadRequest) (domain.StorageDownloadResult, error)
	StorageDelete(ctx context.Context, request domain.StorageDeleteRequest) (domain.StorageDeleteResult, error)
	StorageMakeDir(ctx context.Context, request domain.StorageMakeDirRequest) (domain.StorageMakeDirResult, error)
	StorageMove(ctx context.Context, request domain.StorageMoveRequest) (domain.StorageMoveResult, error)
	StoragePresignURL(ctx context.Context, request domain.StoragePresignRequest) (domain.StoragePresignResult, error)
	StoragePublicURL(ctx context.Context, request domain.StoragePublicURLRequest) (domain.StoragePublicURLResult, error)
}

// JavaScriptHostProvider supplies the deliberately narrow application services
// that a JavaScript node can reach through its np object. Node modules depend
// on this port rather than Wails, persistence, or a graph-engine concrete type.
type JavaScriptHostProvider interface {
	JavaScriptHost() JavaScriptHost
}

// JavaScriptHost is the application-facing boundary exposed to JavaScript.
// Every method returns plain domain data or bytes; the node module converts it
// to JavaScript values and never exposes Go service objects to a script.
type JavaScriptHost interface {
	ExecutionContext() JavaScriptExecutionContext
	ListPipelines(context.Context) ([]domain.PipelineSummary, error)
	GetPipeline(context.Context, string) (domain.Pipeline, error)
	ListFunctions(context.Context) ([]domain.FunctionSummary, error)
	ListTriggers(context.Context) ([]domain.TriggerBinding, error)
	ListExecutions(context.Context, int) ([]domain.Execution, error)
	ListReports(context.Context, int) ([]domain.Report, error)
	GetReport(context.Context, string) (domain.Report, error)
	CreateReport(context.Context, string, string, string, []string) (domain.Report, error)
	ReadChatHistory(context.Context, string, int) ([]domain.ChatMessage, error)
	AppendChatReply(context.Context, string, string) (domain.ChatMessage, error)
	UpdateChatStatus(context.Context, string, string) error
	ListDirectory(context.Context, string) ([]map[string]any, error)
	ReadFile(context.Context, string) ([]byte, error)
	WriteFile(context.Context, string, []byte) (string, error)
	HTTPRequest(context.Context, JavaScriptHTTPRequest) (JavaScriptHTTPResponse, error)
	Notify(context.Context, string, string) error
}

// JavaScriptExecutionContext identifies a running script without revealing its
// raw inputs, secrets, or host implementation.
type JavaScriptExecutionContext struct {
	PipelineID  string
	ExecutionID string
}

// JavaScriptHTTPRequest is the exact request accepted by np.http.request.
// The node validates JavaScript input before this boundary is crossed.
type JavaScriptHTTPRequest struct {
	URL     string
	Method  string
	Headers map[string][]string
	Body    []byte
}

// JavaScriptHTTPResponse is the safe, bounded result returned by
// np.http.request. Callers choose whether Body is represented as text or
// bytes; the host does not implicitly decode it.
type JavaScriptHTTPResponse struct {
	Status  int
	Headers map[string][]string
	Body    []byte
}

// TwitchChatSenderProvider is the narrow runtime port used by the Twitch Send
// Chat Message node. It prevents a node module from importing the EventSub
// service, HTTP client, vault, or Wails façade.
type TwitchChatSenderProvider interface {
	TwitchChatSender() TwitchChatSender
}

// DiscordSenderProvider is the narrow runtime port used by the Discord action
// nodes. It prevents a node module from importing the gateway service,
// discordgo, the vault, or the Wails façade.
type DiscordSenderProvider interface {
	DiscordSender() DiscordSender
}

// TelegramSenderProvider is the narrow runtime port used by the Telegram
// action nodes. It prevents a node module from importing the polling service,
// the HTTP client, the vault, or the Wails façade.
type TelegramSenderProvider interface {
	TelegramSender() TelegramSender
}

// DialogOpenerProvider is the runtime port consumed by Display Message and
// Display Question nodes. The host implements this to expose its dialog
// opener without giving node modules access to the desktop shell.
type DialogOpenerProvider interface {
	DialogOpener() DialogOpener
}

// InputDialogOpenerProvider is the runtime port consumed by the Display Input
// Dialog node. The host implements this to expose the styled input dialog
// opener to node modules.
type InputDialogOpenerProvider interface {
	InputDialogOpener() InputDialogOpener
}

// DialogOpener is the focused runtime port consumed by Display Message and
// Display Question nodes. It owns the small surface node modules need to show
// a native dialog without depending on Wails or the dialog implementation.
type DialogOpener interface {
	ShowMessage(ctx context.Context, title, message string) error
	ShowQuestion(ctx context.Context, title, message string) (DialogChoice, error)
}

// InputDialogOpener is the focused runtime port consumed by the Display Input
// Dialog node. It returns the typed user value or reports cancellation.
type InputDialogOpener interface {
	ShowInput(ctx context.Context, request InputRequest) (InputResponse, error)
}

// FormDialogOpenerProvider is the runtime port consumed by the Form node.
type FormDialogOpenerProvider interface {
	FormDialogOpener() FormDialogOpener
}

// FormDialogOpener blocks until the user submits or cancels the styled form.
type FormDialogOpener interface {
	ShowForm(ctx context.Context, request FormRequest) (FormResponse, error)
}

// FormRequest carries the data needed to render a styled form dialog.
type FormRequest struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Continue string         `json:"continueLabel"`
	Cancel   string         `json:"cancelLabel"`
	Items    []FormItemSpec `json:"items"`
}

// FormResponse is returned from a styled form dialog.
type FormResponse struct {
	Canceled bool           `json:"canceled"`
	Values   map[string]any `json:"values"`
}

// FormItemSpec is one field in the form, mirrored from the form node's layout.
type FormItemSpec struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind"`
	Label       string           `json:"label"`
	Col         int              `json:"col"`
	Row         int              `json:"row"`
	Span        int              `json:"span"`
	RowSpan     int              `json:"rowSpan"`
	InputType   string           `json:"inputType,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
	Options     []FormItemOption `json:"options,omitempty"`
}

// FormItemOption is one dropdown option.
type FormItemOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// DialogChoice reports which button the user pressed on a question dialog.
type DialogChoice string

const (
	DialogYes    DialogChoice = "yes"
	DialogNo     DialogChoice = "no"
	DialogCancel DialogChoice = "cancel"
)

// InputRequest carries the data needed to render a styled input dialog. It is
// mirrored on the dialog implementation; node modules construct one through
// this contract instead of importing the dialogs package directly.
type InputRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Label       string `json:"label"`
	InputType   string `json:"inputType"`
	Continue    string `json:"continueLabel"`
	Cancel      string `json:"cancelLabel"`
	Placeholder string `json:"placeholder"`
}

// InputResponse is returned from a styled input dialog.
type InputResponse struct {
	Canceled bool   `json:"canceled"`
	Value    string `json:"value"`
}

// TwitchChatSender accepts only the action's typed request and returns a
// non-secret delivery result. Identity selection and OAuth token handling are
// owned by infrastructure.
type TwitchChatSender interface {
	SendTwitchChatMessage(context.Context, domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error)
}

// DiscordSender accepts only the actions' typed requests and returns
// non-secret results. Identity selection and bot token handling are owned by
// infrastructure.
type DiscordSender interface {
	SendDiscordMessage(ctx context.Context, request domain.DiscordMessageRequest) (domain.DiscordMessageResult, error)
	SendDiscordDirectMessage(ctx context.Context, request domain.DiscordDMRequest) (domain.DiscordMessageResult, error)
	AddDiscordReaction(ctx context.Context, request domain.DiscordReactionRequest) (domain.DiscordActionResult, error)
	EditDiscordMessage(ctx context.Context, request domain.DiscordEditRequest) (domain.DiscordActionResult, error)
	DeleteDiscordMessage(ctx context.Context, request domain.DiscordDeleteRequest) (domain.DiscordActionResult, error)
	RespondDiscordInteraction(ctx context.Context, request domain.DiscordCommandReplyRequest) (domain.DiscordMessageResult, error)
	SendDiscordFollowup(ctx context.Context, request domain.DiscordFollowupRequest) (domain.DiscordMessageResult, error)
	EditDiscordInteractionMessage(ctx context.Context, request domain.DiscordCommandEditRequest) (domain.DiscordActionResult, error)
}

// TelegramSender accepts only the actions' typed requests and returns
// non-secret results. Identity selection and bot token handling are owned by
// infrastructure.
type TelegramSender interface {
	SendTelegramMessage(ctx context.Context, request domain.TelegramMessageRequest) (domain.TelegramMessageResult, error)
	SendTelegramPhoto(ctx context.Context, request domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error)
	SendTelegramDocument(ctx context.Context, request domain.TelegramDocumentRequest) (domain.TelegramMessageResult, error)
	EditTelegramMessage(ctx context.Context, request domain.TelegramEditRequest) (domain.TelegramActionResult, error)
	DeleteTelegramMessage(ctx context.Context, request domain.TelegramDeleteRequest) (domain.TelegramActionResult, error)
	AnswerTelegramCallbackQuery(ctx context.Context, request domain.TelegramCallbackAnswerRequest) (domain.TelegramActionResult, error)
	SendTelegramChatAction(ctx context.Context, request domain.TelegramChatActionRequest) (domain.TelegramActionResult, error)
	PinTelegramMessage(ctx context.Context, request domain.TelegramPinRequest) (domain.TelegramActionResult, error)
}

// Registry checks node IDs and provides deterministic access to first-party
// module implementations.
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]Node
}

// New creates an empty module registry.
func New() *Registry { return &Registry{nodes: make(map[string]Node)} }

// Register adds one complete node implementation to the registry.
func (r *Registry) Register(node Node) error {
	if node == nil {
		return fmt.Errorf("cannot register a nil node")
	}
	definition := node.Definition()
	if definition.Type == "" {
		return fmt.Errorf("node definition needs a type")
	}
	if definition.Mode != domain.NodePure && definition.Mode != domain.NodeImpure && definition.Mode != domain.NodeEvent {
		return fmt.Errorf("node %q has unsupported executable mode %q", definition.Type, definition.Mode)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.nodes[definition.Type]; exists {
		return fmt.Errorf("node type %q is registered more than once", definition.Type)
	}
	r.nodes[definition.Type] = node
	return nil
}

// Get returns the complete module implementation for a node type.
func (r *Registry) Get(nodeType string) (Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	node, ok := r.nodes[nodeType]
	return node, ok
}

// All returns all implementations in stable type order.
func (r *Registry) All() []Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		result = append(result, node)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Definition().Type < result[j].Definition().Type
	})
	return result
}
