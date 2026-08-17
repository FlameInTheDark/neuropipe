// Package displayinput registers the Display Input Dialog Blueprint node, which
// shows a styled dialog with a title, message, labelled input field, and
// Continue/Cancel buttons. The node blocks until the user responds, then
// routes execution to the matching exec pin (Continue or Canceled) and emits
// the typed input value (or nil when cancelled) on its Value data pin.
package displayinput

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Display Input Dialog module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the complete Display Input Dialog module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	textType := typespec.String()
	anyType := typespec.Any()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "title", Name: "title", Type: typespec.String()},
		{ID: "message", Name: "message", Type: typespec.String()},
		{ID: "value", Name: "value", Type: typespec.Any()},
		{ID: "canceled", Name: "canceled", Type: typespec.Bool()},
	}}
	return domain.NodeDefinition{
		Type:        "action:display_input",
		Category:    "Display",
		Label:       "Display Input Dialog",
		Description: "Show a dialog with an input field and Continue/Cancel buttons; route execution by the user's choice.",
		Icon:        "keyboard",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "title", Label: "Title", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "message", Label: "Message", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "label", Label: "Label", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "continue", Label: "Continue", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#34d399", MaxConnections: 1},
			{ID: "canceled", Label: "Canceled", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#f87171", MaxConnections: 1},
			{ID: "value", Label: "Value", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataAny, Type: &anyType, Color: "#a1a1aa", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "title", Label: "Title", DataType: domain.DataText, Description: "Dialog title that was shown."},
					{Path: "message", Label: "Message", DataType: domain.DataText, Description: "Dialog message that was shown."},
					{Path: "value", Label: "Value", DataType: domain.DataAny, Description: "User-supplied value (nil when cancelled)."},
					{Path: "canceled", Label: "Canceled", DataType: domain.DataBoolean, Description: "Whether the user cancelled the dialog."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "title", Label: "Title", Kind: "string", Placeholder: "Neuropipe", Required: false},
			{Name: "message", Label: "Message", Kind: "textarea", Placeholder: "Enter a value:", Required: false},
			{Name: "label", Label: "Field label", Kind: "string", Placeholder: "Value", Required: false},
			datanodes.Select("type", "Input type", "text", "number"),
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"title": "Neuropipe", "message": "", "label": "Value", "type": "text"},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

// resolve adapts the output Value pin's contract to the configured input
// type so the editor highlights number-only connections when the user selects
// `number`, while keeping the on-wire value nil-typed when cancelled.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	config := config(node)
	target, _ := config["type"].(string)
	if target == "number" {
		definition.Outputs = append([]domain.NodePort(nil), definition.Outputs...)
		for index := range definition.Outputs {
			if definition.Outputs[index].ID == "value" && definition.Outputs[index].Kind == domain.PinData {
				numberType := typespec.Float()
				definition.Outputs[index].DataType = domain.DataNumber
				definition.Outputs[index].Type = &numberType
				definition.Outputs[index].Color = "#86efac"
			}
		}
	}
	return definition, nil
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display input cancelled: %w", err)
	}
	provider, ok := runtime.(nodes.InputDialogOpenerProvider)
	if !ok || provider.InputDialogOpener() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("input dialogs are unavailable for this execution")
	}
	opener := provider.InputDialogOpener()
	title := resolveText(invocation, "title", "Neuropipe")
	message := resolveText(invocation, "message", "")
	label := resolveText(invocation, "label", "Value")
	inputType, _ := invocation.Config["type"].(string)
	if inputType == "" {
		inputType = "text"
	}
	request := nodes.InputRequest{
		Title:       title,
		Message:     message,
		Label:       label,
		InputType:   inputType,
		Continue:    "Continue",
		Cancel:      "Cancel",
		Placeholder: placeholderFor(inputType),
	}
	response, err := opener.ShowInput(ctx, request)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display input: %w", err)
	}
	if response.Canceled {
		return nodes.ExecutionResult{
			Outputs: map[string]any{
				"value":  nil,
				"result": map[string]any{"title": title, "message": message, "value": nil, "canceled": true},
			},
			Ports: []string{"canceled"},
		}, nil
	}
	value, err := typedValue(inputType, response.Value)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display input value: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"value": value,
			"result": map[string]any{
				"title":    title,
				"message":  message,
				"value":    value,
				"canceled": false,
			},
		},
		Ports: []string{"continue"},
	}, nil
}

// typedValue converts the raw text input into the requested type. Number
// parsing uses strconv for strict validation; text values pass through
// unchanged.
func typedValue(inputType, raw string) (any, error) {
	switch inputType {
	case "number":
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, fmt.Errorf("number input is empty")
		}
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, fmt.Errorf("number input is invalid: %w", err)
		}
		return value, nil
	default:
		return raw, nil
	}
}

func placeholderFor(inputType string) string {
	if inputType == "number" {
		return "0"
	}
	return ""
}

func resolveText(invocation nodes.Invocation, name, fallback string) string {
	if value, ok := invocation.Inputs[name].(string); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	if value, ok := invocation.Config[name].(string); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func config(node domain.FlowNode) map[string]any {
	if value, ok := node.Data["config"].(map[string]any); ok {
		return value
	}
	return node.Data
}
