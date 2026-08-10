package multigate

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
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Exec("reset", "Reset", domain.PinInput)}
	outputs := []domain.NodePort{flow.Exec("a", "A", domain.PinOutput), flow.Exec("b", "B", domain.PinOutput), flow.Exec("c", "C", domain.PinOutput)}
	return registrar.Register(Node{Metadata: flow.Node("flow:multi_gate", "Flow", "MultiGate", "Cycle through A, B, and C outputs.", "git-fork", inputs, outputs, []domain.ConfigField{flow.Field("loop", "Loop", "boolean", "false", false)}, map[string]any{"loop": false}), Executor: Execute})
}

// Execute cycles named ports and delegates only its index storage to the host.
func Execute(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	store, ok := runtime.(nodes.MultiGateStore)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("multi gate runtime is unavailable")
	}
	if invocation.ExecInput == "reset" {
		store.SetMultiGateIndex(invocation.Node.ID, 0)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"reset": true}}}, nil
	}
	ports := []string{"a", "b", "c"}
	index := store.MultiGateIndex(invocation.Node.ID)
	if index >= len(ports) {
		if !configuredBool(invocation.Config["loop"], invocation.SchemaVersion) {
			return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"complete": true}}}, nil
		}
		index = 0
	}
	store.SetMultiGateIndex(invocation.Node.ID, index+1)
	return nodes.ExecutionResult{Outputs: map[string]any{"result": map[string]any{"index": index}}, Ports: []string{ports[index]}}, nil
}

func configuredBool(value any, schemaVersion int) bool {
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	if schemaVersion >= domain.GraphSchemaV3 {
		return false
	}
	boolean, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(value)))
	return err == nil && boolean
}
