// Package displayquestion registers the Display Question Blueprint node, which
// shows a native OS question dialog with Yes and No buttons. Execution blocks
// until the user chooses one. The chosen exec pin (Yes or No) is selected so
// the graph follows the user's decision.
package displayquestion

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

// New creates the Display Question module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Executor: execute}
}

// Register contributes the complete Display Question module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	textType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "title", Name: "title", Type: typespec.String()},
		{ID: "message", Name: "message", Type: typespec.String()},
		{ID: "choice", Name: "choice", Type: typespec.String()},
	}}
	return domain.NodeDefinition{
		Type:        "action:display_question",
		Category:    "Local",
		Label:       "Display Question",
		Description: "Show a native dialog with Yes and No buttons and route execution to the chosen branch.",
		Icon:        "circle-help",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "title", Label: "Title", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "message", Label: "Message", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "yes", Label: "Yes", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#34d399", MaxConnections: 1},
			{ID: "no", Label: "No", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#f87171", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "title", Label: "Title", DataType: domain.DataText, Description: "Dialog title that was shown."},
					{Path: "message", Label: "Message", DataType: domain.DataText, Description: "Dialog message that was shown."},
					{Path: "choice", Label: "Choice", DataType: domain.DataText, Description: "The button the user pressed (yes or no)."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "title", Label: "Title", Kind: "string", Placeholder: "Neuropipe", Required: false},
			{Name: "message", Label: "Message", Kind: "textarea", Placeholder: "Continue?", Required: false},
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"title": "Neuropipe", "message": ""},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display question cancelled: %w", err)
	}
	provider, ok := runtime.(nodes.DialogOpenerProvider)
	if !ok || provider.DialogOpener() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("native dialogs are unavailable for this execution")
	}
	opener := provider.DialogOpener()
	title := resolveText(invocation, "title", "Neuropipe")
	message := resolveText(invocation, "message", "")
	choice, err := opener.ShowQuestion(ctx, title, message)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display question: %w", err)
	}
	result := map[string]any{
		"title":   title,
		"message": message,
		"choice":  string(choice),
	}
	switch choice {
	case nodes.DialogYes:
		return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"yes"}}, nil
	case nodes.DialogNo:
		return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"no"}}, nil
	default:
		// Cancellation or unknown selection: route to No so the graph still
		// terminates a downstream branch rather than leaving it dangling.
		result["choice"] = string(nodes.DialogNo)
		return nodes.ExecutionResult{Outputs: map[string]any{"result": result}, Ports: []string{"no"}}, nil
	}
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
