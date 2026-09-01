package pipeline

import (
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// definitionForRegisteredNode asks a first-party module to resolve its own
// dynamic ports. The fallback remains only for plugin and custom function
// definitions that do not carry a registered module handler.
func definitionForRegisteredNode(registry *catalog.Registry, definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	if module, exists := registry.Node(node.Type); exists {
		return module.Resolve(node)
	}
	return definitionForNode(definition, node)
}

// definitionForNode expands configuration-driven pins for engine-implemented
// node types. Keeping this in the runtime layer makes the editor, validator,
// and interpreter agree on the same stable IDs.
func definitionForNode(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	config := configFor(node)
	switch node.Type {
	case "llm:choice":
		options, exists := config["options"]
		if !exists {
			options = definition.DefaultConfig["options"]
		}
		ports, err := routeOptionPorts(options)
		if err != nil {
			return definition, err
		}
		definition.Outputs = append(ports, definition.Outputs...)
		definition.Inputs = filterChatContextPins(node, config, definition.Inputs)
	case "llm:prompt", "llm:extract", "llm:boolean", "llm:summarize", "llm:agent", "llm:coding_agent":
		definition.Inputs = filterChatContextPins(node, config, definition.Inputs)
	}
	return definition, nil
}

// filterNodePort returns the ports without the named pin, sharing the original
// slice whenever the pin is absent so callers keep ownership semantics.
func filterNodePort(ports []domain.NodePort, id string) []domain.NodePort {
	filtered := make([]domain.NodePort, 0, len(ports))
	for _, port := range ports {
		if port.ID == id {
			continue
		}
		filtered = append(filtered, port)
	}
	return filtered
}

// filterChatContextPins hides the toggle-gated chat pins of LLM nodes: the
// Chat Run ID pin exists only while status updates are enabled, and the Chat
// ID pin only in the agents' chat-history mode. The editor mirrors this so
// validation and connections always agree.
func filterChatContextPins(node domain.FlowNode, config map[string]any, inputs []domain.NodePort) []domain.NodePort {
	if !boolValue(config["updateChatStatus"]) {
		inputs = filterNodePort(inputs, "chatRunId")
	}
	if (node.Type == "llm:agent" || node.Type == "llm:coding_agent") && !chatHistoryMode(config) {
		inputs = filterNodePort(inputs, "chatId")
	}
	return inputs
}

func routeOptionPorts(value any) ([]domain.NodePort, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("options must be a list")
	}
	ports := make([]domain.NodePort, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, raw := range items {
		option, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("options item %d must be an object", index+1)
		}
		id := strings.TrimSpace(fmt.Sprint(option["id"]))
		if id == "" {
			return nil, fmt.Errorf("options item %d needs an ID", index+1)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("options contains duplicate ID %q", id)
		}
		seen[id] = struct{}{}
		label := strings.TrimSpace(fmt.Sprint(option["label"]))
		if label == "" {
			label = id
		}
		ports = append(ports, domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1})
	}
	return ports, nil
}
