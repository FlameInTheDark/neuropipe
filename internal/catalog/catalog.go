// Package catalog provides the single source of truth for built-in node metadata.
package catalog

import (
	"sort"
	"strings"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/builtin"
)

// Registry keeps node metadata shared by graph validation and the React palette.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]domain.NodeDefinition
	builtins    map[string]struct{}
	modules     *nodes.Registry
	// variableOptions feeds the picklist on Global Variable nodes. It is wired
	// once during Desktop composition before the catalog is used.
	variableOptions func() []domain.Option
}

// SetVariableOptions wires the global variable catalogue into the registry so
// dynamic picklist options can be injected on Global Variable nodes at All().
func (r *Registry) SetVariableOptions(source func() []domain.Option) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.variableOptions = source
}

// New creates a registry containing Neuropipe's built-in node catalog.
func New() *Registry {
	definitions := make(map[string]domain.NodeDefinition)
	builtinTypes := make(map[string]struct{})
	modules := nodes.New()
	if err := builtin.RegisterAll(modules); err != nil {
		panic("register built-in Blueprint nodes: " + err.Error())
	}
	for _, definition := range builtins() {
		definitions[definition.Type] = normalizeDefinition(definition)
		builtinTypes[definition.Type] = struct{}{}
	}
	for _, module := range modules.All() {
		definition := normalizeDefinition(module.Definition())
		if _, exists := definitions[definition.Type]; exists {
			panic("register built-in Blueprint nodes: duplicate " + definition.Type)
		}
		definitions[definition.Type] = definition
		builtinTypes[definition.Type] = struct{}{}
	}
	return &Registry{definitions: definitions, builtins: builtinTypes, modules: modules}
}

// Get returns a node definition by its stable type identifier.
func (r *Registry) Get(nodeType string) (domain.NodeDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definition, ok := r.definitions[nodeType]
	return definition, ok
}

// Node returns the behavior registered by a first-party node module. Plugin
// and custom-function definitions deliberately have no host implementation.
func (r *Registry) Node(nodeType string) (nodes.Node, bool) {
	if r.modules == nil {
		return nil, false
	}
	return r.modules.Get(nodeType)
}

// All returns the current catalog in display order.
func (r *Registry) All() []domain.NodeDefinition {
	r.mu.RLock()
	options := r.variableOptions
	definitions := make([]domain.NodeDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definitions = append(definitions, definition)
	}
	r.mu.RUnlock()
	if options != nil {
		list := options()
		for index, definition := range definitions {
			if definition.Type == "data:get_global_variable" || definition.Type == "flow:set_global_variable" {
				definitions[index] = injectVariableOptions(definition, list)
			}
		}
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Category == definitions[j].Category {
			return definitions[i].Label < definitions[j].Label
		}
		return definitions[i].Category < definitions[j].Category
	})
	return definitions
}

// injectVariableOptions replaces the select field options for the variable
// name field on Global Variable nodes. It clones the fields before mutation so
// the caller's definition value is never altered in place.
func injectVariableOptions(definition domain.NodeDefinition, options []domain.Option) domain.NodeDefinition {
	definition.Fields = append([]domain.ConfigField(nil), definition.Fields...)
	for index, field := range definition.Fields {
		if field.Name == "name" && field.Kind == "select" {
			field.Options = options
			definition.Fields[index] = field
		}
	}
	return definition
}

// Register adds a plugin definition after its bundle has been validated.
func (r *Registry) Register(definition domain.NodeDefinition) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[definition.Type]; exists {
		return false
	}
	r.definitions[definition.Type] = normalizeDefinition(definition)
	return true
}

// ReplaceDynamic replaces function-derived definitions without allowing them to
// overwrite first-party node identifiers.
func (r *Registry) ReplaceDynamic(definitions []domain.NodeDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for nodeType := range r.definitions {
		if _, builtin := r.builtins[nodeType]; !builtin {
			delete(r.definitions, nodeType)
		}
	}
	for _, definition := range definitions {
		if _, builtin := r.builtins[definition.Type]; builtin {
			continue
		}
		r.definitions[definition.Type] = normalizeDefinition(definition)
	}
}

