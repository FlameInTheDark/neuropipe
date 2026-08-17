package pipeline

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Status texts published to the chat run while an LLM node works. They mirror
// the plain English the chat lifecycle already uses ("Working", "Completed"),
// which the execution service overwrites with the final state.
const (
	chatStatusThinking = "Thinking"
	chatStatusRunning  = "Running"
)

// chatStatusReporter returns a live-status publisher for an LLM node, or nil
// when its Update-chat-status toggle is off. A wired Chat Run ID pin overrides
// the inspector value because callers pass the merged map. Status updates keep
// the run marked Running; the execution service writes the final state.
func (e *Engine) chatStatusReporter(ctx context.Context, node domain.FlowNode, config map[string]any) (func(string) error, error) {
	if !boolValue(config["updateChatStatus"]) {
		return nil, nil
	}
	chatRunID := text(config, "chatRunId")
	if chatRunID == "" {
		return nil, fmt.Errorf("node %q enables chat status updates but has no Chat Run ID", node.ID)
	}
	if e.chat == nil {
		return nil, fmt.Errorf("chat status is unavailable for this execution")
	}
	return func(statusText string) error {
		if err := e.chat.UpdateChatStatus(ctx, chatRunID, statusText); err != nil {
			return fmt.Errorf("update chat status: %w", err)
		}
		return nil
	}, nil
}

// reportModelStatus publishes a progress status when a reporter is active.
func reportModelStatus(status func(string) error, statusText string) error {
	if status == nil {
		return nil
	}
	return status(statusText)
}

// toolStatusText describes one connected-tool invocation for the chat run.
func toolStatusText(name string) string {
	if name == "" {
		return chatStatusRunning
	}
	return chatStatusRunning + " " + name
}
