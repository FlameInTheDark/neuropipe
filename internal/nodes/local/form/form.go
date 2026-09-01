// Package form registers the Form Blueprint node, which shows a full-screen
// form modal built from a grid-based layout. Each input and dropdown becomes
// a typed output data pin; text panels produce no pin. The user fills the
// form and clicks Submit (routes from "submit") or Cancel (routes from
// "canceled").
package form

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the Form module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the complete Form module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	textType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "title", Name: "title", Type: typespec.String()},
		{ID: "canceled", Name: "canceled", Type: typespec.Bool()},
		{ID: "values", Name: "values", Type: domain.TypeSpec{Kind: domain.TypeMap, Key: &domain.TypeSpec{Kind: domain.TypeString}, Value: &domain.TypeSpec{Kind: domain.TypeAny}}},
	}}
	return domain.NodeDefinition{
		Type:        "action:form",
		Category:    "Display",
		Label:       "Form",
		Description: "Show a full-screen form modal; emit one output pin per input field and dropdown.",
		Icon:        "text-cursor-input",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "title", Label: "Title", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "message", Label: "Message", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "submit", Label: "Submit", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#34d399", MaxConnections: 1},
			{ID: "canceled", Label: "Canceled", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#f87171", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "title", Label: "Title", DataType: domain.DataText},
					{Path: "canceled", Label: "Canceled", DataType: domain.DataBoolean},
					{Path: "values", Label: "Values", DataType: domain.DataObject},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "title", Label: "Title", Kind: "string", Placeholder: "Form", Required: false},
			{Name: "message", Label: "Message", Kind: "textarea", Placeholder: "Please fill in the form.", Required: false},
			{Name: "form", Label: "Form layout", Kind: "form-builder", Required: true},
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"title": "Form", "message": "", "form": map[string]any{"items": []any{map[string]any{"id": "field_1", "kind": "input", "label": "Input", "col": 0, "row": 0, "span": 4, "rowSpan": 1, "inputType": "text"}}}},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

// resolve adapts the output pin contract to the configured form layout so the
// editor highlights compatible connections for each field's type.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	layout, err := formLayoutFor(config(node), def.DefaultConfig)
	if err != nil {
		return def, err
	}
	dynamic := make([]domain.NodePort, 0, len(layout.Items))
	for _, item := range layout.Items {
		if item.Kind == ItemText {
			continue
		}
		var dataType domain.DataType
		var typeSpec domain.TypeSpec
		switch {
		case item.Kind == ItemInput && item.InputType == InputNumber:
			dataType, typeSpec = domain.DataNumber, typespec.Float()
		case item.Kind == ItemInput:
			dataType, typeSpec = domain.DataText, typespec.String()
		case item.Kind == ItemDropdown:
			dataType, typeSpec = domain.DataText, typespec.String()
		}
		t := typeSpec
		dynamic = append(dynamic, domain.NodePort{
			ID: item.ID, Label: item.Label, Kind: domain.PinData, Direction: domain.PinOutput,
			DataType: dataType, Type: &t, Color: dataColor(dataType), MaxConnections: 1,
		})
	}
	resolved := def
	resolved.Outputs = append(dynamic, def.Outputs...)
	return resolved, nil
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("form cancelled: %w", err)
	}
	provider, ok := runtime.(nodes.FormDialogOpenerProvider)
	if !ok || provider.FormDialogOpener() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("form dialogs are unavailable for this execution")
	}
	layout, err := formLayoutFor(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	title := resolveText(invocation, "title", "Form")
	message := resolveText(invocation, "message", "")
	items := make([]nodes.FormItemSpec, 0, len(layout.Items))
	for _, item := range layout.Items {
		spec := nodes.FormItemSpec{
			ID: item.ID, Kind: string(item.Kind), Label: item.Label,
			Col: item.Col, Row: item.Row, Span: item.Span, RowSpan: item.RowSpan,
			InputType: string(item.InputType), Placeholder: item.Placeholder,
		}
		for _, opt := range item.Options {
			spec.Options = append(spec.Options, nodes.FormItemOption{Value: opt.Value, Label: opt.Label})
		}
		items = append(items, spec)
	}
	response, err := provider.FormDialogOpener().ShowForm(ctx, nodes.FormRequest{
		Title: title, Message: message,
		Continue: "Submit", Cancel: "Cancel",
		Items: items,
	})
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("form: %w", err)
	}
	result := map[string]any{"title": title, "canceled": response.Canceled, "values": response.Values}
	if response.Canceled {
		return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"canceled"}}, nil
	}
	outputs := map[string]any{"result": result}
	for id, value := range response.Values {
		outputs[id] = value
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"submit"}}, nil
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

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

func dataColor(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	default:
		return "#a1a1aa"
	}
}
