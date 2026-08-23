package executord

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
)

const hostCallTimeout = 5 * time.Minute

// TunnelCaller is the executor side of the reverse tunnel. The desktop opens
// the bidirectional stream; this caller writes HostCall frames onto it for
// nodes that need desktop-hosted services and correlates HostResponse frames
// by request ID.
type TunnelCaller struct {
	mu      sync.Mutex
	stream  executorv1.Executor_TunnelServer
	pending map[string]chan *executorv1.HostResponse
	sendMu  sync.Mutex
}

func NewTunnelCaller() *TunnelCaller {
	return &TunnelCaller{pending: make(map[string]chan *executorv1.HostResponse)}
}

// Attach registers a live tunnel client stream (opened by the desktop
// session) and consumes responses until the stream ends. Pending calls fail
// immediately so nodes never hang on a dead session.
func (t *TunnelCaller) Attach(stream executorv1.Executor_TunnelServer) {
	t.mu.Lock()
	t.stream = stream
	for id, wait := range t.pending {
		close(wait)
		delete(t.pending, id)
	}
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		if t.stream == stream {
			t.stream = nil
		}
		t.mu.Unlock()
	}()
	for {
		frame, err := stream.Recv()
		if err != nil {
			return
		}
		response := frame.GetResponse()
		if response == nil {
			continue
		}
		t.mu.Lock()
		wait, ok := t.pending[frame.GetRequestId()]
		if ok {
			delete(t.pending, frame.GetRequestId())
		}
		t.mu.Unlock()
		if ok {
			wait <- response
			close(wait)
		}
	}
}

// Call performs one host call over the attached stream.
func (t *TunnelCaller) Call(ctx context.Context, kind string, request any, response any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode %s host call: %w", kind, err)
	}
	t.mu.Lock()
	stream := t.stream
	if stream == nil {
		t.mu.Unlock()
		return fmt.Errorf("%w: desktop session is not connected", remoteexec.ErrHostCallFailed)
	}
	id := newRequestID()
	pending := make(chan *executorv1.HostResponse, 1)
	t.pending[id] = pending
	t.mu.Unlock()
	defer t.forget(id)

	if err := t.send(stream, id, kind, payload); err != nil {
		return fmt.Errorf("%s host call failed: %w", kind, err)
	}

	select {
	case reply, ok := <-pending:
		if !ok || reply == nil {
			return fmt.Errorf("%w: %s call interrupted by disconnect", remoteexec.ErrHostCallFailed, kind)
		}
		if !reply.GetOk() {
			return fmt.Errorf("%s", reply.GetError())
		}
		if response != nil && len(reply.GetPayloadJson()) > 0 {
			if err := json.Unmarshal(reply.GetPayloadJson(), response); err != nil {
				return fmt.Errorf("decode %s host response: %w", kind, err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hostCallTimeout):
		return fmt.Errorf("%s host call timed out", kind)
	}
}

// send serialises SendMsg: gRPC streams do not allow concurrent sends.
func (t *TunnelCaller) send(stream executorv1.Executor_TunnelServer, id, kind string, payload []byte) error {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	return stream.Send(&executorv1.TunnelFrame{
		RequestId: id,
		Payload: &executorv1.TunnelFrame_Call{Call: &executorv1.HostCall{
			Kind:        kind,
			PayloadJson: payload,
		}},
	})
}

func (t *TunnelCaller) forget(id string) {
	t.mu.Lock()
	delete(t.pending, id)
	t.mu.Unlock()
}

func newRequestID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer[:])
}

// proxiedLLM routes AI-node provider calls through the desktop session so
// API keys stay configured only inside Neuropipe. It satisfies both
// pipeline.LLMRunner and pipeline.AssistantRunner.
type proxiedLLM struct{ tunnel *TunnelCaller }

func (p proxiedLLM) Chat(ctx context.Context, request pipeline.ChatRequest) (pipeline.ChatResponse, error) {
	var response pipeline.ChatResponse
	if err := p.tunnel.Call(ctx, remoteexec.HostCallLLMChat, request, &response); err != nil {
		return pipeline.ChatResponse{}, err
	}
	return response, nil
}

func (p proxiedLLM) Converse(ctx context.Context, request domain.AssistantChatRequest) (domain.AssistantChatResponse, error) {
	var response domain.AssistantChatResponse
	if err := p.tunnel.Call(ctx, remoteexec.HostCallLLMConverse, request, &response); err != nil {
		return domain.AssistantChatResponse{}, err
	}
	return response, nil
}

// proxiedReports persists reports in the desktop workspace.
type proxiedReports struct{ tunnel *TunnelCaller }

func (p proxiedReports) CreateReport(ctx context.Context, report domain.Report) (domain.Report, error) {
	var created domain.Report
	if err := p.tunnel.Call(ctx, remoteexec.HostCallCreateReport, report, &created); err != nil {
		return domain.Report{}, err
	}
	return created, nil
}

// proxiedChat forwards chat conversation updates to the desktop.
type proxiedChat struct{ tunnel *TunnelCaller }

func (p proxiedChat) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	var message domain.ChatMessage
	request := remoteexec.ChatAppendReplyRequest{ChatRunID: chatRunID, Content: content}
	if err := p.tunnel.Call(ctx, remoteexec.HostCallChatAppendReply, request, &message); err != nil {
		return domain.ChatMessage{}, err
	}
	return message, nil
}

func (p proxiedChat) UpdateChatStatus(ctx context.Context, chatRunID, status string) error {
	request := remoteexec.ChatUpdateStatusRequest{ChatRunID: chatRunID, Status: status}
	return p.tunnel.Call(ctx, remoteexec.HostCallChatUpdateStat, request, nil)
}

func (p proxiedChat) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	var messages []domain.ChatMessage
	request := remoteexec.ChatReadHistoryRequest{ChatID: chatID, Limit: limit}
	if err := p.tunnel.Call(ctx, remoteexec.HostCallChatReadHistory, request, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// proxiedSQL executes registered-database queries on the desktop so database
// credentials never leave the user's machine.
type proxiedSQL struct{ tunnel *TunnelCaller }

func (p proxiedSQL) ExecuteSQL(ctx context.Context, request domain.SQLRequest) (domain.SQLResult, error) {
	var result domain.SQLResult
	if err := p.tunnel.Call(ctx, remoteexec.HostCallSQLQuery, request, &result); err != nil {
		return domain.SQLResult{}, err
	}
	return result, nil
}

// proxiedTwitch sends Twitch chat messages through the desktop identities.
type proxiedTwitch struct{ tunnel *TunnelCaller }

func (p proxiedTwitch) SendTwitchChatMessage(ctx context.Context, request domain.TwitchChatMessageRequest) (domain.TwitchChatMessageResult, error) {
	var result domain.TwitchChatMessageResult
	if err := p.tunnel.Call(ctx, remoteexec.HostCallTwitchSend, request, &result); err != nil {
		return domain.TwitchChatMessageResult{}, err
	}
	return result, nil
}

var (
	_ pipeline.LLMRunner       = proxiedLLM{}
	_ pipeline.AssistantRunner = proxiedLLM{}
	_ pipeline.ReportWriter    = proxiedReports{}
	_ pipeline.ChatWriter      = proxiedChat{}
	_ nodes.SQLExecutor        = proxiedSQL{}
	_ nodes.TwitchChatSender   = proxiedTwitch{}
)
