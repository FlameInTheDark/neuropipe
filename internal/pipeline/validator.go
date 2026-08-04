package pipeline

import (
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Validate ensures a Blueprint-v2 graph has safe control flow and typed pin
// connections before it is run or published.
func Validate(definition domain.FlowDefinition, registry *catalog.Registry) error {
	if definition.SchemaVersion != domain.GraphSchemaV2 {
		return ValidationError{Message: "legacy graph: rebuild this pipeline with Blueprint v2 pins"}
	}
	if len(definition.Nodes) == 0 {
		return ValidationError{Message: "a pipeline needs at least one node"}
	}
	nodes := make(map[string]domain.FlowNode, len(definition.Nodes))
	metadata := make(map[string]domain.NodeDefinition, len(definition.Nodes))
	events := 0
	for _, node := range definition.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return ValidationError{Message: "every node needs an ID"}
		}
		if _, exists := nodes[node.ID]; exists {
			return ValidationError{Message: fmt.Sprintf("node ID %q is duplicated", node.ID)}
		}
		definition, exists := registry.Get(node.Type)
		if !exists {
			return ValidationError{Message: fmt.Sprintf("node %q uses unavailable type %q", node.ID, node.Type)}
		}
		definition, err := definitionForNode(definition, node)
		if err != nil {
			return ValidationError{Message: fmt.Sprintf("node %q has invalid configuration: %v", node.ID, err)}
		}
		if definition.Mode == domain.NodeEvent {
			events++
			if err := validateTrigger(node); err != nil {
				return err
			}
		}
		if containsTemplate(node.Data) {
			return ValidationError{Message: fmt.Sprintf("node %q still uses a legacy {{...}} template; wire a data pin instead", node.ID)}
		}
		nodes[node.ID], metadata[node.ID] = node, definition
	}
	if events == 0 {
		return ValidationError{Message: "a Blueprint pipeline needs an event node"}
	}
	execGraph, dataGraph := make(map[string][]string), make(map[string][]string)
	incoming := make(map[string]int)
	for _, edge := range definition.Edges {
		source, sourceOK := nodes[edge.Source]
		target, targetOK := nodes[edge.Target]
		if !sourceOK {
			return ValidationError{Message: fmt.Sprintf("edge %q has an unknown source", edge.ID)}
		}
		if !targetOK {
			return ValidationError{Message: fmt.Sprintf("edge %q has an unknown target", edge.ID)}
		}
		kind := edgeKind(edge)
		sourceHandle := edge.SourceHandle
		if sourceHandle == "" {
			sourceHandle = "out"
		}
		targetHandle := edge.TargetHandle
		if targetHandle == "" {
			targetHandle = "in"
		}
		sourcePin, sourcePinOK := findOutput(metadata[source.ID], sourceHandle)
		targetPin, targetPinOK := findInput(metadata[target.ID], targetHandle)
		if !sourcePinOK {
			return ValidationError{Message: fmt.Sprintf("edge %q uses unavailable source pin %q", edge.ID, sourceHandle)}
		}
		if !targetPinOK {
			return ValidationError{Message: fmt.Sprintf("edge %q uses unavailable target pin %q", edge.ID, targetHandle)}
		}
		if sourcePin.Kind != kind || targetPin.Kind != kind {
			return ValidationError{Message: fmt.Sprintf("edge %q must connect matching %s pins", edge.ID, kind)}
		}
		if kind == domain.PinData && !typesCompatible(sourcePin.DataType, targetPin.DataType) {
			return ValidationError{Message: fmt.Sprintf("edge %q cannot connect %s data to %s", edge.ID, sourcePin.DataType, targetPin.DataType)}
		}
		key := edge.Target + ":" + targetHandle
		incoming[key]++
		if targetPin.MaxConnections > 0 && incoming[key] > targetPin.MaxConnections {
			return ValidationError{Message: fmt.Sprintf("pin %q accepts only %d connection", targetHandle, targetPin.MaxConnections)}
		}
		if kind == domain.PinExec {
			execGraph[edge.Source] = append(execGraph[edge.Source], edge.Target)
		} else {
			dataGraph[edge.Source] = append(dataGraph[edge.Source], edge.Target)
		}
	}
	if hasCycle(nodes, execGraph) {
		return ValidationError{Message: "arbitrary Exec cycles are not supported; use a dedicated loop node"}
	}
	if hasCycle(nodes, dataGraph) {
		return ValidationError{Message: "data-pin dependencies cannot form a cycle"}
	}
	return nil
}

func findInput(definition domain.NodeDefinition, id string) (domain.NodePort, bool) {
	for _, pin := range definition.Inputs {
		if pin.ID == id {
			return pin, true
		}
	}
	return domain.NodePort{}, false
}
func findOutput(definition domain.NodeDefinition, id string) (domain.NodePort, bool) {
	for _, pin := range definition.Outputs {
		if pin.ID == id {
			return pin, true
		}
	}
	return domain.NodePort{}, false
}
func typesCompatible(source, target domain.DataType) bool {
	return source == domain.DataAny || target == domain.DataAny || source == target
}

func validateTrigger(node domain.FlowNode) error {
	config := configFor(node)
	switch node.Type {
	case "trigger:button":
		if text(config, "label") == "" {
			return ValidationError{Message: fmt.Sprintf("button trigger %q needs a label", node.ID)}
		}
	case "trigger:cron":
		if text(config, "cron") == "" {
			return ValidationError{Message: fmt.Sprintf("cron trigger %q needs an expression", node.ID)}
		}
	case "trigger:chat":
		if text(config, "label") == "" {
			return ValidationError{Message: fmt.Sprintf("chat trigger %q needs a label", node.ID)}
		}
	}
	return nil
}

func containsTemplate(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, "{{") && strings.Contains(typed, "}}")
	case map[string]any:
		for _, nested := range typed {
			if containsTemplate(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsTemplate(nested) {
				return true
			}
		}
	}
	return false
}

func hasCycle(nodes map[string]domain.FlowNode, adjacency map[string][]string) bool {
	state := make(map[string]uint8, len(nodes))
	var visit func(string) bool
	visit = func(id string) bool {
		state[id] = 1
		for _, next := range adjacency[id] {
			if state[next] == 1 || (state[next] == 0 && visit(next)) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for id := range nodes {
		if state[id] == 0 && visit(id) {
			return true
		}
	}
	return false
}

func configFor(node domain.FlowNode) map[string]any {
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return node.Data
}
func text(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}
