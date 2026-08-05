package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

const maxBlueprintLoopIterations = 10_000

// Execute runs only the versioned Blueprint graph format. Legacy graphs are
// preserved by persistence but intentionally cannot reach this interpreter.
func (e *Engine) Execute(ctx context.Context, definition domain.FlowDefinition, triggerNodeID string, initial Packet) (RunResult, error) {
	if definition.SchemaVersion != domain.GraphSchemaV2 {
		return RunResult{}, ValidationError{Message: "this is a legacy pipeline. Rebuild it as a Blueprint v2 graph before running."}
	}
	if err := Validate(definition, e.registry); err != nil {
		return RunResult{}, err
	}
	state := newBlueprintState(e, ctx, definition)
	if err := state.start(triggerNodeID, initial); err != nil {
		return state.result, err
	}
	return state.result, nil
}

type blueprintFrame struct {
	values map[string]map[string]any
	pure   map[string]bool
}

func newBlueprintFrame() *blueprintFrame {
	return &blueprintFrame{values: make(map[string]map[string]any), pure: make(map[string]bool)}
}

// loopChild copies only impure/event values. Pure values can depend on loop
// outputs and therefore must be resolved again for each iteration.
func (f *blueprintFrame) loopChild() *blueprintFrame {
	child := newBlueprintFrame()
	for nodeID, outputs := range f.values {
		if f.pure[nodeID] {
			continue
		}
		child.values[nodeID] = cloneValues(outputs)
	}
	return child
}

func cloneValues(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

type blueprintState struct {
	engine           *Engine
	ctx              context.Context
	definition       domain.FlowDefinition
	nodes            map[string]domain.FlowNode
	result           RunResult
	variables        Packet
	visits           int
	callStack        []string
	functionID       string
	functionRevision int
	parentNodeID     string
	functionOutputs  []domain.FunctionPin
	once             map[string]bool
	gates            map[string]bool
	flipFlops        map[string]bool
	multiGates       map[string]int
	loopDepth        int
	breakRequested   bool
}

func newBlueprintState(engine *Engine, ctx context.Context, definition domain.FlowDefinition) *blueprintState {
	nodes := make(map[string]domain.FlowNode, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodes[node.ID] = node
	}
	variables := make(Packet)
	engine.variables = variables
	return &blueprintState{engine: engine, ctx: ctx, definition: definition, nodes: nodes, result: RunResult{NodeRuns: make([]domain.NodeRun, 0, len(definition.Nodes))}, variables: variables, once: make(map[string]bool), gates: make(map[string]bool), flipFlops: make(map[string]bool), multiGates: make(map[string]int)}
}

func (s *blueprintState) start(nodeID string, initial Packet) error {
	node, exists := s.nodes[nodeID]
	if !exists {
		return fmt.Errorf("selected trigger %q does not exist", nodeID)
	}
	definition, _ := s.engine.registry.Get(node.Type)
	if definition.Mode != domain.NodeEvent {
		return fmt.Errorf("node %q is not an event trigger", nodeID)
	}
	frame := newBlueprintFrame()
	if node.Type == "trigger:chat" {
		frame.values[node.ID] = map[string]any{
			"text":      fmt.Sprint(initial["text"]),
			"chatId":    fmt.Sprint(initial["chatId"]),
			"chatRunId": fmt.Sprint(initial["chatRunId"]),
		}
	} else {
		frame.values[node.ID] = map[string]any{"payload": clonePacket(initial), "result": clonePacket(initial)}
	}
	if err := s.recordEvent(node, frame.values[node.ID]); err != nil {
		return err
	}
	return s.follow(node.ID, "out", frame)
}

func (s *blueprintState) recordEvent(node domain.FlowNode, output map[string]any) error {
	started := time.Now().UTC()
	if s.engine.gate != nil {
		definition, _ := s.engine.registry.Get(node.Type)
		if err := s.engine.gate.Allow(s.ctx, node, definition.Capabilities); err != nil {
			return err
		}
	}
	finished := time.Now().UTC()
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: map[string]any{}, Output: output, StartedAt: started, FinishedAt: finished})
	return nil
}

func (s *blueprintState) follow(sourceID, port string, frame *blueprintFrame) error {
	for _, edge := range s.definition.Edges {
		if edge.Source != sourceID || edgeKind(edge) != domain.PinExec {
			continue
		}
		handle := edge.SourceHandle
		if handle == "" {
			handle = "out"
		}
		if handle != port {
			continue
		}
		targetHandle := edge.TargetHandle
		if targetHandle == "" {
			targetHandle = "in"
		}
		if err := s.runExec(edge.Target, targetHandle, frame); err != nil {
			return err
		}
		if s.result.Returned || s.breakRequested {
			return nil
		}
	}
	return nil
}

