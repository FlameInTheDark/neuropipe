package foreach

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	items := flow.Data("items", "Array", domain.PinInput, domain.DataAny)
	items.DataType = domain.DataList
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), items}
	outputs := []domain.NodePort{flow.Exec("loop", "Loop Body", domain.PinOutput), flow.Exec("completed", "Completed", domain.PinOutput), flow.Data("item", "Array Element", domain.PinOutput, domain.DataAny), flow.Data("index", "Array Index", domain.PinOutput, domain.DataNumber)}
	return registrar.Register(Node{Metadata: flow.Node("flow:for_each", "Flow", "For Each Loop", "Run Loop Body for every item, then Completed.", "repeat-2", inputs, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute converts an array into immutable iteration outputs; the engine owns
// following the loop port and enforcing its safety limit.
func Execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	items, ok := invocation.Inputs["items"].([]any)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("for-each loop expects Array to be a list")
	}
	iterations := make([]map[string]any, 0, len(items))
	for index, item := range items {
		iterations = append(iterations, map[string]any{"item": item, "index": float64(index), "result": map[string]any{"item": item, "index": float64(index)}})
		if len(iterations) > 10_000 {
			break
		}
	}
	return nodes.ExecutionResult{Loop: &nodes.LoopPlan{Iterations: iterations, ReportedCount: len(items)}}, nil
}
