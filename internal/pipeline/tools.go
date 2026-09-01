package pipeline

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// connectedTool keeps the public provider contract separate from the stable
// function-pin IDs used by the Blueprint interpreter.
type connectedTool struct {
	function   domain.CustomFunction
	definition domain.ChatToolDefinition
	inputs     map[string]string
	outputs    map[string]string
}

// executeConnectedToolAgent runs a native function-tool conversation for an
// Agent node. The host owns model turns and function invocation; the provider
// only receives declarative schemas and JSON-safe tool results.
func (s *blueprintState) executeConnectedToolAgent(node domain.FlowNode, config, inputs map[string]any) (map[string]any, error) {
	tools, err := s.connectedTools(node.ID)
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return nil, nil
	}
	assistant, supported := s.engine.llm.(AssistantRunner)
	if !supported {
		return nil, fmt.Errorf("the configured LLM provider does not support connected tools")
	}
	if assistant == nil {
		return nil, fmt.Errorf("configure an LLM provider in Settings before running AI nodes")
	}
	if err := s.ctx.Err(); err != nil {
		return nil, fmt.Errorf("execution cancelled: %w", err)
	}

	// A wired data pin always overrides the inspector value, matching every
	// other impure node (see the config merge in blueprint.go).
	merged := cloneValues(config)
	for key, value := range inputs {
		merged[key] = value
	}

	prompt := text(merged, "instructions")
	if node.Type == "llm:coding_agent" {
		prompt = "Coding task:\n" + text(merged, "task") + "\n\nWorkspace: " + text(merged, "workspace")
	}
	history, err := s.engine.agentHistory(s.ctx, node, merged)
	if err != nil {
		return nil, err
	}
	messages := agentHistoryMessages(prompt, history)
	if history == nil {
		// One-message mode: the composed task is the single user turn. In
		// chat-history mode the conversation already ends with the user's
		// latest message, which the agent answers directly.
		messages = append(messages, domain.ChatMessage{Role: domain.ChatRoleUser, Content: promptWithInput(prompt, Packet(inputs))})
	}
	definitions := make([]domain.ChatToolDefinition, 0, len(tools))
	byName := make(map[string]connectedTool, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, tool.definition)
		byName[tool.definition.Name] = tool
	}

	maxTurns, err := configuredToolTurns(config)
	if err != nil {
		return nil, err
	}
	status, err := s.engine.chatStatusReporter(s.ctx, node, merged)
	if err != nil {
		return nil, err
	}
	for turn := 0; maxTurns < 0 || turn < maxTurns; turn++ {
		if err := reportModelStatus(status, chatStatusThinking); err != nil {
			return nil, err
		}
		response, err := assistant.Converse(s.ctx, domain.AssistantChatRequest{
			Messages:   messages,
			Tools:      definitions,
			ProviderID: text(merged, "providerId"),
			Model:      text(merged, "model"),
			Metrics:    s.engine.llmMetricContext(node),
		})
		if err != nil {
			return nil, err
		}
		if len(response.ToolCalls) == 0 {
			return mergePacket(Packet(inputs), Packet{"llm": map[string]any{"content": response.Content}}), nil
		}
		messages = append(messages, domain.ChatMessage{Role: domain.ChatRoleAssistant, Content: response.Content, ToolCalls: response.ToolCalls})
		for _, call := range response.ToolCalls {
			tool, found := byName[call.Name]
			if !found {
				return nil, fmt.Errorf("LLM requested an unavailable tool")
			}
			// Chat readers see the published function name, not the generated
			// tool-call identifier.
			displayName := tool.function.Name
			if displayName == "" {
				displayName = call.Name
			}
			if err := reportModelStatus(status, toolStatusText(displayName)); err != nil {
				return nil, err
			}
			result, err := s.runConnectedTool(node, tool, call.Arguments)
			if err != nil {
				// A bad tool call is feedback for the model, not a reason to end
				// the whole agent run. The raw error can contain local execution
				// details, so return only a safe contract-oriented message.
				result = toolFailureResult(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("encode LLM tool result: %w", err)
			}
			messages = append(messages, domain.ChatMessage{Role: domain.ChatRoleTool, Content: string(encoded), ToolCallID: call.ID, ToolName: call.Name})
		}
	}
	return nil, fmt.Errorf("LLM agent reached its maximum number of tool turns")
}