func (s *blueprintState) runExec(nodeID, execInput string, frame *blueprintFrame) error {
	if err := s.ctx.Err(); err != nil {
		return fmt.Errorf("execution cancelled: %w", err)
	}
	s.visits++
	if s.visits > maxNodeVisits {
		return fmt.Errorf("execution exceeded the %d-node safety limit", maxNodeVisits)
	}
	node, exists := s.nodes[nodeID]
	if !exists {
		return fmt.Errorf("execution reached missing node %q", nodeID)
	}
	definition, exists := s.engine.registry.Get(node.Type)
	if !exists {
		return fmt.Errorf("node %q uses unavailable type %q", node.ID, node.Type)
	}
	definition, err := definitionForNode(definition, node)
	if err != nil {
		return fmt.Errorf("node %q has invalid configuration: %w", node.ID, err)
	}
	if definition.Mode == domain.NodePure {
		return fmt.Errorf("pure node %q cannot receive an Exec pulse", node.ID)
	}
	if definition.Mode == domain.NodeEvent {
		return fmt.Errorf("event node %q cannot receive an Exec pulse", node.ID)
	}

	inputs, err := s.resolveInputs(node, definition, frame)
	if node.Type == "function:return" {
		inputs, err = s.resolveFunctionOutputs(node, frame)
	}
	if err != nil {
		return s.recordFailure(node, inputs, err)
	}
	if s.engine.gate != nil {
		if err := s.engine.gate.Allow(s.ctx, node, definition.Capabilities); err != nil {
			return s.recordFailure(node, inputs, err)
		}
	}

	if node.Type == "flow:for_each" {
		return s.runForEach(node, definition, inputs, frame)
	}
	if node.Type == "flow:for_loop" {
		return s.runForLoop(node, definition, inputs, frame)
	}
	if node.Type == "flow:while" {
		return s.runWhile(node, definition, frame)
	}

	started := time.Now().UTC()
	outputs, ports, err := s.executeImpure(node, definition, inputs, frame, execInput)
	finished := time.Now().UTC()
	if err != nil {
		return s.recordFailureAt(node, inputs, err, started, finished)
	}
	frame.values[node.ID] = outputs
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: inputs, Output: outputs, StartedAt: started, FinishedAt: finished})
	for _, port := range ports {
		nextFrame := frame
		if startsNewActivation(node.Type) {
			nextFrame = frame.loopChild()
		}
		if err := s.follow(node.ID, port, nextFrame); err != nil {
			return err
		}
		if s.result.Returned || s.breakRequested {
			return nil
		}
	}
	return nil
}

// A control split receives an isolated memoization frame. Impure results from
// the path leading to the split remain readable, while pure values are
// recomputed only when the newly activated branch needs them.
func startsNewActivation(nodeType string) bool {
	switch nodeType {
	case "flow:branch", "flow:sequence", "flow:switch", "flow:flip_flop", "flow:multi_gate", "llm:boolean", "llm:choice":
		return true
	default:
		return false
	}
}

func (s *blueprintState) runForEach(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame) error {
	items, ok := asSlice(inputs["items"])
	if !ok {
		return s.recordFailure(node, inputs, fmt.Errorf("for-each loop expects Array to be a list"))
	}
	started := time.Now().UTC()
	s.loopDepth++
	defer func() { s.loopDepth-- }()
	for index, item := range items {
		if index >= maxBlueprintLoopIterations {
			return s.recordFailureAt(node, inputs, fmt.Errorf("for-each loop exceeded the %d iteration limit", maxBlueprintLoopIterations), started, time.Now().UTC())
		}
		child := frame.loopChild()
		child.values[node.ID] = map[string]any{"item": item, "index": float64(index), "result": map[string]any{"item": item, "index": float64(index)}}
		if err := s.follow(node.ID, "loop", child); err != nil {
			return err
		}
		if s.result.Returned {
			return nil
		}
		if s.consumeBreak() {
			break
		}
	}
	outputs := map[string]any{"result": map[string]any{"count": len(items)}}
	frame.values[node.ID] = outputs
	finished := time.Now().UTC()
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: inputs, Output: outputs, StartedAt: started, FinishedAt: finished})
	return s.follow(node.ID, "completed", frame)
}

