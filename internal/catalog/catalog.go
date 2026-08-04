// Package catalog provides the single source of truth for built-in node metadata.
package catalog

import (
	"sort"
	"strings"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Registry keeps node metadata shared by graph validation and the React palette.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]domain.NodeDefinition
	builtins    map[string]struct{}
}

// New creates a registry containing Neuropipe's built-in node catalog.
func New() *Registry {
	definitions := make(map[string]domain.NodeDefinition)
	builtinTypes := make(map[string]struct{})
	for _, definition := range builtins() {
		definitions[definition.Type] = definition
		builtinTypes[definition.Type] = struct{}{}
	}
	return &Registry{definitions: definitions, builtins: builtinTypes}
}

// Get returns a node definition by its stable type identifier.
func (r *Registry) Get(nodeType string) (domain.NodeDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definition, ok := r.definitions[nodeType]
	return definition, ok
}

// All returns the current catalog in display order.
func (r *Registry) All() []domain.NodeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definitions := make([]domain.NodeDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Category == definitions[j].Category {
			return definitions[i].Label < definitions[j].Label
		}
		return definitions[i].Category < definitions[j].Category
	})
	return definitions
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
	chatTriggerOutput := []domain.NodePort{
		port("out", "Start", "flow", "#fafafa"),
		{ID: "text", Label: "Text", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
		{ID: "chatId", Label: "Chat ID", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
		{ID: "chatRunId", Label: "Chat Run ID", Kind: domain.PinData, DataType: domain.DataText, Color: dataColor(domain.DataText), MaxConnections: 1},
	}
	flowInput := []domain.NodePort{port("in", "Input", "flow", "#a1a1aa")}
	flowOutput := []domain.NodePort{port("out", "Output", "flow", "#fafafa")}
	definitions := []domain.NodeDefinition{
		node("trigger:button", "Triggers", "Button Trigger", "Launch this published pipeline from the Trigger board.", "play", "#fafafa", nil, triggerOutput,
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

		node("action:http", "Actions", "HTTP Request", "Call an HTTP endpoint and pass its JSON or text response downstream.", "globe", "#60a5fa", flowInput, flowOutput,
			[]domain.ConfigField{
				field("url", "URL", "string", "https://api.example.com", true),
				selectField("method", "Method", []string{"GET", "POST", "PUT", "PATCH", "DELETE"}),
				field("body", "Body", "textarea", "", false),
				httpHeadersField(),
				customUserAgentToggleField(),
				visibleWhen(field("userAgent", "User-Agent", "http-user-agent", "Neuropipe/0.1", true), "useCustomUserAgent"),
			}, map[string]any{"method": "GET", "headers": []any{}, "useCustomUserAgent": false, "userAgent": ""}, domain.CapabilityNetwork),
		node("action:file_read", "Local", "Read File", "Read text, JSON, or CSV from an approved local path.", "file-down", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{field("path", "Path", "string", "C:\\Work\\input.json", true)}, map[string]any{}, domain.CapabilityFileRead),
		node("action:file_write", "Local", "Write File", "Write text or JSON to an approved local path.", "file-up", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{field("path", "Path", "string", "C:\\Work\\output.json", true), field("content", "Content", "textarea", "", true)}, map[string]any{}, domain.CapabilityFileWrite),
		node("action:terminal", "Local", "Run Terminal Command", "Run PowerShell, Windows PowerShell, or cmd in an approved workspace.", "terminal", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{selectField("shell", "Shell", []string{"PowerShell", "Windows PowerShell", "cmd"}), field("command", "Command", "textarea", "Get-Date", true), field("workingDirectory", "Working directory", "string", "C:\\Work", false)}, map[string]any{"shell": "PowerShell"}, domain.CapabilityTerminal),
		node("action:notification", "Local", "Desktop Notification", "Show a Windows toast-style desktop notification.", "bell", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{field("title", "Title", "string", "Neuropipe", true), field("message", "Message", "textarea", "Done", true)}, map[string]any{}),
		node("action:report", "Actions", "Create Report", "Save rendered Markdown to the local Reports feed.", "notebook-pen", "#60a5fa", flowInput, flowOutput,
			[]domain.ConfigField{field("title", "Report title", "string", "Pipeline report", true), field("tags", "Tags", "tags", "Daily, Operations", false), field("markdown", "Markdown", "textarea", "# Report", true)}, map[string]any{}),
		node("action:git", "Local", "Git", "Run a focused local Git operation in an approved repository.", "git-branch", "#c4b5fd", flowInput, flowOutput,
			[]domain.ConfigField{selectField("operation", "Operation", []string{"status", "diff", "log", "fetch", "pull"}), field("repository", "Repository", "string", "C:\\Work\\repo", true)}, map[string]any{"operation": "status"}, domain.CapabilityGit),
		node("action:subpipeline", "Actions", "Run Pipeline", "Run another published pipeline with the current packet.", "workflow", "#60a5fa", flowInput, flowOutput,
			[]domain.ConfigField{field("pipelineId", "Pipeline", "string", "", true)}, map[string]any{}),

		node("llm:prompt", "AI", "LLM Prompt", "Generate text with the selected provider and model.", "sparkles", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("prompt", "Prompt", "textarea", "Summarise the connected input.", true), field("model", "Model override", "string", "", false)}, map[string]any{}),
		node("llm:extract", "AI", "Structured Extract", "Extract schema-shaped JSON from the current packet.", "braces", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("prompt", "Instructions", "textarea", "Extract the requested fields.", true), schemaField("schema", "Fields to extract")}, map[string]any{"schema": objectSchema()}),
		node("llm:boolean", "AI", "LLM Boolean Router", "Call a constrained route function and emit exactly one decision branch.", "circle-help", "#f472b6", flowInput, []domain.NodePort{port("true", "True", "flow", "#34d399"), port("false", "False", "flow", "#f87171"), port("error", "Error", "flow", "#fb7185")},
			[]domain.ConfigField{field("prompt", "Question", "textarea", "Is the connected input ready?", true)}, map[string]any{}),
		node("llm:choice", "AI", "LLM Choice Router", "Choose one configured option through constrained structured output.", "list-checks", "#f472b6", flowInput, []domain.NodePort{port("error", "Error", "flow", "#fb7185")},
			[]domain.ConfigField{field("prompt", "Question", "textarea", "Choose the best option.", true), routeOptionsField("options", "Options")}, map[string]any{"options": routeOptions("option-a", "Option A", "option-b", "Option B")}),
		node("llm:summarize", "AI", "Summarize", "Create a concise summary of input data.", "align-left", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("instructions", "Instructions", "textarea", "Summarise the input for a busy reader.", true)}, map[string]any{}),
		node("llm:agent", "AI", "Agent", "A tool-using agent with only explicitly connected tools.", "bot", "#f472b6", flowInput, []domain.NodePort{port("out", "Output", "flow", "#fafafa"), port("tool", "Tools", "tool", "#a78bfa")},
			[]domain.ConfigField{field("instructions", "Instructions", "textarea", "Complete the task using the connected tools.", true), field("maxTurns", "Maximum turns", "number", "8", true)}, map[string]any{"maxTurns": 8}),
		node("llm:coding_agent", "AI", "Coding Agent", "An agent preset for scoped file, Git, and terminal workspaces.", "code-2", "#f472b6", flowInput, flowOutput,
			[]domain.ConfigField{field("task", "Task", "textarea", "", true), field("workspace", "Workspace", "string", "C:\\Work\\repo", true), field("maxTurns", "Maximum turns", "number", "12", true)}, map[string]any{"maxTurns": 12}, domain.CapabilityFileRead, domain.CapabilityFileWrite, domain.CapabilityTerminal, domain.CapabilityGit),
		node("visual:comment", "Canvas", "Comment", "A canvas-only note for documenting a Blueprint-style graph.", "message-square-text", "#71717a", nil, nil,
			[]domain.ConfigField{field("title", "Title", "string", "New comment", true), field("body", "Body", "textarea", "Describe this section of the pipeline.", false)}, map[string]any{"title": "New comment", "body": ""}),
	}
	return append(definitions, blueprintBuiltins()...)
}