func builtins() []domain.NodeDefinition {
	triggerOutput := []domain.NodePort{port("out", "Start", "flow", "#fafafa")}
	buttonTriggerOutput := []domain.NodePort{
		port("out", "Start", "flow", "#fafafa"),
		{
			ID:        "payload",
			Label:     "Payload",
			Kind:      domain.PinData,
			Direction: domain.PinOutput,
			DataType:  domain.DataObject,
			Type: &domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
				{ID: "trigger", Name: "trigger", Type: domain.TypeSpec{Kind: domain.TypeString}},
			}},
			Fields:         []domain.DataField{{Path: "trigger", DataType: domain.DataText}},
			Color:          "#60a5fa",
			MaxConnections: 1,
		},
	}
	chatTriggerOutput := []domain.NodePort{
		port("out", "Start", "flow", "#fafafa"),
		{ID: "text", Label: "Text", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
		{ID: "chatId", Label: "Chat ID", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
		{ID: "chatRunId", Label: "Chat Run ID", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
	}
	flowInput := []domain.NodePort{port("in", "Input", "flow", "#a1a1aa")}
	flowOutput := []domain.NodePort{port("out", "Output", "flow", "#fafafa")}
	llmToolInput := domain.NodePort{ID: "tools", Label: "Tools", Kind: domain.PinTool, Direction: domain.PinInput, Color: "#a78bfa"}
	httpInputs := append(append([]domain.NodePort{}, flowInput...), domain.NodePort{ID: "headers", Label: "Headers", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataObject,
		Type: &domain.TypeSpec{Kind: domain.TypeMap, Key: &domain.TypeSpec{Kind: domain.TypeString}, Value: &domain.TypeSpec{Kind: domain.TypeString}}, Color: "#e879f9", MaxConnections: 1,
		IgnoreConfigFallback: true})
	definitions := []domain.NodeDefinition{
		node("trigger:button", "Triggers", "Button Trigger", "Launch this published pipeline from the Trigger board.", "play", "#fafafa", nil, buttonTriggerOutput,
			[]domain.ConfigField{field("label", "Button label", "string", "Daily briefing", true), field("hotkey", "Global hotkey", "string", "Ctrl+Alt+B", false)}, map[string]any{"label": "Run pipeline", "icon": "play", "color": "#fafafa", "gridPosition": 0}),
		node("trigger:cron", "Triggers", "Cron Trigger", "Run on a five-field cron schedule in an IANA timezone.", "clock-3", "#a1a1aa", nil, triggerOutput,
			[]domain.ConfigField{field("cron", "Cron expression", "string", "0 9 * * 1-5", true), field("timezone", "Timezone", "string", "Local", false)}, map[string]any{"cron": "0 9 * * 1-5", "timezone": "Local"}),
		node("trigger:file_watch", "Triggers", "File Watch", "Start when a file or folder changes inside an approved root.", "folder-search", "#a1a1aa", nil, triggerOutput,
			[]domain.ConfigField{field("path", "Path", "string", "C:\\Work\\Inbox", true)}, map[string]any{}),
		node("trigger:hotkey", "Triggers", "Global Hotkey", "Start when a global keyboard shortcut is pressed.", "keyboard", "#a1a1aa", nil, triggerOutput,
			[]domain.ConfigField{field("hotkey", "Shortcut", "string", "Ctrl+Alt+N", true)}, map[string]any{}),
		node("trigger:webhook", "Triggers", "Local Webhook", "Receive a signed request on Neuropipe's loopback webhook server.", "webhook", "#a1a1aa", nil, triggerOutput,
			[]domain.ConfigField{field("path", "Path", "string", "/inbox", true), field("secret", "Signing secret", "secret", "Select a secret", true)}, map[string]any{}),
		node("trigger:chat", "Triggers", "Chat Trigger", "Start this published pipeline from a local chat conversation.", "message-circle", "#a78bfa", nil, chatTriggerOutput,
			[]domain.ConfigField{field("label", "Chat label", "string", "Assistant", true)}, map[string]any{"label": "Chat"}),

		node("action:http", "Actions", "HTTP Request", "Call an HTTP endpoint and pass its JSON or text response downstream.", "globe", "#60a5fa",
			httpInputs,
			append(flowOutput, httpResultOutput()),
			[]domain.ConfigField{
				field("url", "URL", "string", "https://api.example.com", true),
				selectField("method", "Method", []string{"GET", "POST", "PUT", "PATCH", "DELETE"}),
				field("body", "Body", "textarea", "", false),
				boolField("headersFromPin", "Take headers from Headers pin", false),
				visibleWhen(httpHeadersField(), "!headersFromPin"),
				customUserAgentToggleField(),
				visibleWhen(field("userAgent", "User-Agent", "http-user-agent", "Neuropipe/0.1", true), "useCustomUserAgent"),
				field("stripScripts", "Remove scripts", "boolean", "", false),
				field("stripStyles", "Remove styles", "boolean", "", false),
			}, map[string]any{"method": "GET", "headers": []any{}, "headersFromPin": false, "useCustomUserAgent": false, "userAgent": "", "stripScripts": false, "stripStyles": false}, domain.CapabilityNetwork),
		node("action:terminal", "Local", "Run Terminal Command", "Run PowerShell, Windows PowerShell, or cmd in an approved workspace.", "terminal", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{selectField("shell", "Shell", []string{"PowerShell", "Windows PowerShell", "cmd"}), field("command", "Command", "textarea", "Get-Date", true), field("workingDirectory", "Working directory", "string", "C:\\Work", false)}, map[string]any{"shell": "PowerShell"}, domain.CapabilityTerminal),
		node("action:notification", "Local", "Desktop Notification", "Show a Windows toast-style desktop notification.", "bell", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{field("title", "Title", "string", "Neuropipe", true), field("message", "Message", "textarea", "Done", true)}, map[string]any{}),
		node("action:report", "Actions", "Create Report", "Save rendered Markdown to the local Reports feed.", "notebook-pen", "#60a5fa", flowInput, flowOutput,
			[]domain.ConfigField{field("title", "Report title", "string", "Pipeline report", true), field("tags", "Tags", "tags", "Daily, Operations", false), field("markdown", "Markdown", "textarea", "# Report", true)}, map[string]any{}),
		node("action:git", "Local", "Git", "Run a focused local Git operation in an approved repository.", "git-branch", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{selectField("operation", "Operation", []string{"status", "diff", "log", "fetch", "pull"}), field("repository", "Repository", "string", "C:\\Work\\repo", true)}, map[string]any{"operation": "status"}, domain.CapabilityGit),
		pipelinesListNode(),

		node("llm:prompt", "AI", "LLM Prompt", "Generate text with the selected provider and model.", "sparkles", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("prompt", "Prompt", "textarea", "Summarise the connected input.", true), llmProviderField(), llmModelField(), chatStatusToggleField(), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "updateChatStatus": false, "chatRunId": ""}),
		node("llm:extract", "AI", "Structured Extract", "Extract schema-shaped JSON from the current packet.", "braces", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("prompt", "Instructions", "textarea", "Extract the requested fields.", true), llmProviderField(), llmModelField(), schemaField("schema", "Fields to extract"), chatStatusToggleField(), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "schema": objectSchema(), "updateChatStatus": false, "chatRunId": ""}),
		node("llm:boolean", "AI", "LLM Boolean Router", "Call a constrained route function and emit exactly one decision branch.", "circle-help", "#f472b6", flowInput, []domain.NodePort{port("true", "True", "flow", "#34d399"), port("false", "False", "flow", "#f87171"), port("error", "Error", "flow", "#fb7185")},
			[]domain.ConfigField{field("prompt", "Question", "textarea", "Is the connected input ready?", true), llmProviderField(), llmModelField(), chatStatusToggleField(), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "updateChatStatus": false, "chatRunId": ""}),
		node("llm:choice", "AI", "LLM Choice Router", "Choose one configured option through constrained structured output.", "list-checks", "#f472b6", flowInput, []domain.NodePort{port("error", "Error", "flow", "#fb7185")},
			[]domain.ConfigField{field("prompt", "Question", "textarea", "Choose the best option.", true), llmProviderField(), llmModelField(), routeOptionsField("options", "Options"), chatStatusToggleField(), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "options": routeOptions("option-a", "Option A", "option-b", "Option B"), "updateChatStatus": false, "chatRunId": ""}),
		node("llm:summarize", "AI", "Summarize", "Create a concise summary of input data.", "align-left", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("instructions", "Instructions", "textarea", "Summarise the input for a busy reader.", true), llmProviderField(), llmModelField(), chatStatusToggleField(), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "updateChatStatus": false, "chatRunId": ""}),
		node("llm:agent", "AI", "Agent", "A tool-using agent with only explicitly connected tools.", "bot", "#f472b6", append(append([]domain.NodePort{}, flowInput...), llmToolInput), flowOutput,
			[]domain.ConfigField{field("instructions", "Instructions", "textarea", "Complete the task using the connected tools.", true), llmProviderField(), llmModelField(), chatModeField(), field("maxTurns", "Maximum turns", "number", "8", true), unlimitedTurnsToggleField(), chatStatusToggleField(), visibleWhen(field("chatId", "Chat ID", "string", "", false), "chatMode"), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "maxTurns": 8.0, "chatMode": "message", "chatId": "", "unlimitedTurns": false, "updateChatStatus": false, "chatRunId": ""}),
		node("llm:coding_agent", "AI", "Coding Agent", "An agent preset for scoped file, Git, and terminal workspaces.", "code-2", "#f472b6", append(append([]domain.NodePort{}, flowInput...), llmToolInput), flowOutput,
			[]domain.ConfigField{field("task", "Task", "textarea", "", true), field("workspace", "Workspace", "string", "C:\\Work\\repo", true), llmProviderField(), llmModelField(), chatModeField(), field("maxTurns", "Maximum turns", "number", "12", true), unlimitedTurnsToggleField(), chatStatusToggleField(), visibleWhen(field("chatId", "Chat ID", "string", "", false), "chatMode"), visibleWhen(field("chatRunId", "Chat Run ID", "string", "", false), "updateChatStatus")}, map[string]any{"providerId": "", "model": "", "maxTurns": 12.0, "chatMode": "message", "chatId": "", "unlimitedTurns": false, "updateChatStatus": false, "chatRunId": ""}, domain.CapabilityFileRead, domain.CapabilityFileWrite, domain.CapabilityTerminal, domain.CapabilityGit),
	}
	return append(definitions, blueprintBuiltins()...)
}