func (s *blueprintState) connectedTools(agentID string) ([]connectedTool, error) {
	hasToolEdges := false
	for _, edge := range s.definition.Edges {
		if edge.Target == agentID && edge.TargetHandle == "tools" && edgeKind(edge) == domain.PinTool {
			hasToolEdges = true
			break
		}
	}
	if !hasToolEdges {
		return nil, nil
	}
	if s.engine.functions == nil {
		return nil, fmt.Errorf("custom functions are unavailable")
	}
	result := make([]connectedTool, 0)
	seen := make(map[string]struct{})
	for _, edge := range s.definition.Edges {
		if edge.Target != agentID || edge.TargetHandle != "tools" || edgeKind(edge) != domain.PinTool {
			continue
		}
		source, exists := s.nodes[edge.Source]
		if !exists || !strings.HasPrefix(source.Type, "function:") {
			return nil, fmt.Errorf("tools pin has an invalid source")
		}
		functionID := strings.TrimPrefix(source.Type, "function:")
		if _, duplicate := seen[functionID]; duplicate {
			continue
		}
		function, err := s.engine.functions.GetPublishedFunction(s.ctx, functionID)
		if err != nil {
			return nil, err
		}
		if function.Kind != domain.FunctionTool {
			return nil, fmt.Errorf("connected function %q is not an LLM tool", function.Name)
		}
		tool, err := makeConnectedTool(function)
		if err != nil {
			return nil, err
		}
		seen[functionID] = struct{}{}
		result = append(result, tool)
	}
	return result, nil
}

func makeConnectedTool(function domain.CustomFunction) (connectedTool, error) {
	if err := ValidateToolFunction(function); err != nil {
		return connectedTool{}, err
	}
	name := toolName(function)
	inputs, inputNames, err := toolPinNames(function.Inputs)
	if err != nil {
		return connectedTool{}, fmt.Errorf("prepare LLM tool %q inputs: %w", function.Name, err)
	}
	_, outputNames, err := toolPinNames(function.Outputs)
	if err != nil {
		return connectedTool{}, fmt.Errorf("prepare LLM tool %q outputs: %w", function.Name, err)
	}
	properties := make(map[string]any, len(function.Inputs))
	required := make([]string, 0, len(function.Inputs))
	for _, pin := range function.Inputs {
		argumentName := inputNames[pin.ID]
		schema, err := toolJSONSchema(functionPinType(pin))
		if err != nil {
			return connectedTool{}, fmt.Errorf("prepare LLM tool %q input %q: %w", function.Name, pin.Name, err)
		}
		schema["description"] = strings.TrimSpace(pin.Description)
		properties[argumentName] = schema
		if pin.Default != nil {
			schema["default"] = pin.Default
		}
		if pin.Required {
			required = append(required, argumentName)
		}
	}
	return connectedTool{
		function: function,
		definition: domain.ChatToolDefinition{
			Name:        name,
			Description: toolDescription(function),
			InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false},
		},
		inputs:  inputs,
		outputs: outputNames,
	}, nil
}

func (s *blueprintState) runConnectedTool(agent domain.FlowNode, tool connectedTool, arguments map[string]any) (map[string]any, error) {
	inputs := make(map[string]any, len(tool.function.Inputs))
	known := make(map[string]struct{}, len(tool.inputs))
	for argumentName, pinID := range tool.inputs {
		known[argumentName] = struct{}{}
		if value, exists := arguments[argumentName]; exists {
			inputs[pinID] = value
		}
	}
	for argumentName := range arguments {
		if _, exists := known[argumentName]; !exists {
			return nil, fmt.Errorf("LLM tool %q arguments: unknown argument", tool.function.Name)
		}
	}
	inputs, err := decodeToolArguments(tool.function, inputs)
	if err != nil {
		return nil, fmt.Errorf("LLM tool %q arguments: %w", tool.function.Name, err)
	}
	inputs, err = functionCallInputs(tool.function, inputs)
	if err != nil {
		return nil, fmt.Errorf("LLM tool %q arguments: %w", tool.function.Name, err)
	}
	outputs, err := s.runFunction(domain.FlowNode{ID: agent.ID + ":tool:" + tool.function.ID, Type: "function:" + tool.function.ID, Data: map[string]any{"config": map[string]any{}}}, inputs, newBlueprintFrame())
	if err != nil {
		return nil, fmt.Errorf("run LLM tool %q: %w", tool.function.Name, err)
	}
	result := make(map[string]any, len(outputs))
	for pinID, value := range outputs {
		name, exists := tool.outputs[pinID]
		if !exists {
			continue
		}
		result[name] = value
	}
	return result, nil
}