func (s *blueprintState) runForLoop(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame) error {
	first, firstOK := asInteger(inputs["first"])
	last, lastOK := asInteger(inputs["last"])
	if !firstOK || !lastOK {
		return s.recordFailure(node, inputs, fmt.Errorf("for loop expects numeric First Index and Last Index"))
	}
	started := time.Now().UTC()
	s.loopDepth++
	defer func() { s.loopDepth-- }()
	count := 0
	for index := first; index <= last; index++ {
		count++
		if count > maxBlueprintLoopIterations {
			return s.recordFailureAt(node, inputs, fmt.Errorf("for loop exceeded the %d iteration limit", maxBlueprintLoopIterations), started, time.Now().UTC())
		}
		child := frame.loopChild()
		child.values[node.ID] = map[string]any{"index": float64(index), "result": map[string]any{"index": float64(index)}}
		if err := s.follow(node.ID, "loop", child); err != nil {
			return err
		}
		if s.result.Returned {
			return nil
		}
		if s.consumeBreak() {
			break
		}
	}
	outputs := map[string]any{"result": map[string]any{"count": count}}
	frame.values[node.ID] = outputs
	finished := time.Now().UTC()
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: inputs, Output: outputs, StartedAt: started, FinishedAt: finished})
	return s.follow(node.ID, "completed", frame)
}

func (s *blueprintState) runWhile(node domain.FlowNode, definition domain.NodeDefinition, frame *blueprintFrame) error {
	started := time.Now().UTC()
	s.loopDepth++
	defer func() { s.loopDepth-- }()
	count := 0
	for {
		child := frame.loopChild()
		inputs, err := s.resolveInputs(node, definition, child)
		if err != nil {
			return s.recordFailureAt(node, nil, err, started, time.Now().UTC())
		}
		condition, ok := inputs["condition"].(bool)
		if !ok {
			return s.recordFailureAt(node, inputs, fmt.Errorf("while expects Condition to be Boolean"), started, time.Now().UTC())
		}
		if !condition {
			break
		}
		count++
		if count > maxBlueprintLoopIterations {
			return s.recordFailureAt(node, inputs, fmt.Errorf("while exceeded the %d iteration limit", maxBlueprintLoopIterations), started, time.Now().UTC())
		}
		child.values[node.ID] = map[string]any{"result": map[string]any{"iteration": count}}
		if err := s.follow(node.ID, "loop", child); err != nil {
			return err
		}
		if s.result.Returned {
			return nil
		}
		if s.consumeBreak() {
			break
		}
	}
	outputs := map[string]any{"result": map[string]any{"count": count}}
	frame.values[node.ID] = outputs
	finished := time.Now().UTC()
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: map[string]any{}, Output: outputs, StartedAt: started, FinishedAt: finished})
	return s.follow(node.ID, "completed", frame)
}