func node(nodeType, category, label, description, icon, color string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any, capabilities ...domain.Capability) domain.NodeDefinition {
	return normalizeDefinition(domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: color, Inputs: inputs, Outputs: outputs, Fields: fields, Capabilities: capabilities, DefaultConfig: defaults, TriggerKind: catalogTriggerKind(nodeType), Source: "builtin"})
}

// catalogTriggerKind annotates the pre-module catalog entries. Publishing
// consumes NodeDefinition.TriggerKind, so no application service duplicates
// this list and newly modular triggers declare it themselves.
func catalogTriggerKind(nodeType string) domain.TriggerKind {
	switch nodeType {
	case "trigger:button":
		return domain.TriggerButton
	case "trigger:cron":
		return domain.TriggerCron
	case "trigger:file_watch":
		return domain.TriggerFile
	case "trigger:hotkey":
		return domain.TriggerHotkey
	case "trigger:webhook":
		return domain.TriggerHook
	case "trigger:chat":
		return domain.TriggerChat
	default:
		return ""
	}
}

func normalizeDefinition(definition domain.NodeDefinition) domain.NodeDefinition {
	if definition.Inputs == nil {
		definition.Inputs = make([]domain.NodePort, 0)
	}
	if definition.Outputs == nil {
		definition.Outputs = make([]domain.NodePort, 0)
	}
	if definition.Fields == nil {
		definition.Fields = make([]domain.ConfigField, 0)
	}
	if definition.Capabilities == nil {
		definition.Capabilities = make([]domain.Capability, 0)
	}
	if definition.DefaultConfig == nil {
		definition.DefaultConfig = make(map[string]any)
	}
	definition.Mode = inferMode(definition)
	for index := range definition.Inputs {
		definition.Inputs[index].Direction = domain.PinInput
		normalizePin(&definition.Inputs[index])
	}
	for index := range definition.Outputs {
		definition.Outputs[index].Direction = domain.PinOutput
		normalizePin(&definition.Outputs[index])
	}
	definition = addFieldPins(definition)
	return addKnownOutputShape(definition)
}