// ValidateToolFunction ensures a published function can become a precise,
// JSON-safe native tool definition. It intentionally lives beside tool schema
// construction so the Wails façade and runtime share one contract.
func ValidateToolFunction(function domain.CustomFunction) error {
	if function.Kind != domain.FunctionTool {
		return fmt.Errorf("function %q is not an LLM tool", function.Name)
	}
	if strings.TrimSpace(function.Description) == "" {
		return fmt.Errorf("LLM tool %q needs a description explaining when to use it", function.Name)
	}
	if len(function.Outputs) == 0 {
		return fmt.Errorf("LLM tool %q needs at least one described output", function.Name)
	}
	for _, side := range []struct {
		name string
		pins []domain.FunctionPin
	}{{"input", function.Inputs}, {"output", function.Outputs}} {
		for _, pin := range side.pins {
			if strings.TrimSpace(pin.Description) == "" {
				return fmt.Errorf("LLM tool %q %s %q needs model guidance", function.Name, side.name, pin.Name)
			}
			if _, err := toolJSONSchema(functionPinType(pin)); err != nil {
				return fmt.Errorf("LLM tool %q %s %q: %w", function.Name, side.name, pin.Name, err)
			}
		}
	}
	if _, _, err := toolPinNames(function.Inputs); err != nil {
		return fmt.Errorf("LLM tool %q inputs: %w", function.Name, err)
	}
	if _, _, err := toolPinNames(function.Outputs); err != nil {
		return fmt.Errorf("LLM tool %q outputs: %w", function.Name, err)
	}
	return nil
}

func toolName(function domain.CustomFunction) string {
	base := toolIdentifier(function.Name)
	if base == "" {
		base = "tool"
	}
	suffix := toolIdentifier(strings.ReplaceAll(function.ID, "-", ""))
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	if suffix == "" {
		return "tool_" + base
	}
	const maxNameLength = 64
	prefix := "tool_"
	available := maxNameLength - len(prefix) - len(suffix) - 1
	if available < 1 {
		return prefix + suffix
	}
	if len(base) > available {
		base = strings.TrimRight(base[:available], "_")
	}
	return prefix + base + "_" + suffix
}

func toolIdentifier(value string) string {
	var builder strings.Builder
	previousSeparator := false
	for _, letter := range strings.ToLower(strings.TrimSpace(value)) {
		if letter >= 'a' && letter <= 'z' || letter >= '0' && letter <= '9' {
			builder.WriteRune(letter)
			previousSeparator = false
			continue
		}
		if builder.Len() > 0 && !previousSeparator {
			builder.WriteByte('_')
			previousSeparator = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func toolDescription(function domain.CustomFunction) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(function.Description))
	builder.WriteString("\n\nReturns a JSON object with:\n")
	for index, pin := range function.Outputs {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString("- ")
		builder.WriteString(toolArgumentName(pin.Name, index+1))
		builder.WriteString(" (")
		builder.WriteString(string(functionPinType(pin).Kind))
		builder.WriteString("): ")
		builder.WriteString(strings.TrimSpace(pin.Description))
	}
	return builder.String()
}

func toolJSONSchema(spec domain.TypeSpec) (map[string]any, error) {
	if err := validateToolType(spec); err != nil {
		return nil, err
	}
	return typeSpecJSONSchema(spec), nil
}

func validateToolType(spec domain.TypeSpec) error {
	if err := typespec.ValidateSpec(spec); err != nil {
		return err
	}
	switch spec.Kind {
	case domain.TypeAny:
		return fmt.Errorf("must use a concrete TypeSpec instead of any")
	case domain.TypeList:
		return validateToolType(*spec.Element)
	case domain.TypeMap:
		if spec.Key == nil || spec.Key.Kind != domain.TypeString {
			return fmt.Errorf("JSON object map keys must be string")
		}
		return validateToolType(*spec.Value)
	case domain.TypeRecord:
		if spec.Name != "" {
			return fmt.Errorf("named records cannot be supplied by JSON tool calls")
		}
		for _, field := range spec.Fields {
			if strings.TrimSpace(field.Name) == "" {
				return fmt.Errorf("record fields need names for a JSON tool contract")
			}
			if err := validateToolType(field.Type); err != nil {
				return fmt.Errorf("record field %q: %w", field.Name, err)
			}
		}
	}
	return nil
}

func decodeToolArguments(function domain.CustomFunction, values map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(values))
	byID := make(map[string]domain.FunctionPin, len(function.Inputs))
	for _, pin := range function.Inputs {
		byID[pin.ID] = pin
	}
	for id, value := range values {
		pin, exists := byID[id]
		if !exists {
			return nil, fmt.Errorf("unknown input")
		}
		decoded, err := decodeToolValue(value, functionPinType(pin))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", pin.Name, err)
		}
		result[id] = decoded
	}
	return result, nil
}

