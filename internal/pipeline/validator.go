package pipeline

import (
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Validate ensures a supported Blueprint graph has safe control flow and typed
// pin connections before it is run or published. V3 adds strict TypeSpec
// contracts; V2 remains readable until its persistence migration runs.
func Validate(definition domain.FlowDefinition, registry *catalog.Registry) error {
	if definition.SchemaVersion != domain.GraphSchemaV2 && definition.SchemaVersion != domain.GraphSchemaV3 {
		return ValidationError{Message: "legacy graph: rebuild this pipeline with Blueprint v3 pins"}
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
		// Reroutes are presentation-only wire waypoints in current Blueprint
		// graphs. V2 registrations remain solely to execute immutable legacy
		// revisions until they are reopened and migrated.
		if definition.SchemaVersion >= domain.GraphSchemaV3 && (node.Type == "flow:reroute" || node.Type == "data:reroute") {
			return ValidationError{Message: fmt.Sprintf("node %q is a legacy reroute; reopen the draft to migrate it to a wire waypoint", node.ID)}
		}
		definition, exists := registry.Get(node.Type)
		if !exists {
			return ValidationError{Message: fmt.Sprintf("node %q uses unavailable type %q", node.ID, node.Type)}
		}
		definition, err := definitionForRegisteredNode(registry, definition, node)
		if err != nil {
			return ValidationError{Message: fmt.Sprintf("node %q has invalid configuration: %v", node.ID, err)}
		}
		if definition.Mode == domain.NodeEvent {
			events++
			if err := validateTrigger(node); err != nil {
				return err
			}
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
		if kind == domain.PinData && !pinsCompatibleForSchema(definition.SchemaVersion, sourcePin, targetPin) {
			return ValidationError{Message: fmt.Sprintf("edge %q cannot connect %s data to %s", edge.ID, pinTypeName(sourcePin), pinTypeName(targetPin))}
		}
		key := edge.Target + ":" + targetHandle
		incoming[key]++
		if targetPin.MaxConnections > 0 && incoming[key] > targetPin.MaxConnections {
			return ValidationError{Message: fmt.Sprintf("pin %q accepts only %d connection", targetHandle, targetPin.MaxConnections)}
		}
		switch kind {
		case domain.PinExec:
			execGraph[edge.Source] = append(execGraph[edge.Source], edge.Target)
		case domain.PinData:
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

func pinsCompatible(source, target domain.NodePort) bool {
	if source.Type != nil && target.Type != nil {
		return typespec.Assignable(*source.Type, *target.Type)
	}
	return typesCompatible(source.DataType, target.DataType)
}

func pinsCompatibleForSchema(schemaVersion int, source, target domain.NodePort) bool {
	if schemaVersion >= domain.GraphSchemaV3 {
		return pinsCompatible(source, target)
	}
	return typesCompatible(source.DataType, target.DataType)
}

func pinTypeName(pin domain.NodePort) string {
	if pin.Type != nil {
		return string(pin.Type.Kind)
	}
	return string(pin.DataType)
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
