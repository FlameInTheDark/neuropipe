package pipeline

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datenodes "github.com/FlameInTheDark/neuropipe/internal/nodes/date"
)

// evaluateDate is a test compatibility bridge. Date behavior is implemented
// by internal/nodes/date and exercised through the same handler contract as
// Blueprint execution.
func evaluateDate(nodeType string, inputs map[string]any, config map[string]any) (map[string]any, error) {
	module, exists := datenodes.Find(nodeType)
	if !exists {
		return nil, fmt.Errorf("unsupported date node %q", nodeType)
	}
	result, err := module.Execute(context.Background(), nodes.Invocation{Node: domain.FlowNode{Type: nodeType}, Definition: module.Definition(), Config: config, Inputs: inputs}, nil)
	return result.Outputs, err
}