// decodeToolValue decodes JSON transport values to the exact Go values used by
// the function contract. JSON has one number representation, so an integral
// number is decoded to int64 only for an explicitly declared int. This is
// transport decoding, not a Blueprint wire conversion.
func decodeToolValue(value any, spec domain.TypeSpec) (any, error) {
	switch spec.Kind {
	case domain.TypeBool:
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return value, nil
	case domain.TypeString:
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("must be text")
		}
		return value, nil
	case domain.TypeInt:
		return decodeToolInteger(value)
	case domain.TypeFloat:
		return decodeToolFloat(value)
	case domain.TypeBytes:
		encoded, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a Base64 string")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("must be valid Base64")
		}
		return decoded, nil
	case domain.TypeList:
		items, ok := value.([]any)
		if !ok || spec.Element == nil {
			return nil, fmt.Errorf("must be a list")
		}
		result := make([]any, len(items))
		for index, item := range items {
			decoded, err := decodeToolValue(item, *spec.Element)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", index+1, err)
			}
			result[index] = decoded
		}
		return result, nil
	case domain.TypeMap:
		if spec.Key == nil || spec.Value == nil || spec.Key.Kind != domain.TypeString {
			return nil, fmt.Errorf("must be a string-keyed map")
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("must be an object")
		}
		result := make(map[string]any, len(object))
		for key, item := range object {
			decoded, err := decodeToolValue(item, *spec.Value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			result[key] = decoded
		}
		return result, nil
	case domain.TypeRecord:
		if spec.Name != "" {
			return nil, fmt.Errorf("must use an anonymous record contract")
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("must be an object")
		}
		fields := make(map[string]domain.TypeFieldSpec, len(spec.Fields))
		for _, field := range spec.Fields {
			fields[field.Name] = field
		}
		result := make(map[string]any, len(object))
		for key, item := range object {
			field, exists := fields[key]
			if !exists {
				return nil, fmt.Errorf("has an unknown field %q", key)
			}
			decoded, err := decodeToolValue(item, field.Type)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			result[key] = decoded
		}
		return result, nil
	default:
		return nil, fmt.Errorf("must use a supported JSON tool type")
	}
}

