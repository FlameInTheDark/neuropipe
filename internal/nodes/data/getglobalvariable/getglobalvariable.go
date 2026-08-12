// Package getglobalvariable registers Get Global Variable, the read half of
// the workspace-scoped Global Variables feature. It deliberately sits in the
// Data category as a pure node: reading shared state does not change it, and
// the lock-protected store guarantees a stable snapshot.
package getglobalvariable

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	definition := datanodes.Node("data:get_global_variable", "Data", "Get Global Variable", "Read a workspace variable shared across pipelines and runs.", "database",
		nil,
		[]domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)},
		[]domain.ConfigField{{Name: "name", Label: "Variable", Kind: "select", Required: true}},
		map[string]any{"name": ""},
	)
	return registrar.Register(Node{Metadata: definition, Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
		return resolve(definition, node), nil
	}, Executor: nodes.Outputs(Evaluate)})
}

// resolve rewrites the output pin to the variable's declared type and fills
// the name picklist from the current declarations. Unresolved configs keep the
// generic Any pin; the executor fails them explicitly at runtime, so the
// catalogue never claims a type the host cannot honour.
func resolve(definition domain.NodeDefinition, node domain.FlowNode) domain.NodeDefinition {
	resolved := definition
	resolved.Fields = injectOptions(definition.Fields)
	name, _ := node.Data["config"].(map[string]any)["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return resolved
	}
	resolved.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
	if declaredType != nil {
		if declared, ok := declaredType(name); ok {
			for index := range resolved.Outputs {
				resolved.Outputs[index].DataType = declared
				typeSpec := typespec.FromDataType(declared)
				resolved.Outputs[index].Type = &typeSpec
			}
		}
	}
	return resolved
}

// declaredType and declaredOptions are injected by the variables service via
// Desktop composition. Keeping them behind package-level hooks keeps node
// metadata static while still letting the resolver react to the latest
// declared types and names.
var (
	declaredType    func(name string) (domain.DataType, bool)
	declaredOptions func() []domain.Option
)

// SetDeclaredType wires the variable type catalogue into the resolver.
func SetDeclaredType(resolver func(name string) (domain.DataType, bool)) {
	declaredType = resolver
}

// SetDeclaredOptions wires the variable name picklist into the resolver.
func SetDeclaredOptions(source func() []domain.Option) {
	declaredOptions = source
}

// injectOptions refreshes the name select from the live declaration list so
// resolved editor definitions carry the same picklist as the palette catalog.
func injectOptions(fields []domain.ConfigField) []domain.ConfigField {
	cloned := append([]domain.ConfigField(nil), fields...)
	if declaredOptions == nil {
		return cloned
	}
	options := append([]domain.Option(nil), declaredOptions()...)
	for index, field := range cloned {
		if field.Name == "name" && field.Kind == "select" {
			field.Options = options
			cloned[index] = field
		}
	}
	return cloned
}

// Evaluate reads through the narrow GlobalVariableReader capability. A miss
// means the variable was deleted mid-run or never existed, which is a hard
// node failure: the pipeline misconfiguration must be visible, not silent.
func Evaluate(_ context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (map[string]any, error) {
	globals, ok := runtime.(nodes.GlobalVariableReader)
	if !ok {
		return nil, fmt.Errorf("global variable runtime is unavailable")
	}
	name, _ := invocation.Config["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("select a variable to read")
	}
	value, exists := globals.ReadGlobalVariable(name)
	if !exists {
		return nil, fmt.Errorf("global variable %q is not declared or has been deleted", name)
	}
	return map[string]any{"value": value}, nil
}