func (s *blueprintState) executeImpure(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame, execInput string) (map[string]any, []string, error) {
	if strings.HasPrefix(node.Type, "function:") && node.Type != "function:return" && node.Type != "function:entry" {
		outputs, err := s.runFunction(node, inputs, frame)
		return outputs, []string{"out"}, err
	}
	switch node.Type {
	case "flow:reroute":
		return map[string]any{"result": map[string]any{}}, []string{"out"}, nil
	case "flow:branch":
		condition, ok := inputs["condition"].(bool)
		if !ok {
			return nil, nil, fmt.Errorf("branch expects Condition to be Boolean")
		}
		if condition {
			return map[string]any{"result": map[string]any{"condition": true}}, []string{"true"}, nil
		}
		return map[string]any{"result": map[string]any{"condition": false}}, []string{"false"}, nil
	case "flow:sequence":
		return map[string]any{"result": map[string]any{}}, []string{"then_0", "then_1"}, nil
	case "flow:switch":
		configuration, err := switchConfigurationFor(node, definition.DefaultConfig)
		if err != nil {
			return nil, nil, err
		}
		value := inputs["selection"]
		result := map[string]any{"value": value, "comparator": string(configuration.Comparator), "matchedCase": nil}
		for _, item := range configuration.Cases {
			matched, err := matchSwitchCase(value, configuration, item)
			if err != nil {
				return nil, nil, err
			}
			if matched {
				result["matchedCase"] = map[string]any{"id": item.ID, "label": item.Label}
				return map[string]any{"result": result}, []string{item.ID}, nil
			}
		}
		return map[string]any{"result": result}, []string{"default"}, nil
	case "flow:do_once":
		if execInput == "reset" {
			delete(s.once, node.ID)
			return map[string]any{"result": map[string]any{"reset": true}}, nil, nil
		}
		if s.once[node.ID] {
			return map[string]any{"result": map[string]any{"alreadyDone": true}}, nil, nil
		}
		s.once[node.ID] = true
		return map[string]any{"result": map[string]any{"first": true}}, []string{"out"}, nil
	case "flow:gate":
		open, exists := s.gates[node.ID]
		if !exists {
			open = boolConfig(node, "startOpen", true)
		}
		switch execInput {
		case "open":
			s.gates[node.ID] = true
			return map[string]any{"result": map[string]any{"open": true}}, nil, nil
		case "close":
			s.gates[node.ID] = false
			return map[string]any{"result": map[string]any{"open": false}}, nil, nil
		case "toggle":
			s.gates[node.ID] = !open
			return map[string]any{"result": map[string]any{"open": !open}}, nil, nil
		default:
			s.gates[node.ID] = open
			if open {
				return map[string]any{"result": map[string]any{"open": true}}, []string{"out"}, nil
			}
			return map[string]any{"result": map[string]any{"open": false}}, nil, nil
		}
	case "flow:flip_flop":
		nextA := !s.flipFlops[node.ID]
		s.flipFlops[node.ID] = nextA
		if nextA {
			return map[string]any{"result": map[string]any{"output": "a"}}, []string{"a"}, nil
		}
		return map[string]any{"result": map[string]any{"output": "b"}}, []string{"b"}, nil
	case "flow:multi_gate":
		if execInput == "reset" {
			s.multiGates[node.ID] = 0
			return map[string]any{"result": map[string]any{"reset": true}}, nil, nil
		}
		index := s.multiGates[node.ID]
		ports := []string{"a", "b", "c"}
		if index >= len(ports) {
			if !boolConfig(node, "loop", false) {
				return map[string]any{"result": map[string]any{"complete": true}}, nil, nil
			}
			index = 0
		}
		s.multiGates[node.ID] = index + 1
		return map[string]any{"result": map[string]any{"index": index}}, []string{ports[index]}, nil
	case "flow:break":
		if s.loopDepth == 0 {
			return nil, nil, fmt.Errorf("break can only run inside a loop body")
		}
		s.breakRequested = true
		return map[string]any{"result": map[string]any{"break": true}}, nil, nil
	case "flow:set_variable":
		name := strings.TrimSpace(configText(node, "name"))
		if !variableName.MatchString(name) {
			return nil, nil, fmt.Errorf("variable name must start with a letter or underscore and contain only letters, numbers, or underscores")
		}
		s.variables[name] = inputs["value"]
		return map[string]any{"result": inputs["value"]}, []string{"out"}, nil
	case "flow:return", "function:return":
		s.result.Returned = true
		s.result.Value = Packet(cloneValues(inputs))
		return map[string]any{"result": cloneValues(inputs)}, nil, nil
	}

	// Existing impure nodes retain their hardened implementation. Data-pin values
	// replace their fields before the legacy action logic is invoked.
	config := cloneValues(definition.DefaultConfig)
	for key, value := range configFor(node) {
		config[key] = value
	}
	for key, value := range inputs {
		config[key] = value
	}
	copyNode := node
	copyNode.Data = map[string]any{"config": config}
	legacyResult, err := s.engine.executeNode(s.ctx, copyNode, Packet(inputs))
	if err != nil {
		return nil, nil, err
	}
	outputs := map[string]any{"result": map[string]any{}}
	ports := make([]string, 0, len(legacyResult))
	for port, packets := range legacyResult {
		if len(packets) == 0 {
			continue
		}
		outputs["result"] = packets[0]
		if outputIsExec(definition, port) {
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 && hasExecOutput(definition, "out") {
		ports = append(ports, "out")
	}
	return outputs, ports, nil
}

func (s *blueprintState) consumeBreak() bool {
	if !s.breakRequested {
		return false
	}
	s.breakRequested = false
	return true
}

func (s *blueprintState) resolveInputs(node domain.FlowNode, definition domain.NodeDefinition, frame *blueprintFrame) (map[string]any, error) {
	result := make(map[string]any)
	config := configFor(node)
	for _, pin := range definition.Inputs {
		if pin.Kind != domain.PinData {
			continue
		}
		value, found, err := s.resolveInput(node.ID, pin, frame)
		if err != nil {
			return result, err
		}
		if !found {
			value, found = config[pin.ID]
		}
		if !found && pin.Default != nil {
			value, found = pin.Default, true
		}
		if !found && pin.Required {
			return result, fmt.Errorf("node %q requires data pin %q", node.ID, pin.Label)
		}
		if found && !matchesDataType(value, pin.DataType) {
			return result, fmt.Errorf("node %q pin %q requires %s data", node.ID, pin.Label, pin.DataType)
		}
		if found {
			result[pin.ID] = value
		}
	}
	return result, nil
}

func (s *blueprintState) resolveInput(targetID string, pin domain.NodePort, frame *blueprintFrame) (any, bool, error) {
	for _, edge := range s.definition.Edges {
		if edge.Target != targetID || edge.TargetHandle != pin.ID || edgeKind(edge) != domain.PinData {
			continue
		}
		value, err := s.resolveData(edge.Source, edge.SourceHandle, frame)
		return value, true, err
	}
	return nil, false, nil
}

func (s *blueprintState) resolveData(nodeID, pinID string, frame *blueprintFrame) (any, error) {
	node, exists := s.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("data source node %q is missing", nodeID)
	}
	definition, exists := s.engine.registry.Get(node.Type)
	if !exists {
		return nil, fmt.Errorf("data source %q uses unavailable type %q", nodeID, node.Type)
	}
	definition, err := definitionForNode(definition, node)
	if err != nil {
		return nil, fmt.Errorf("node %q has invalid configuration: %w", node.ID, err)
	}
	if values, cached := frame.values[nodeID]; cached {
		value, exists := values[pinID]
		if !exists {
			return nil, fmt.Errorf("node %q has no cached output pin %q", nodeID, pinID)
		}
		return value, nil
	}
	if definition.Mode == domain.NodeImpure || definition.Mode == domain.NodeEvent {
		return nil, fmt.Errorf("node %q (%s) has an Exec pin but has not run on this execution path; cannot read data pin %q", nodeID, definition.Label, pinID)
	}
	if definition.Mode != domain.NodePure {
		return nil, fmt.Errorf("node %q cannot produce data", nodeID)
	}
	inputs, err := s.resolveInputs(node, definition, frame)
	if err != nil {
		return nil, err
	}
	started := time.Now().UTC()
	outputs, err := s.evaluatePure(node, inputs, frame)
	if err != nil {
		finished := time.Now().UTC()
		s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunFailed, Input: inputs, Error: err.Error(), StartedAt: started, FinishedAt: finished})
		return nil, err
	}
	finished := time.Now().UTC()
	frame.values[nodeID] = outputs
	frame.pure[nodeID] = true
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: inputs, Output: outputs, StartedAt: started, FinishedAt: finished})
	value, exists := outputs[pinID]
	if !exists {
		return nil, fmt.Errorf("pure node %q did not produce data pin %q", nodeID, pinID)
	}
	return value, nil
}

