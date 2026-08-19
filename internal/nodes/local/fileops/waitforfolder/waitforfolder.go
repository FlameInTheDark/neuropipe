// Package waitforfolder registers the Wait For Folder Blueprint node.
package waitforfolder

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node {
	return Node{Metadata: definition(), Executor: execute}
}

func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	pathType := typespec.String()
	floatType := typespec.Float()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "found", Name: "found", Type: typespec.Bool()},
		{ID: "waitedSeconds", Name: "waitedSeconds", Type: typespec.Float()},
	}}
	return domain.NodeDefinition{
		Type:        "action:wait_for_folder",
		Category:    "Folders",
		Label:       "Wait For Folder",
		Description: "Poll a path until a directory exists at it, or until the timeout elapses.",
		Icon:        "hourglass",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "timeoutSeconds", Label: "Timeout (s)", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", MaxConnections: 1, Default: 30.0},
			{ID: "pollIntervalSeconds", Label: "Poll (s)", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", MaxConnections: 1, Default: 0.5},
		},
		Outputs: []domain.NodePort{
			{ID: "found", Label: "Found", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#34d399", MaxConnections: 1},
			{ID: "timeout", Label: "Timeout", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#f87171", MaxConnections: 1},
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\inbox", Required: true},
			{Name: "timeoutSeconds", Label: "Timeout (s)", Kind: "number", Placeholder: "30", Required: false},
			{Name: "pollIntervalSeconds", Label: "Poll (s)", Kind: "number", Placeholder: "0.5", Required: false},
		},
		Capabilities:      []domain.Capability{domain.CapabilityFileRead},
		DefaultConfig:     map[string]any{"timeoutSeconds": 30.0, "pollIntervalSeconds": 0.5},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	path, err := fileops.CleanInput(invocation.Inputs["path"])
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("wait for folder: %w", err)
	}
	timeout := floatValue(invocation, "timeoutSeconds", 30)
	if timeout <= 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("wait for folder timeout must be greater than zero")
	}
	poll := floatValue(invocation, "pollIntervalSeconds", 0.5)
	if poll <= 0 {
		poll = 0.5
	}
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
	interval := time.Duration(poll * float64(time.Second))
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("wait for folder cancelled: %w", err)
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			waited := time.Since(start).Seconds()
			return nodes.ExecutionResult{
				Outputs: map[string]any{
					"result": map[string]any{"found": true, "waitedSeconds": waited},
				},
				Ports: []string{"found"},
			}, nil
		}
		if time.Now().After(deadline) {
			waited := time.Since(start).Seconds()
			return nodes.ExecutionResult{
				Outputs: map[string]any{
					"result": map[string]any{"found": false, "waitedSeconds": waited},
				},
				Ports: []string{"timeout"},
			}, nil
		}
		select {
		case <-ctx.Done():
			return nodes.ExecutionResult{}, fmt.Errorf("wait for folder cancelled: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

func floatValue(invocation nodes.Invocation, name string, fallback float64) float64 {
	if value, ok := asFloat(invocation.Inputs[name]); ok {
		return value
	}
	if value, ok := asFloat(invocation.Config[name]); ok {
		return value
	}
	return fallback
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			var parsed float64
			if _, err := fmt.Sscanf(trimmed, "%g", &parsed); err == nil {
				return parsed, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}
