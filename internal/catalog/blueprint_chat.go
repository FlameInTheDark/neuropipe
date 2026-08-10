package catalog

import "github.com/FlameInTheDark/neuropipe/internal/domain"

func blueprintChatBuiltins() []domain.NodeDefinition {
	return []domain.NodeDefinition{
		blueprintNode("action:chat_reply", "Chat", "Reply to Chat", "Send an ordered Markdown reply to the active chat run.", "reply", "#a78bfa", domain.NodeImpure,
			append(execInput(), dataPin("text", "Text", domain.PinInput, domain.DataText), dataPin("chatRunId", "Chat Run ID", domain.PinInput, domain.DataText)), execOutput(), nil, map[string]any{}),
		blueprintNode("action:chat_status", "Chat", "Update Chat Status", "Update the spinner text for the active chat run.", "loader-circle", "#a78bfa", domain.NodeImpure,
			append(execInput(), dataPin("status", "Status", domain.PinInput, domain.DataText), dataPin("chatRunId", "Chat Run ID", domain.PinInput, domain.DataText)), execOutput(), nil, map[string]any{}),
	}
}