func (s *blueprintState) evaluatePure(node domain.FlowNode, inputs map[string]any, frame *blueprintFrame) (map[string]any, error) {
	config := configFor(node)
	switch node.Type {
	case "data:constant":
		value, exists := inputs["value"]
		if !exists {
			value = config["value"]
		}
		return map[string]any{"value": value}, nil
	case "data:format_text":
		text, err := renderTemplate(configText(node, "format"), inputs)
		if err != nil {
			return nil, err
		}
		return map[string]any{"text": text}, nil
	case "data:get_field", "data:break_object":
		definition, _ := s.engine.registry.Get(node.Type)
		configuredOutputs, err := getFieldOutputs(config, definition.DefaultConfig)
		if err != nil {
			return nil, err
		}
		outputs := make(map[string]any, len(configuredOutputs))
		for _, output := range configuredOutputs {
			value := valueAtAny(inputs["source"], output.Path)
			if !matchesDataType(value, output.DataType) {
				return nil, fmt.Errorf("%s output %q is declared %s, but %q contains %T", strings.ReplaceAll(node.Type, ":", " "), output.Label, output.DataType, output.Path, value)
			}
			outputs[output.ID] = value
		}
		return outputs, nil
	case "data:build_object":
		if _, legacy := config["fields"]; !legacy {
			key, ok := inputs["key"].(string)
			if !ok || strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("build object requires a non-empty Key")
			}
			return map[string]any{"object": map[string]any{key: inputs["value"]}}, nil
		}
		definition, _ := s.engine.registry.Get(node.Type)
		fields, err := objectFields(config, definition.DefaultConfig)
		if err != nil {
			return nil, err
		}
		object := make(map[string]any, len(fields))
		for _, field := range fields {
			if err := setObjectPath(object, field.Key, inputs[field.ID]); err != nil {
				return nil, fmt.Errorf("build object field %q: %w", field.Label, err)
			}
		}
		return map[string]any{"object": object}, nil
	case "data:cast":
		value, err := castValue(inputs["value"], configText(node, "target"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"value": value}, nil
	case "data:json_query":
		return map[string]any{"value": valueAtAny(inputs["source"], configText(node, "path"))}, nil
	case "data:equals":
		return map[string]any{"value": fmt.Sprint(inputs["left"]) == fmt.Sprint(inputs["right"])}, nil
	case "data:greater_than":
		left, leftOK := asNumber(inputs["left"])
		right, rightOK := asNumber(inputs["right"])
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("greater than requires numeric inputs")
		}
		return map[string]any{"value": left > right}, nil
	case "data:json_parse":
		var value any
		if err := json.Unmarshal([]byte(fmt.Sprint(inputs["text"])), &value); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		return map[string]any{"value": value}, nil
	case "data:get_variable":
		name := configText(node, "name")
		value, exists := s.variables[name]
		if !exists {
			return nil, fmt.Errorf("variable %q has not been set in this execution", name)
		}
		return map[string]any{"value": value}, nil
	case "data:chat_history":
		if s.engine.chat == nil {
			return nil, fmt.Errorf("chat history is unavailable for this execution")
		}
		chatID := strings.TrimSpace(fmt.Sprint(inputs["chatId"]))
		if chatID == "" {
			return nil, fmt.Errorf("read chat history requires a chat ID")
		}
		limit := 50
		if value, ok := asNumber(inputs["limit"]); ok {
			limit = int(value)
		}
		messages, err := s.engine.chat.ReadChatHistory(s.ctx, chatID, limit)
		if err != nil {
			return nil, fmt.Errorf("read chat history: %w", err)
		}
		values := make([]any, 0, len(messages))
		for _, message := range messages {
			values = append(values, map[string]any{"id": message.ID, "role": string(message.Role), "content": message.Content, "createdAt": message.CreatedAt.Format(time.RFC3339)})
		}
		return map[string]any{"messages": values}, nil
	case "data:reroute":
		return map[string]any{"value": inputs["value"]}, nil
	case "math:add", "math:subtract", "math:multiply", "math:divide":
		return evaluateMath(node.Type, inputs)
	case "date:now", "date:create", "date:extract", "date:format", "date:parse", "date:compare", "date:add", "date:subtract", "date:to_unix", "date:to_unix_ms":
		return evaluateDate(node.Type, inputs, config)
	}
	if strings.HasPrefix(node.Type, "function:") {
		return s.evaluateFunction(node, inputs, frame)
	}
	return nil, fmt.Errorf("node type %q is not a pure evaluator", node.Type)
}