func port(id, label, kind, color string) domain.NodePort {
	pinKind := domain.PinData
	if kind == "flow" {
		pinKind = domain.PinExec
	}
	if kind == "tool" {
		pinKind = domain.PinTool
	}
	return domain.NodePort{ID: id, Label: label, Kind: pinKind, Color: color}
}

func normalizePin(pin *domain.NodePort) {
	if pin.Kind == "" {
		pin.Kind = domain.PinData
	}
	if pin.Kind == domain.PinData && pin.DataType == "" {
		pin.DataType = domain.DataAny
	}
	if pin.Kind == domain.PinData {
		if pin.Type == nil {
			pin.Type = typeSpecForDataType(pin.DataType)
		}
		pin.Color = dataColor(pin.DataType)
	}
	if pin.Kind != domain.PinTool && pin.MaxConnections == 0 {
		pin.MaxConnections = 1
	}
}

func inferMode(definition domain.NodeDefinition) domain.NodeExecutionMode {
	if definition.Mode != "" {
		return definition.Mode
	}
	if strings.HasPrefix(definition.Type, "trigger:") {
		return domain.NodeEvent
	}
	return domain.NodeImpure
}

func addFieldPins(definition domain.NodeDefinition) domain.NodeDefinition {
	if definition.PortContractOwned {
		return definition
	}
	if definition.Mode == domain.NodeVisual {
		return definition
	}
	seen := make(map[string]struct{}, len(definition.Inputs))
	for _, pin := range definition.Inputs {
		seen[pin.ID] = struct{}{}
	}
	for _, field := range definition.Fields {
		if isConfigurationOnlyField(field.Kind) {
			continue
		}
		if _, exists := seen[field.Name]; exists {
			continue
		}
		defaultValue, hasDefault := definition.DefaultConfig[field.Name]
		dataType := fieldDataType(field.Kind)
		if definition.Type == "data:constant" && field.Name == "value" {
			dataType = domain.DataAny
		}
		pin := domain.NodePort{ID: field.Name, Label: field.Label, Kind: domain.PinData, Direction: domain.PinInput, DataType: dataType, Type: typeSpecForDataType(dataType), Required: field.Required, MaxConnections: 1}
		if hasDefault {
			pin.Default = defaultValue
		}
		definition.Inputs = append(definition.Inputs, pin)
	}
	if definition.Mode == domain.NodeEvent && definition.Type != "trigger:chat" && !hasOutputPin(definition.Outputs, "payload", domain.PinData) {
		definition.Outputs = append(definition.Outputs, domain.NodePort{ID: "payload", Label: "Payload", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: typeSpecForDataType(domain.DataObject), Color: "#60a5fa"})
	}
	if definition.Mode == domain.NodeImpure {
		found := false
		for _, pin := range definition.Outputs {
			if pin.ID == "result" && pin.Kind == domain.PinData {
				found = true
			}
		}
		if !found {
			definition.Outputs = append(definition.Outputs, domain.NodePort{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: typeSpecForDataType(domain.DataObject), Color: "#60a5fa"})
		}
	}
	return definition
}

