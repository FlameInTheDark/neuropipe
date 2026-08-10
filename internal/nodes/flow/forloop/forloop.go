package forloop

import (
	"context"
	"fmt"
	"math"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	flow "github.com/FlameInTheDark/neuropipe/internal/nodes/flow"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{flow.Exec("in", "Exec", domain.PinInput), flow.Data("first", "First Index", domain.PinInput, domain.DataNumber), flow.Data("last", "Last Index", domain.PinInput, domain.DataNumber)}
	outputs := []domain.NodePort{flow.Exec("loop", "Loop Body", domain.PinOutput), flow.Exec("completed", "Completed", domain.PinOutput), flow.Data("index", "Index", domain.PinOutput, domain.DataNumber)}
	return registrar.Register(Node{Metadata: flow.Node("flow:for_loop", "Flow", "For Loop", "Run Loop Body between inclusive numeric bounds.", "repeat", inputs, outputs, nil, map[string]any{}), Executor: Execute})
}

// Execute builds the inclusive index range and leaves graph traversal to the
// host. A hard runtime loop limit is still enforced by the host.
func Execute(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	first, firstOK := integer(invocation.Inputs["first"])
	last, lastOK := integer(invocation.Inputs["last"])
	if !firstOK || !lastOK {
		return nodes.ExecutionResult{}, fmt.Errorf("for loop expects numeric First Index and Last Index")
	}
	iterations := make([]map[string]any, 0)
	for index := first; index <= last; index++ {
		iterations = append(iterations, map[string]any{"index": float64(index), "result": map[string]any{"index": float64(index)}})
		if index == math.MaxInt || len(iterations) > 10_000 {
			break
		}
	}
	return nodes.ExecutionResult{Loop: &nodes.LoopPlan{Iterations: iterations, ReportedCount: -1}}, nil
}

func integer(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), !math.IsNaN(number) && !math.IsInf(number, 0) && number == math.Trunc(number) && number >= float64(math.MinInt) && number <= float64(math.MaxInt)
	case int:
		return number, true
	case int64:
		return int(number), int64(int(number)) == number
	default:
		return 0, false
	}
}
