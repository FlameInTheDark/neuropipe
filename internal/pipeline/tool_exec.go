package pipeline

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// This file exposes published LLM tool functions to callers outside a
// running Blueprint graph. Agent nodes consume the same machinery through
// connectedTools/runConnectedTool; model chats reach it through
// execution.Service.RunToolFunction so both surfaces share one schema
// builder and one argument decoder.

// ToolFunctionName returns the provider-safe tool identifier for a function
// (the same name an Agent node advertises for the function).
func ToolFunctionName(function domain.CustomFunction) string {
	return toolName(function)
}

// ToolFunctionDefinition builds the provider contract (name, description,
// JSON input schema) for one published LLM tool function. It validates the
// function first, so callers get the same precise schema the Agent path uses.
func ToolFunctionDefinition(function domain.CustomFunction) (domain.ChatToolDefinition, error) {
	tool, err := makeConnectedTool(function)
	if err != nil {
		return domain.ChatToolDefinition{}, err
	}
	return tool.definition, nil
}

// ExecuteToolFunction runs one published LLM tool function's Blueprint graph
// with model-supplied JSON arguments. Arguments are matched to the function's
// input pins by their public tool names, decoded into pin values, and the
// graph's Function Return pins are mapped back to a JSON-safe result object.
//
// The engine must already carry the infrastructure the function's nodes need
// (function resolver, notification/chat writers, …); the execution service
// builds it with the same options a pipeline run receives. The capability
// gate is nil because tool functions are user-authored local automation that
// has already been reviewed at publish time.
func ExecuteToolFunction(ctx context.Context, engine *Engine, function domain.CustomFunction, arguments map[string]any) (map[string]any, error) {
	tool, err := makeConnectedTool(function)
	if err != nil {
		return nil, err
	}
	state := newBlueprintState(engine, ctx, domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3})
	return state.runConnectedTool(domain.FlowNode{ID: "chat:" + function.ID, Type: "function:" + function.ID, Data: map[string]any{"config": map[string]any{}}}, tool, arguments)
}
