package formattext

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:format_text", "Data", "Format Text", "Format text with an explicit Value data pin.", "text", []domain.NodePort{datanodes.Pin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{datanodes.Pin("text", "Text", domain.PinOutput, domain.DataText)}, []domain.ConfigField{datanodes.Field("format", "Format", "string", "{.value}", true)}, map[string]any{"format": "{.value}"}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate renders this node's format string against its input object.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	format, _ := invocation.Config["format"].(string)
	templateValue, err := template.New("format").Parse(format)
	if err != nil {
		return nil, fmt.Errorf("incorrect format template: %w", err)
	}
	var output strings.Builder
	if err := templateValue.Execute(&output, invocation.Inputs); err != nil {
		return nil, fmt.Errorf("unable to execute template: %w", err)
	}
	return map[string]any{"text": output.String()}, nil
}