func (s *blueprintState) evaluateFunction(node domain.FlowNode, inputs map[string]any, frame *blueprintFrame) (map[string]any, error) {
	return s.runFunction(node, inputs, frame)
}

func (s *blueprintState) runFunction(node domain.FlowNode, inputs map[string]any, frame *blueprintFrame) (map[string]any, error) {
	if s.engine.functions == nil {
		return nil, fmt.Errorf("custom functions are unavailable")
	}
	functionID := strings.TrimPrefix(node.Type, "function:")
	if functionID == "" {
		return nil, fmt.Errorf("invalid custom function node type %q", node.Type)
	}
	for _, active := range s.callStack {
		if active == functionID {
			return nil, fmt.Errorf("recursive custom function call %q is not allowed", functionID)
		}
	}
	function, err := s.engine.functions.GetPublishedFunction(s.ctx, functionID)
	if err != nil {
		return nil, err
	}
	definition, exists := s.engine.registry.Get(node.Type)
	if !exists {
		return nil, fmt.Errorf("custom function %q is not registered", function.Name)
	}
	if function.Mode != definition.Mode {
		return nil, fmt.Errorf("function %q changed execution mode; repair this call node", function.Name)
	}
	child := newBlueprintState(s.engine, s.ctx, function.DraftDefinition)
	child.variables, child.engine.variables = s.variables, s.variables
	child.callStack = append(append([]string{}, s.callStack...), functionID)
	child.functionID, child.functionRevision, child.parentNodeID, child.functionOutputs = functionID, function.PublishedRevision, node.ID, function.Outputs

	if function.Mode == domain.NodePure {
		for _, input := range function.DraftDefinition.Nodes {
			if input.Type == "function:input" {
				childFrame := newBlueprintFrame()
				childFrame.values[input.ID] = cloneValues(inputs)
				childFrame.pure[input.ID] = true
				outputs, err := child.resolvePureFunctionOutputs(childFrame)
				s.appendFunctionRuns(child)
				return outputs, err
			}
		}
		return nil, fmt.Errorf("pure function %q needs a Function Inputs node", function.Name)
	}

	entryID := ""
	for _, item := range function.DraftDefinition.Nodes {
		if item.Type == "function:entry" {
			entryID = item.ID
			break
		}
	}
	if entryID == "" {
		return nil, fmt.Errorf("function %q needs a Function Entry node", function.Name)
	}
	childFrame := newBlueprintFrame()
	childFrame.values[entryID] = cloneValues(inputs)
	started := time.Now().UTC()
	child.result.NodeRuns = append(child.result.NodeRuns, domain.NodeRun{NodeID: entryID, NodeType: "function:entry", Status: domain.RunCompleted, Input: inputs, Output: cloneValues(inputs), StartedAt: started, FinishedAt: time.Now().UTC()})
	err = child.follow(entryID, "out", childFrame)
	s.appendFunctionRuns(child)
	if err != nil {
		return nil, err
	}
	if !child.result.Returned {
		return nil, fmt.Errorf("function %q completed without reaching Function Return", function.Name)
	}
	return map[string]any(child.result.Value), nil
}

