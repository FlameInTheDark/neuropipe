package gate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{flow.Exec("in", "Enter", domain.PinInput), flow.Exec("open", "Open", domain.PinInput), flow.Exec("close", "Close", domain.PinInput), flow.Exec("toggle", "Toggle", domain.PinInput)}
	fields := []domain.ConfigField{flow.Field("startOpen", "Start open", "boolean", "true", false)}
	return registrar.Register(Node{Metadata: flow.Node("flow:gate", "Flow", "Gate", "Route execution only while the gate is open.", "door-open", inputs, []domain.NodePort{flow.Exec("out", "Then", domain.PinOutput)}, fields, map[string]any{"startOpen": true}), Executor: Execute})
}

// Execute owns Gate transitions while storing the mutable bit behind a narrow
// GateStore interface supplied by the graph host.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	store, ok := runtime.(nodes.GateStore)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("gate runtime is unavailable")
	}
	open, configured := store.GateOpen(invocation.Node.ID)
	if !configured {
		open = configuredBool(invocation.Config["startOpen"], invocation.SchemaVersion, true)
	}
	switch invocation.ExecInput {
	case "open":
		store.SetGateOpen(invocation.Node.ID, true)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"open": true}}}, nil
	case "close":
		store.SetGateOpen(invocation.Node.ID, false)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"open": false}}}, nil
	case "toggle":
		store.SetGateOpen(invocation.Node.ID, !open)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"open": !open}}}, nil
	default:
		store.SetGateOpen(invocation.Node.ID, open)
		if open {
			return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"open": true}}, Ports: []string{"out"}}, nil
		}
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"open": false}}}, nil
	}
}

func configuredBool(value any, schemaVersion int, fallback bool) bool {
	if value == nil {
		return fallback
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	if schemaVersion >= domain.GraphSchemaV3 {
		return fallback
	}
	boolean, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value)))
	return err == nil && boolean
}
