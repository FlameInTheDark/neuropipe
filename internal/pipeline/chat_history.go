package pipeline

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// agentChatHistoryLimit matches the turn count the built-in chat view feeds
// its model-mode conversations, so pipeline agents see the same context window.
const agentChatHistoryLimit = 200

// ChatMode selects how an agent receives its conversation. Graphs saved with
// the earlier Use-chat-history toggle keep working through pullChatHistory.
const (
	chatModeMessage = "message"
	chatModeHistory = "history"
)

// chatHistoryMode reports whether the agent continues a chat conversation
// instead of answering one composed message. The default one-message value
// never masks the earlier toggle, so graphs saved with pullChatHistory
// enabled keep their behaviour until the mode is re-saved.
func chatHistoryMode(config map[string]any) bool {
	if mode, ok := config["chatMode"].(string); ok && mode == chatModeHistory {
		return true
	}
	return boolValue(config["pullChatHistory"])
}

// agentChatHistory reads the conversation a chat-mode agent continues. The
// Chat ID pin carries the conversation ID emitted by the Chat trigger, so the
// latest user turn is already the final history entry.
func (e *Engine) agentChatHistory(ctx context.Context, chatID string) ([]domain.ChatMessage, error) {
	if e.chat == nil {
		return nil, fmt.Errorf("chat history is unavailable for this execution")
	}
	messages, err := e.chat.ReadChatHistory(ctx, chatID, agentChatHistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("read chat history: %w", err)
	}
	// Keep plain dialogue turns only: tool chatter and stored tool-call
	// references belong to their original run and would dangle in a fresh
	// provider request.
	history := make([]domain.ChatMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role != domain.ChatRoleUser && message.Role != domain.ChatRoleAssistant {
			continue
		}
		history = append(history, domain.ChatMessage{Role: message.Role, Content: message.Content})
	}
	if len(history) == 0 {
		return nil, fmt.Errorf("chat %q has no messages to answer", chatID)
	}
	return history, nil
}

// agentHistory returns the conversation turns for a chat-history agent, or
// nil when the agent runs in one-message mode. A wired Chat ID pin overrides
// the inspector value because callers pass the merged map.
func (e *Engine) agentHistory(ctx context.Context, node domain.FlowNode, merged map[string]any) ([]domain.ChatMessage, error) {
	if !chatHistoryMode(merged) {
		return nil, nil
	}
	chatID := text(merged, "chatId")
	if chatID == "" {
		return nil, fmt.Errorf("node %q uses chat history mode but has no Chat ID", node.ID)
	}
	return e.agentChatHistory(ctx, chatID)
}

// agentHistoryMessages assembles the conversation prefix for an agent run:
// the system prompt, then the chat history in history mode. One-message mode
// has no history; its user turn is appended by the caller.
func agentHistoryMessages(systemPrompt string, history []domain.ChatMessage) []domain.ChatMessage {
	messages := make([]domain.ChatMessage, 0, 1+len(history))
	if systemPrompt != "" {
		messages = append(messages, domain.ChatMessage{Role: domain.ChatRoleSystem, Content: systemPrompt})
	}
	return append(messages, history...)
}