func hasOutputPin(outputs []domain.NodePort, id string, kind domain.PinKind) bool {
	for _, output := range outputs {
		if output.ID == id && output.Kind == kind {
			return true
		}
	}
	return false
}

func fieldDataType(kind string) domain.DataType {
	switch kind {
	case "number":
		return domain.DataNumber
	case "json":
		return domain.DataObject
	case "boolean":
		return domain.DataBoolean
	default:
		return domain.DataText
	}
}

func isConfigurationOnlyField(kind string) bool {
	return kind == "route-options" || kind == "switch-cases" || kind == "json-schema" || kind == "type-spec" || kind == "wire-representation" || kind == "secret" || kind == "field-outputs" || kind == "object-fields" || kind == "http-headers" || kind == "http-user-agent-toggle" || kind == "http-user-agent" || kind == "javascript-editor" || kind == "boolean" || kind == "chat-mode" || kind == "form-builder" || kind == "image-editor" || kind == "embed-editor" || kind == "llm-provider"
}

func field(name, label, kind, placeholder string, required bool) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: kind, Placeholder: placeholder, Required: required}
}

func selectField(name, label string, options []string) domain.ConfigField {
	values := make([]domain.Option, 0, len(options))
	for _, option := range options {
		values = append(values, domain.Option{Value: option, Label: option})
	}
	return domain.ConfigField{Name: name, Label: label, Kind: "select", Options: values, Required: true}
}

func routeOptionsField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "route-options", Required: true}
}

func schemaField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "json-schema", Required: true}
}

func httpHeadersField() domain.ConfigField {
	return domain.ConfigField{Name: "headers", Label: "Request headers", Kind: "http-headers"}
}

func boolField(name, label string, defaultValue bool) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "boolean"} // default wired via DefaultConfig
}

func customUserAgentToggleField() domain.ConfigField {
	return domain.ConfigField{Name: "useCustomUserAgent", Label: "Use custom User-Agent", Kind: "http-user-agent-toggle"}
}

// chatModeField selects how the agent receives its conversation: a single
// composed message, or the prior turns of a chat conversation by ID.
func chatModeField() domain.ConfigField {
	return domain.ConfigField{Name: "chatMode", Label: "Mode", Kind: "chat-mode", Required: true, Options: []domain.Option{
		{Value: "message", Label: "One message"},
		{Value: "history", Label: "Chat history"},
	}}
}