func (s *blueprintState) appendFunctionRuns(child *blueprintState) {
	for _, run := range child.result.NodeRuns {
		run.ParentNodeID, run.FunctionID, run.FunctionRevision = child.parentNodeID, child.functionID, child.functionRevision
		s.result.NodeRuns = append(s.result.NodeRuns, run)
	}
}

func (s *blueprintState) resolveFunctionOutputs(node domain.FlowNode, frame *blueprintFrame) (map[string]any, error) {
	if len(s.functionOutputs) == 0 {
		return map[string]any{}, nil
	}
	result := make(map[string]any, len(s.functionOutputs))
	for _, pin := range s.functionOutputs {
		value, found := any(nil), false
		for _, edge := range s.definition.Edges {
			if edge.Target != node.ID || edge.TargetHandle != pin.ID || edgeKind(edge) != domain.PinData {
				continue
			}
			resolved, err := s.resolveData(edge.Source, edge.SourceHandle, frame)
			if err != nil {
				return result, err
			}
			value, found = resolved, true
			break
		}
		if !found {
			return result, fmt.Errorf("function return is missing output pin %q", pin.Name)
		}
		if !matchesDataType(value, pin.DataType) {
			return result, fmt.Errorf("function return pin %q requires %s data", pin.Name, pin.DataType)
		}
		result[pin.ID] = value
	}
	return result, nil
}

func (s *blueprintState) resolvePureFunctionOutputs(frame *blueprintFrame) (map[string]any, error) {
	outputID := ""
	for _, node := range s.definition.Nodes {
		if node.Type == "function:output" {
			outputID = node.ID
			break
		}
	}
	if outputID == "" {
		return nil, fmt.Errorf("pure function needs a Function Outputs node")
	}
	return s.resolveFunctionOutputs(domain.FlowNode{ID: outputID}, frame)
}

func (s *blueprintState) recordFailure(node domain.FlowNode, input any, err error) error {
	return s.recordFailureAt(node, input, err, time.Now().UTC(), time.Now().UTC())
}
func (s *blueprintState) recordFailureAt(node domain.FlowNode, input any, err error, started, finished time.Time) error {
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunFailed, Input: input, Error: err.Error(), StartedAt: started, FinishedAt: finished})
	return fmt.Errorf("execute %s: %w", node.Type, err)
}

