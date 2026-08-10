package catalog

import "github.com/FlameInTheDark/neuropipe/internal/domain"

func blueprintFunctionBuiltins() []domain.NodeDefinition {
	return []domain.NodeDefinition{
		blueprintNode("function:entry", "Functions", "Function Entry", "The single entry point for an impure custom function.", "log-in", "#a78bfa", domain.NodeEvent, nil, execOutput(), nil, map[string]any{}),
		blueprintNode("function:return", "Functions", "Function Return", "The single return point for an impure custom function.", "log-out", "#a78bfa", domain.NodeImpure, execInput(), nil, nil, map[string]any{}),
		blueprintNode("function:input", "Functions", "Function Inputs", "Typed inputs supplied by a pure custom function call.", "log-in", "#a78bfa", domain.NodePure, nil, nil, nil, map[string]any{}),
		blueprintNode("function:output", "Functions", "Function Outputs", "Typed outputs returned from a pure custom function call.", "log-out", "#a78bfa", domain.NodePure, nil, nil, nil, map[string]any{}),
	}
}
