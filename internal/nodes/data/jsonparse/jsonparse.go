package jsonparse

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:json_parse", "Data", "Parse JSON", "Parse JSON text into an object or list.", "file-json", []domain.NodePort{datanodes.Pin("text", "Text", domain.PinInput, domain.DataText)}, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, nil, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate decodes JSON text into graph-safe JSON values.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	text, ok := invocation.Inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("parse JSON requires text input")
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return map[string]any{"value": value}, nil
}
