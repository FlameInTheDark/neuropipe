package breakobject

import (
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getfield"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	defaults := []any{map[string]any{"id": "value", "label": "Value", "path": "value", "dataType": "any"}}
	definition := datanodes.Node("data:break_object", "Data", "Break Object", "Split configured object key paths into typed output pins.", "unfold-vertical", []domain.NodePort{datanodes.Pin("source", "Source", domain.PinInput, domain.DataObject)}, nil, []domain.ConfigField{datanodes.FieldOutputs("outputs", "Outputs")}, map[string]any{"outputs": defaults})
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return getfield.ResolveDefinition(definition, node)
	}, Executor: nodes.Outputs(getfield.Evaluate)})
}
