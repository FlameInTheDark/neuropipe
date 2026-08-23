// Package remoteexec contains the gRPC contract helpers shared by the desktop
// control plane (client) and the standalone executor daemon (server). It owns
// bearer-token authentication, tunnel frame codecs, and proto/domain
// conversions so neither side depends on the other's internals.
package remoteexec

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// Host call kinds carried inside Tunnel frames. The executor issues these
// calls whenever a node needs a desktop-hosted service.
const (
	HostCallLLMChat         = "llm.chat"
	HostCallLLMConverse     = "llm.converse"
	HostCallCreateReport    = "report.create"
	HostCallReportGet       = "report.get"
	HostCallReportList      = "report.list"
	HostCallChatAppendReply = "chat.append_reply"
	HostCallChatUpdateStat  = "chat.update_status"
	HostCallChatReadHistory = "chat.read_history"
	HostCallSQLQuery        = "sql.query"
	HostCallTwitchSend      = "twitch.send_message"
)

// ChatAppendReplyRequest identifies the chat run receiving a reply.
type ChatAppendReplyRequest struct {
	ChatRunID string `json:"chatRunId"`
	Content   string `json:"content"`
}

// ChatUpdateStatusRequest forwards a chat status text.
type ChatUpdateStatusRequest struct {
	ChatRunID string `json:"chatRunId"`
	Status    string `json:"status"`
}

// ChatReadHistoryRequest loads a bounded conversation window.
type ChatReadHistoryRequest struct {
	ChatID string `json:"chatId"`
	Limit  int    `json:"limit"`
}

// ReportListRequest bounds a desktop report listing.
type ReportListRequest struct {
	Limit int `json:"limit"`
}

// HostBridge is the desktop-side service surface answering executor tunnel
// calls. The app composition implements it with the same local services used
// for desktop-local runs, so secrets and provider configuration stay local.
type HostBridge interface {
	LLMChat(ctx context.Context, request pipeline.ChatRequest) (pipeline.ChatResponse, error)
	LLMConverse(ctx context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error)
	CreateReport(ctx context.Context, report domain.Report) (domain.Report, error)
	GetReport(ctx context.Context, id string) (domain.Report, error)
	ListReports(ctx context.Context, limit int) ([]domain.Report, error)
	AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error)
	UpdateChatStatus(ctx context.Context, chatRunID, status string) error
	ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error)
	ExecuteSQL(ctx context.Context, request domain.SQLRequest) (domain.SQLResult, error)
	SendTwitchChatMessage(ctx context.Context, request domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error)
}

func encode(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode tunnel payload: %w", err)
	}
	return data, nil
}

func decode(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode tunnel payload: %w", err)
	}
	return nil
}