func node(nodeType, category, label, description, icon, color string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any, capabilities ...domain.Capability) domain.NodeDefinition {
	return normalizeDefinition(domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: color, Inputs: inputs, Outputs: outputs, Fields: fields, Capabilities: capabilities, DefaultConfig: defaults, Source: "builtin"})
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
	if kind == "flow" || kind == "tool" {
		pinKind = domain.PinExec
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
		pin.Color = dataColor(pin.DataType)
	}
	if pin.MaxConnections == 0 {
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
	if definition.Type == "visual:comment" {
		return domain.NodeVisual
	}
	return domain.NodeImpure
}

func addFieldPins(definition domain.NodeDefinition) domain.NodeDefinition {
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
		pin := domain.NodePort{ID: field.Name, Label: field.Label, Kind: domain.PinData, Direction: domain.PinInput, DataType: dataType, Required: field.Required, MaxConnections: 1}
		if hasDefault {
			pin.Default = defaultValue
		}
		definition.Inputs = append(definition.Inputs, pin)
	}
	if definition.Mode == domain.NodeEvent && definition.Type != "trigger:chat" {
		definition.Outputs = append(definition.Outputs, domain.NodePort{ID: "payload", Label: "Payload", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Color: "#60a5fa"})
	}
	if definition.Mode == domain.NodeImpure {
		found := false
		for _, pin := range definition.Outputs {
			if pin.ID == "result" && pin.Kind == domain.PinData {
				found = true
			}
		}
		if !found {
			definition.Outputs = append(definition.Outputs, domain.NodePort{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Color: "#60a5fa"})
		}
	}
	return definition
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
	return kind == "route-options" || kind == "switch-cases" || kind == "json-schema" || kind == "secret" || kind == "field-outputs" || kind == "object-fields" || kind == "http-headers" || kind == "http-user-agent-toggle" || kind == "http-user-agent"
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

func switchCasesField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "switch-cases", Required: true}
}

func schemaField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "json-schema", Required: true}
}

func httpHeadersField() domain.ConfigField {
	return domain.ConfigField{Name: "headers", Label: "Request headers", Kind: "http-headers"}
}

func customUserAgentToggleField() domain.ConfigField {
	return domain.ConfigField{Name: "useCustomUserAgent", Label: "Use custom User-Agent", Kind: "http-user-agent-toggle"}
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

func switchCasesConfig() map[string]any {
	return map[string]any{
		"comparator": "equals",
		"cases": []any{
			map[string]any{"id": "case-a", "label": "Case A", "valueType": "text", "value": "case-a"},
			map[string]any{"id": "case-b", "label": "Case B", "valueType": "text", "value": "case-b"},
		},
	}
}

func objectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
