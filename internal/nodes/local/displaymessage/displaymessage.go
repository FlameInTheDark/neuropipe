// Package displaymessage registers the Display Message Blueprint node, which
// shows a native OS information dialog with a title, message, and OK button.
// Pipeline execution blocks until the user dismisses the dialog.
package displaymessage

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

// New creates the Display Message module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Executor: execute}
}

// Register contributes the complete Display Message module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	textType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "title", Name: "title", Type: typespec.String()},
		{ID: "message", Name: "message", Type: typespec.String()},
		{ID: "dismissed", Name: "dismissed", Type: typespec.Bool()},
	}}
	return domain.NodeDefinition{
		Type:        "action:display_message",
		Category:    "Local",
		Label:       "Display Message",
		Description: "Show a native dialog window with a title, message, and OK button.",
		Icon:        "message-square",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "title", Label: "Title", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
			{ID: "message", Label: "Message", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "title", Label: "Title", DataType: domain.DataText, Description: "Dialog title that was shown."},
					{Path: "message", Label: "Message", DataType: domain.DataText, Description: "Dialog message that was shown."},
					{Path: "dismissed", Label: "Dismissed", DataType: domain.DataBoolean, Description: "Whether the user dismissed the dialog."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "title", Label: "Title", Kind: "string", Placeholder: "Neuropipe", Required: false},
			{Name: "message", Label: "Message", Kind: "textarea", Placeholder: "Done", Required: false},
		},
		Capabilities:      []domain.Capability{},
		DefaultConfig:     map[string]any{"title": "Neuropipe", "message": ""},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display message cancelled: %w", err)
	}
	provider, ok := runtime.(nodes.DialogOpenerProvider)
	if !ok || provider.DialogOpener() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("native dialogs are unavailable for this execution")
	}
	opener := provider.DialogOpener()
	title := resolveText(invocation, "title", "Neuropipe")
	message := resolveText(invocation, "message", "")
	if err := opener.ShowMessage(ctx, title, message); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("display message: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"title":     title,
				"message":   message,
				"dismissed": true,
			},
		},
		Ports: []string{"out"},
	}, nil
}

// resolveText prefers a connected data pin over the inspector field. Empty
// values fall back to the supplied default so users can omit either source.
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