func decodeToolInteger(value any) (int64, error) {
	switch number := value.(type) {
	case json.Number:
		result, err := number.Int64()
		if err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
		return result, nil
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < math.MinInt64 || number > math.MaxInt64 {
			return 0, fmt.Errorf("must be an integer")
		}
		return int64(number), nil
	case float32:
		return decodeToolInteger(float64(number))
	case int:
		return int64(number), nil
	case int64:
		return number, nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func decodeToolFloat(value any) (float64, error) {
	var result float64
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		if err != nil {
			return 0, fmt.Errorf("must be a number")
		}
		result = parsed
	case float64:
		result = number
	case float32:
		result = float64(number)
	case int:
		result = float64(number)
	case int64:
		result = float64(number)
	default:
		return 0, fmt.Errorf("must be a number")
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, fmt.Errorf("must be finite")
	}
	return result, nil
}

func toolFailureResult(err error) map[string]any {
	if strings.Contains(err.Error(), "arguments:") {
		return map[string]any{"error": "The arguments do not match this tool's published contract. Correct them and try again."}
	}
	return map[string]any{"error": "The tool could not complete the request. Try a different supported action or finish without this tool."}
}

// configuredToolTurns resolves the agent's tool-turn budget. Unlimited turns
// return -1 so long-running tasks keep going until the model stops calling
// tools or the execution is cancelled.
func configuredToolTurns(config map[string]any) (int, error) {
	if boolValue(config["unlimitedTurns"]) {
		return -1, nil
	}
	value, exists := config["maxTurns"]
	if !exists || value == nil {
		return 8, nil
	}
	count, ok := asNumber(value)
	if !ok || math.IsNaN(count) || math.IsInf(count, 0) || count < 1 || math.Trunc(count) != count {
		return 0, fmt.Errorf("maximum tool turns must be a positive integer")
	}
	if count > 32 {
		return 0, fmt.Errorf("maximum tool turns cannot exceed 32")
	}
	return int(count), nil
}

func functionCallInputs(function domain.CustomFunction, values map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(function.Inputs))
	known := make(map[string]struct{}, len(function.Inputs))
	for _, pin := range function.Inputs {
		known[pin.ID] = struct{}{}
		value, found := values[pin.ID]
		if !found && pin.Default != nil {
			value, found = pin.Default, true
		}
		if !found {
			if pin.Required {
				return nil, fmt.Errorf("function input %q is required", pin.Name)
			}
			continue
		}
		if err := typespec.ValidateValue(value, functionPinType(pin)); err != nil {
			return nil, fmt.Errorf("function input %q: %w", pin.Name, err)
		}
		result[pin.ID] = value
	}
	for id := range values {
		if _, exists := known[id]; !exists {
			return nil, fmt.Errorf("function received an unknown input")
		}
	}
	return result, nil
}

func functionPinType(pin domain.FunctionPin) domain.TypeSpec {
	if pin.Type != nil {
		return *pin.Type
	}
	return typespec.FromDataType(pin.DataType)
}

func toolPinNames(pins []domain.FunctionPin) (map[string]string, map[string]string, error) {
	byName := make(map[string]string, len(pins))
	byID := make(map[string]string, len(pins))
	for index, pin := range pins {
		name := toolArgumentName(pin.Name, index+1)
		if _, exists := byName[name]; exists {
			return nil, nil, fmt.Errorf("duplicate argument name %q", name)
		}
		byName[name], byID[pin.ID] = pin.ID, name
	}
	return byName, byID, nil
}

func toolArgumentName(value string, index int) string {
	var builder strings.Builder
	for _, letter := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(letter) || unicode.IsDigit(letter) || letter == '_' {
			builder.WriteRune(letter)
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('_')
		}
	}
	name := strings.Trim(builder.String(), "_")
	if name == "" || unicode.IsDigit(rune(name[0])) {
		return fmt.Sprintf("argument_%d", index)
	}
	return name
}

func typeSpecJSONSchema(spec domain.TypeSpec) map[string]any {
	switch spec.Kind {
	case domain.TypeBool:
		return map[string]any{"type": "boolean"}
	case domain.TypeString:
		return map[string]any{"type": "string"}
	case domain.TypeInt:
		return map[string]any{"type": "integer"}
	case domain.TypeFloat:
		return map[string]any{"type": "number"}
	case domain.TypeBytes:
		return map[string]any{"type": "string", "contentEncoding": "base64"}
	case domain.TypeList:
		item := domain.TypeSpec{Kind: domain.TypeAny}
		if spec.Element != nil {
			item = *spec.Element
		}
		return map[string]any{"type": "array", "items": typeSpecJSONSchema(item)}
	case domain.TypeMap:
		value := domain.TypeSpec{Kind: domain.TypeAny}
		if spec.Value != nil {
			value = *spec.Value
		}
		return map[string]any{"type": "object", "additionalProperties": typeSpecJSONSchema(value)}
	case domain.TypeRecord:
		properties := make(map[string]any, len(spec.Fields))
		required := make([]string, 0, len(spec.Fields))
		for _, field := range spec.Fields {
			properties[field.Name] = typeSpecJSONSchema(field.Type)
			if !field.Optional {
				required = append(required, field.Name)
			}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	default:
		return map[string]any{}
	}
}
