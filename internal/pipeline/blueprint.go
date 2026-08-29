package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

const maxBlueprintLoopIterations = 10_000

// Execute runs only the versioned Blueprint graph format. Legacy graphs are
// preserved by persistence but intentionally cannot reach this interpreter.
func (e *Engine) Execute(ctx context.Context, definition domain.FlowDefinition, triggerNodeID string, initial Packet) (RunResult, error) {
	if definition.SchemaVersion != domain.GraphSchemaV2 && definition.SchemaVersion != domain.GraphSchemaV3 {
		return RunResult{}, ValidationError{Message: "this is a legacy pipeline. Rebuild it as a Blueprint v3 graph before running."}
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
	var err error
	definition, err = definitionForRegisteredNode(s.engine.registry, definition, node)
	if err != nil {
		return fmt.Errorf("selected trigger %q has invalid configuration: %w", nodeID, err)
	}
	if definition.Mode != domain.NodeEvent {
		return fmt.Errorf("node %q is not an event trigger", nodeID)
	}
	frame := newBlueprintFrame()
	if module, registered := s.engine.registry.Node(node.Type); registered {
		result, executeErr := module.Execute(s.ctx, nodes.Invocation{Node: node, Definition: definition, SchemaVersion: s.definition.SchemaVersion, Config: configFor(node), Inputs: map[string]any{"event": initial["event"]}}, s)
		if executeErr != nil {
			return s.recordFailure(node, map[string]any{"event": initial["event"]}, executeErr)
		}
		frame.values[node.ID] = result.Outputs
		if err := s.recordEvent(node, frame.values[node.ID]); err != nil {
			return err
		}
		for _, port := range result.Ports {
			if err := s.follow(node.ID, port, frame); err != nil {
				return err
			}
		}
		return nil
	}
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
	definition, err := definitionForRegisteredNode(s.engine.registry, definition, node)
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

	started := time.Now().UTC()
	outputs, ports, loop, err := s.executeImpure(node, definition, inputs, frame, execInput)
	finished := time.Now().UTC()
	if err != nil {
		return s.recordFailureAt(node, inputs, err, started, finished)
	}
	if loop != nil {
		return s.runLoopPlan(node, definition, inputs, frame, loop, started)
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

// runLoopPlan owns graph traversal, frame isolation, cancellation, and loop
// safety. The node module supplies only its iterations or Boolean condition
// contract through nodes.LoopPlan.
func (s *blueprintState) runLoopPlan(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame, plan *nodes.LoopPlan, started time.Time) error {
	s.loopDepth++
	defer func() { s.loopDepth-- }()
	count := 0
	runIteration := func(values map[string]any) error {
		count++
		if count > maxBlueprintLoopIterations {
			return fmt.Errorf("loop exceeded the %d iteration limit", maxBlueprintLoopIterations)
		}
		child := frame.loopChild()
		child.values[node.ID] = cloneValues(values)
		if err := s.follow(node.ID, "loop", child); err != nil {
			return err
		}
		if s.result.Returned || s.consumeBreak() {
			return errLoopStopped
		}
		return nil
	}

	if plan.Continue != nil {
		for {
			child := frame.loopChild()
			iterationInputs, err := s.resolveInputs(node, definition, child)
			if err != nil {
				return s.recordFailureAt(node, nil, err, started, time.Now().UTC())
			}
			continueLoop, err := plan.Continue(iterationInputs)
			if err != nil {
				return s.recordFailureAt(node, iterationInputs, err, started, time.Now().UTC())
			}
			if !continueLoop {
				break
			}
			values := map[string]any{"result": map[string]any{"iteration": count + 1}}
			if err := runIteration(values); err != nil {
				if err == errLoopStopped {
					break
				}
				return s.recordFailureAt(node, iterationInputs, err, started, time.Now().UTC())
			}
		}
	} else {
		for _, values := range plan.Iterations {
			if err := runIteration(values); err != nil {
				if err == errLoopStopped {
					break
				}
				return s.recordFailureAt(node, inputs, err, started, time.Now().UTC())
			}
		}
	}

	reportedCount := count
	if plan.ReportedCount >= 0 {
		reportedCount = plan.ReportedCount
	}
	outputs := map[string]any{"result": map[string]any{"count": reportedCount}}
	frame.values[node.ID] = outputs
	finished := time.Now().UTC()
	s.result.NodeRuns = append(s.result.NodeRuns, domain.NodeRun{NodeID: node.ID, NodeType: node.Type, Status: domain.RunCompleted, Input: inputs, Output: outputs, StartedAt: started, FinishedAt: finished})
	return s.follow(node.ID, "completed", frame)
}

var errLoopStopped = fmt.Errorf("loop stopped")

// SQLExecutor exposes only the database operation required by action:sql.
func (s *blueprintState) SQLExecutor() nodes.SQLExecutor { return s.engine.databases }

// KVExecutor exposes only the key/value operation required by KV nodes.
func (s *blueprintState) KVExecutor() nodes.KVExecutor { return s.engine.kv }

// StorageExecutor exposes only the storage operations required by storage
// nodes.
func (s *blueprintState) StorageExecutor() nodes.StorageExecutor { return s.engine.storage }

func (s *blueprintState) executeImpure(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame, execInput string) (map[string]any, []string, *nodes.LoopPlan, error) {
	if strings.HasPrefix(node.Type, "function:") && node.Type != "function:return" && node.Type != "function:entry" {
		outputs, err := s.runFunction(node, inputs, frame)
		return outputs, []string{"out"}, nil, err
	}
	if node.Type == "llm:agent" || node.Type == "llm:coding_agent" {
		outputs, err := s.executeConnectedToolAgent(node, configFor(node), inputs)
		if err != nil || outputs != nil {
			if outputs == nil {
				return nil, nil, nil, err
			}
			return map[string]any{"result": outputs}, []string{"out"}, nil, err
		}
	}
	if module, exists := s.engine.registry.Node(node.Type); exists {
		result, err := module.Execute(s.ctx, nodes.Invocation{
			Node:            node,
			Definition:      definition,
			SchemaVersion:   s.definition.SchemaVersion,
			ExecInput:       execInput,
			Config:          configFor(node),
			Inputs:          inputs,
			ConnectedInputs: s.connectedInputs(node.ID),
		}, s)
		return result.Outputs, result.Ports, result.Loop, err
	}
	switch node.Type {
	case "function:return":
		s.result.Returned = true
		s.result.Value = Packet(cloneValues(inputs))
		return map[string]any{"result": cloneValues(inputs)}, nil, nil, nil
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
		return nil, nil, nil, err
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
	return outputs, ports, nil, nil
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
		fromConfiguration := false
		if !found && !pin.IgnoreConfigFallback {
			value, found = config[pin.ID]
			fromConfiguration = found
		}
		if !found && pin.Default != nil {
			value, found = pin.Default, true
			fromConfiguration = true
		}
		if !found && pin.Required {
			return result, fmt.Errorf("node %q requires data pin %q", node.ID, pin.Label)
		}
		if found && fromConfiguration && s.definition.SchemaVersion >= domain.GraphSchemaV3 && pin.Type != nil {
			var canonicalErr error
			value, canonicalErr = canonicalConfigurationValue(value, *pin.Type)
			if canonicalErr != nil {
				return result, fmt.Errorf("node %q pin %q: %w", node.ID, pin.Label, canonicalErr)
			}
		}
		if found && s.definition.SchemaVersion >= domain.GraphSchemaV3 && pin.Type != nil {
			if err := typespec.ValidateValue(value, *pin.Type); err != nil {
				return result, fmt.Errorf("node %q pin %q: %w", node.ID, pin.Label, err)
			}
		} else if found && !matchesDataType(value, pin.DataType) {
			return result, fmt.Errorf("node %q pin %q requires %s data", node.ID, pin.Label, pin.DataType)
		}
		if found {
			result[pin.ID] = value
		}
	}
	return result, nil
}

// canonicalConfigurationValue is the JSON/Wails boundary for persisted
// inspector values. JSON represents all numbers as a single number token, so
// an integral float64 is reified as a Go int only when the *declared config
// pin* is Int. Wired data never takes this path: a Float wire remains a Float
// and is rejected by an Int pin without an explicit Cast node.
func canonicalConfigurationValue(value any, target domain.TypeSpec) (any, error) {
	if target.Kind != domain.TypeInt {
		return value, nil
	}
	var number float64
	switch typed := value.(type) {
	case int:
		return typed, nil
	case float64:
		number = typed
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return nil, fmt.Errorf("must be an integer")
		}
		number = parsed
	default:
		return value, nil
	}
	// A number arriving through a JSON/Wails any-map is a float64. Keep the
	// conversion inside its exact IEEE-754 integer range on 64-bit builds.
	max := float64(9_007_199_254_740_991)
	if strconv.IntSize == 32 {
		max = 2_147_483_647
	}
	min := -max - 1
	if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < min || number > max {
		return nil, fmt.Errorf("must be a finite integer")
	}
	return int(number), nil
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

func (s *blueprintState) connectedInputs(targetID string) map[string]bool {
	connected := make(map[string]bool)
	for _, edge := range s.definition.Edges {
		if edge.Target == targetID && edgeKind(edge) == domain.PinData {
			connected[edge.TargetHandle] = true
		}
	}
	return connected
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
	definition, err := definitionForRegisteredNode(s.engine.registry, definition, node)
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
	outputs, err := s.evaluatePure(node, definition, inputs, frame)
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

func (s *blueprintState) evaluatePure(node domain.FlowNode, definition domain.NodeDefinition, inputs map[string]any, frame *blueprintFrame) (map[string]any, error) {
	config := configFor(node)
	if module, exists := s.engine.registry.Node(node.Type); exists {
		result, err := module.Execute(s.ctx, nodes.Invocation{
			Node:            node,
			Definition:      definition,
			SchemaVersion:   s.definition.SchemaVersion,
			Config:          config,
			Inputs:          inputs,
			ConnectedInputs: s.connectedInputs(node.ID),
		}, s)
		if err != nil {
			return nil, err
		}
		if len(result.Ports) != 0 || result.Loop != nil {
			return nil, fmt.Errorf("pure node type %q attempted to control execution", node.Type)
		}
		return result.Outputs, nil
	}
	switch node.Type {
	}
	if strings.HasPrefix(node.Type, "function:") {
		return s.evaluateFunction(node, inputs, frame)
	}
	return nil, fmt.Errorf("node type %q is not a pure evaluator", node.Type)
}

func (s *blueprintState) evaluateFunction(node domain.FlowNode, inputs map[string]any, frame *blueprintFrame) (map[string]any, error) {
	return s.runFunction(node, inputs, frame)
}

// LookupVariable implements nodes.VariableReader without exposing the graph
// interpreter's mutable packet to node modules.
func (s *blueprintState) LookupVariable(name string) (any, bool) {
	value, exists := s.variables[name]
	return value, exists
}

// ReadChatHistory implements nodes.ChatHistoryReader through the engine's
// application-provided chat service.
func (s *blueprintState) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	if s.engine.chat == nil {
		return nil, fmt.Errorf("chat history is unavailable for this execution")
	}
	return s.engine.chat.ReadChatHistory(ctx, chatID, limit)
}

// ClaimOnce implements nodes.OnceStore.
func (s *blueprintState) ClaimOnce(nodeID string) bool {
	if s.once[nodeID] {
		return false
	}
	s.once[nodeID] = true
	return true
}

// ResetOnce implements nodes.OnceStore.
func (s *blueprintState) ResetOnce(nodeID string) { delete(s.once, nodeID) }

// GateOpen implements nodes.GateStore.
func (s *blueprintState) GateOpen(nodeID string) (bool, bool) {
	value, exists := s.gates[nodeID]
	return value, exists
}

// SetGateOpen implements nodes.GateStore.
func (s *blueprintState) SetGateOpen(nodeID string, open bool) { s.gates[nodeID] = open }

// NextFlipFlop implements nodes.FlipFlopStore.
func (s *blueprintState) NextFlipFlop(nodeID string) bool {
	next := !s.flipFlops[nodeID]
	s.flipFlops[nodeID] = next
	return next
}

// MultiGateIndex implements nodes.MultiGateStore.
func (s *blueprintState) MultiGateIndex(nodeID string) int { return s.multiGates[nodeID] }

// SetMultiGateIndex implements nodes.MultiGateStore.
func (s *blueprintState) SetMultiGateIndex(nodeID string, index int) { s.multiGates[nodeID] = index }

// InLoop implements nodes.LoopController.
func (s *blueprintState) InLoop() bool { return s.loopDepth > 0 }

// RequestBreak implements nodes.LoopController.
func (s *blueprintState) RequestBreak() { s.breakRequested = true }

// StoreVariable implements nodes.VariableWriter.
func (s *blueprintState) StoreVariable(name string, value any) { s.variables[name] = value }

// ReadGlobalVariable implements nodes.GlobalVariableReader. The engine
// consults a workspace-scoped store; the call blocks at the global store's
// own lock so concurrent pipelines see a consistent snapshot.
func (s *blueprintState) ReadGlobalVariable(name string) (any, bool) {
	if s.engine.globals == nil {
		return nil, false
	}
	value, err := s.engine.globals.Read(name)
	if err != nil {
		return nil, false
	}
	return value, true
}

// WriteGlobalVariable implements nodes.GlobalVariableWriter.
func (s *blueprintState) WriteGlobalVariable(name string, value any) error {
	if s.engine.globals == nil {
		return fmt.Errorf("global variables are unavailable for this execution")
	}
	return s.engine.globals.Set(name, value)
}

// IncrementGlobalVariable implements nodes.GlobalVariableWriter with atomicity.
func (s *blueprintState) IncrementGlobalVariable(name string, delta float64) (float64, error) {
	if s.engine.globals == nil {
		return 0, fmt.Errorf("global variables are unavailable for this execution")
	}
	return s.engine.globals.Increment(name, delta)
}

// AppendGlobalVariable implements nodes.GlobalVariableWriter with atomicity.
func (s *blueprintState) AppendGlobalVariable(name string, item any) ([]any, error) {
	if s.engine.globals == nil {
		return nil, fmt.Errorf("global variables are unavailable for this execution")
	}
	return s.engine.globals.Append(name, item)
}

// DeleteVariable implements nodes.VariableStore for JavaScript's scoped
// variable API. This only affects the active Blueprint execution.
func (s *blueprintState) DeleteVariable(name string) { delete(s.variables, name) }

// JavaScriptHost implements nodes.JavaScriptHostProvider without exposing the
// concrete graph interpreter to node modules.
func (s *blueprintState) JavaScriptHost() nodes.JavaScriptHost {
	return s.engine.javascript
}

// TwitchChatSender implements the focused runtime port consumed by the Twitch
// action node without exposing the concrete engine or desktop service.
func (s *blueprintState) TwitchChatSender() nodes.TwitchChatSender { return s.engine.twitch }

// DiscordSender implements the focused runtime port consumed by the Discord
// action nodes without exposing the concrete engine or gateway service.
func (s *blueprintState) DiscordSender() nodes.DiscordSender { return s.engine.discord }

// TelegramSender implements the focused runtime port consumed by the Telegram
// action nodes without exposing the concrete engine or polling service.
func (s *blueprintState) TelegramSender() nodes.TelegramSender { return s.engine.telegram }

// DialogOpener implements the focused runtime port consumed by Display Message
// and Display Question nodes without exposing the concrete engine or dialog
// service to node modules.
func (s *blueprintState) DialogOpener() nodes.DialogOpener { return s.engine.dialogs }

// InputDialogOpener implements the focused runtime port consumed by the
// Display Input Dialog node.
func (s *blueprintState) InputDialogOpener() nodes.InputDialogOpener { return s.engine.inputDialogs }

// FormDialogOpener implements the focused runtime port consumed by the Form
// node.
func (s *blueprintState) FormDialogOpener() nodes.FormDialogOpener { return s.engine.formDialogs }

// Return implements nodes.ReturnSignaler.
func (s *blueprintState) Return(value map[string]any) {
	s.result.Returned = true
	s.result.Value = Packet(cloneValues(value))
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
	if function.Kind != domain.FunctionTool && function.Mode != definition.Mode {
		return nil, fmt.Errorf("function %q changed execution mode; repair this call node", function.Name)
	}
	inputs, err = functionCallInputs(function, inputs)
	if err != nil {
		return nil, err
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
		if s.definition.SchemaVersion >= domain.GraphSchemaV3 {
			if err := typespec.ValidateValue(value, functionPinType(pin)); err != nil {
				return result, fmt.Errorf("function return pin %q: %w", pin.Name, err)
			}
		} else if !matchesDataType(value, pin.DataType) {
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
	default:
		return 0, false
	}
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