func edgeKind(edge domain.FlowEdge) domain.PinKind {
	if edge.Kind != "" {
		return edge.Kind
	}
	return domain.PinExec
}
func outputIsExec(definition domain.NodeDefinition, id string) bool {
	for _, pin := range definition.Outputs {
		if pin.ID == id {
			return pin.Kind == domain.PinExec
		}
	}
	return false
}
func hasExecOutput(definition domain.NodeDefinition, id string) bool {
	return outputIsExec(definition, id)
}
func asSlice(value any) ([]any, bool) { values, ok := value.([]any); return values, ok }
func asNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	case string:
		value, err := strconv.ParseFloat(typed, 64)
		return value, err == nil
	default:
		return 0, false
	}
}
func asInteger(value any) (int, bool) {
	number, ok := asNumber(value)
	return int(number), ok && number == float64(int(number))
}
func castValue(value any, target string) (any, error) {
	switch target {
	case "text":
		return fmt.Sprint(value), nil
	case "number":
		number, ok := asNumber(value)
		if !ok {
			return nil, fmt.Errorf("cannot cast %T to number", value)
		}
		return number, nil
	case "boolean":
		if result, ok := value.(bool); ok {
			return result, nil
		}
		if result, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value))); err == nil {
			return result, nil
		}
		return nil, fmt.Errorf("cannot cast %T to Boolean", value)
	default:
		return nil, fmt.Errorf("unknown cast target %q", target)
	}
}
func boolConfig(node domain.FlowNode, key string, fallback bool) bool {
	value, exists := configFor(node)[key]
	if !exists {
		return fallback
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	boolean, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value)))
	return err == nil && boolean
}
func matchesDataType(value any, dataType domain.DataType) bool {
	if value == nil || dataType == "" || dataType == domain.DataAny {
		return true
	}
	switch dataType {
	case domain.DataText:
		_, ok := value.(string)
		return ok
	case domain.DataBoolean:
		_, ok := value.(bool)
		return ok
	case domain.DataNumber:
		_, ok := asNumber(value)
		return ok
	case domain.DataObject:
		return isObjectValue(value)
	case domain.DataList:
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}
func configText(node domain.FlowNode, key string) string {
	value, _ := configFor(node)[key].(string)
	return strings.TrimSpace(value)
}

func renderTemplate(format string, data any) (string, error) {
	tmpl, err := template.New("format").Parse(format)
	if err != nil {
		return "", fmt.Errorf("incorrect format template: %w", err)
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("unable to execute template: %w", err)
	}

	return out.String(), nil
}

func valueAtAny(value any, path string) any {
	if strings.TrimSpace(path) == "" {
		return value
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		if next, found := objectValueAt(current, part); found {
			current = next
			continue
		}
		next, found := listValueAt(current, part)
		if !found {
			return nil
		}
		current = next
	}
	return current
}

// isObjectValue intentionally accepts JSON-like named maps (including
// http.Header), structs, and pointers to either. Plugins and Go's standard
// library regularly return named object types rather than map[string]any.
func isObjectValue(value any) bool {
	resolved := dereferenceValue(reflect.ValueOf(value))
	if !resolved.IsValid() {
		return false
	}
	return resolved.Kind() == reflect.Struct ||
		(resolved.Kind() == reflect.Map && resolved.Type().Key().Kind() == reflect.String)
}

func objectValueAt(value any, key string) (any, bool) {
	resolved := dereferenceValue(reflect.ValueOf(value))
	if !resolved.IsValid() {
		return nil, false
	}
	switch resolved.Kind() {
	case reflect.Map:
		if resolved.Type().Key().Kind() != reflect.String {
			return nil, false
		}
		mapKey := reflect.New(resolved.Type().Key()).Elem()
		mapKey.SetString(key)
		item := resolved.MapIndex(mapKey)
		if !item.IsValid() || !item.CanInterface() {
			return nil, false
		}
		return item.Interface(), true
	case reflect.Struct:
		for index := 0; index < resolved.NumField(); index++ {
			field := resolved.Type().Field(index)
			if field.PkgPath != "" || field.Name == "" {
				continue
			}
			jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
			matchesName := key == field.Name || key == strings.ToLower(field.Name) ||
				(jsonName != "" && jsonName != "-" && key == jsonName)
			if !matchesName {
				continue
			}
			item := resolved.Field(index)
			if !item.CanInterface() {
				return nil, false
			}
			return item.Interface(), true
		}
	}
	return nil, false
}

func listValueAt(value any, key string) (any, bool) {
	index, err := strconv.Atoi(key)
	if err != nil || index < 0 {
		return nil, false
	}
	resolved := dereferenceValue(reflect.ValueOf(value))
	if !resolved.IsValid() || (resolved.Kind() != reflect.Slice && resolved.Kind() != reflect.Array) || index >= resolved.Len() {
		return nil, false
	}
	item := resolved.Index(index)
	if !item.CanInterface() {
		return nil, false
	}
	return item.Interface(), true
}

func dereferenceValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func setObjectPath(object map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	current := object
	for index, part := range parts {
		if index == len(parts)-1 {
			current[part] = value
			return nil
		}
		next, exists := current[part]
		if !exists {
			child := make(map[string]any)
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("key path conflicts at %q", strings.Join(parts[:index+1], "."))
		}
		current = child
	}
	return nil
}