// unlimitedTurnsToggleField removes the tool-turn budget for long-running
// agent tasks; cancellation remains the only stop condition.
func unlimitedTurnsToggleField() domain.ConfigField {
	return domain.ConfigField{Name: "unlimitedTurns", Label: "Unlimited turns", Kind: "boolean"}
}

// chatStatusToggleField lets an LLM node publish live progress to the chat run
// that triggered it. The revealed Chat Run ID pin carries the run to update.
func chatStatusToggleField() domain.ConfigField {
	return domain.ConfigField{Name: "updateChatStatus", Label: "Update chat status", Kind: "boolean"}
}

// llmProviderField selects the provider for one AI node. An empty value keeps
// the node on the application's default provider, so changing the default
// re-routes every node that never made an explicit choice.
func llmProviderField() domain.ConfigField {
	return domain.ConfigField{Name: "providerId", Label: "Provider", Kind: "llm-provider"}
}

// llmModelField selects one of the provider's configured models. An empty
// value uses the provider's default model.
func llmModelField() domain.ConfigField {
	return domain.ConfigField{Name: "model", Label: "Model", Kind: "llm-model"}
}

func visibleWhen(configField domain.ConfigField, name string) domain.ConfigField {
	configField.VisibleWhen = name
	return configField
}

func routeOptions(values ...string) []any {
	options := make([]any, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		options = append(options, map[string]any{"id": values[index], "label": values[index+1]})
	}
	return options
}

func objectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// httpResultOutput describes the merged response packet the HTTP Request
// node places on its Result pin, so tooltips and downstream pickers show the
// real structure instead of a bare object.
func httpResultOutput() domain.NodePort {
	statusSpec := domain.TypeSpec{Kind: domain.TypeFloat}
	bodySpec := domain.TypeSpec{Kind: domain.TypeString}
	jsonSpec := domain.TypeSpec{Kind: domain.TypeAny}
	headerSpec := domain.TypeSpec{Kind: domain.TypeMap, Key: &domain.TypeSpec{Kind: domain.TypeString}, Value: &domain.TypeSpec{Kind: domain.TypeString}}
	record := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "status", Name: "status", Type: statusSpec},
		{ID: "body", Name: "body", Type: bodySpec},
		{ID: "headers", Name: "headers", Type: headerSpec},
		{ID: "json", Name: "json", Type: jsonSpec},
	}}
	return domain.NodePort{
		ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
		DataType: domain.DataObject, Type: &record, Color: "#60a5fa", MaxConnections: 1,
		Fields: []domain.DataField{
			{Path: "status", Label: "Status", DataType: domain.DataNumber},
			{Path: "body", Label: "Body", DataType: domain.DataText},
			{Path: "headers", Label: "Headers", DataType: domain.DataObject},
			{Path: "json", Label: "JSON", DataType: domain.DataAny},
		},
	}
}

// pipelinesListNode builds the List Pipelines node. Its executor needs a
// PipelineLister, so it lives beside the other catalog helpers that depend on
// runtime wiring.
func pipelinesListNode() domain.NodeDefinition {
	textSpec := domain.TypeSpec{Kind: domain.TypeString}
	floatSpec := domain.TypeSpec{Kind: domain.TypeFloat}
	pipelineItem := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "id", Name: "id", Type: textSpec},
		{ID: "name", Name: "name", Type: textSpec},
		{ID: "description", Name: "description", Type: textSpec},
		{ID: "status", Name: "status", Type: textSpec},
		{ID: "publishedRevision", Name: "publishedRevision", Type: floatSpec},
	}}
	listSpec := domain.TypeSpec{Kind: domain.TypeList, Value: &pipelineItem}
	flowIn := []domain.NodePort{{ID: "in", Label: "Input", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa"}}
	flowOut := []domain.NodePort{{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa"}}
	dataOut := []domain.NodePort{
		{ID: "pipelines", Label: "Pipelines", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataList, Type: &listSpec, Color: "#60a5fa", MaxConnections: 1},
		{ID: "count", Label: "Count", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &floatSpec, Color: "#86efac", MaxConnections: 1},
	}
	return normalizeDefinition(domain.NodeDefinition{
		Type:              "action:list_pipelines",
		Category:          "Actions",
		Label:             "List Pipelines",
		Description:       "Emit the published pipelines of this workspace as data.",
		Icon:              "list-video",
		Color:             "#60a5fa",
		Mode:              domain.NodeImpure,
		Inputs:            flowIn,
		Outputs:           append(flowOut, dataOut...),
		Fields:            nil,
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	})
}
