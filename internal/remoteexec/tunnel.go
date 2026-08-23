package remoteexec

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
)

// tunnelEnvelope pairs an outgoing frame with its completion signal so a
// single writer goroutine can serialise SendMsg on the shared stream.
type tunnelEnvelope struct {
	frame *executorv1.TunnelFrame
	errCh chan error
}

// newRequestID mints a random correlation ID for reverse host calls.
func newRequestID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer[:])
}

// pumpTunnel serves reverse host-service calls for one desktop session. The
// executor writes HostCall frames; each call is answered on the same stream
// with a HostResponse carrying the original request ID.
func (c *conn) pumpTunnel(ctx context.Context, done chan<- error) {
	if c.manager.bridge == nil {
		done <- fmt.Errorf("host services are not configured")
		return
	}
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		done <- fmt.Errorf("connection closed")
		return
	}
	stream, err := client.Tunnel(ctx)
	if err != nil {
		done <- err
		return
	}
	sends := make(chan tunnelEnvelope, 32)
	go func() {
		<-ctx.Done()
		close(sends)
	}()
	go tunnelWriter(stream, sends)

	for {
		frame, err := stream.Recv()
		if err != nil {
			done <- err
			return
		}
		call := frame.GetCall()
		if call == nil || frame.GetRequestId() == "" {
			continue
		}
		response := c.dispatchHostCall(call)
		envelope := tunnelEnvelope{
			frame: &executorv1.TunnelFrame{RequestId: frame.GetRequestId(), Payload: &executorv1.TunnelFrame_Response{Response: response}},
			errCh: make(chan error, 1),
		}
		select {
		case sends <- envelope:
		default:
			// Writer backlog: drop the response. The calling node times out
			// and fails explicitly instead of stalling the session stream.
		}
	}
}

// tunnelWriter serialises SendMsg on the shared client stream.
func tunnelWriter(stream executorv1.Executor_TunnelClient, sends <-chan tunnelEnvelope) {
	for envelope := range sends {
		err := stream.Send(envelope.frame)
		select {
		case envelope.errCh <- err:
		default:
		}
	}
}

// dispatchHostCall routes one frame to the desktop bridge implementation.
// Errors are reported inside the response so a failing node surfaces its own
// explicit message instead of tearing down the session.
func (c *conn) dispatchHostCall(call *executorv1.HostCall) *executorv1.HostResponse {
	bridge := c.manager.bridge
	if bridge == nil {
		return &executorv1.HostResponse{Ok: false, Error: "host services are unavailable"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), hostCallTimeout)
	defer cancel()

	response := &executorv1.HostResponse{}
	var payload any
	var err error
	switch call.GetKind() {
	case HostCallLLMChat:
		var request pipeline.ChatRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			var reply pipeline.ChatResponse
			reply, err = bridge.LLMChat(ctx, request)
			payload = reply
		}
	case HostCallLLMConverse:
		var request domain.AssistantChatRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			var reply domain.AssistantChatResponse
			reply, err = bridge.LLMConverse(ctx, request)
			payload = reply
		}
	case HostCallCreateReport:
		var report domain.Report
		if err = decode(call.GetPayloadJson(), &report); err == nil {
			payload, err = bridge.CreateReport(ctx, report)
		}
	case HostCallReportGet:
		var id string
		if err = decode(call.GetPayloadJson(), &id); err == nil {
			payload, err = bridge.GetReport(ctx, id)
		}
	case HostCallReportList:
		var request ReportListRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			payload, err = bridge.ListReports(ctx, request.Limit)
		}
	case HostCallChatAppendReply:
		var request ChatAppendReplyRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			payload, err = bridge.AppendChatReply(ctx, request.ChatRunID, request.Content)
		}
	case HostCallChatUpdateStat:
		var request ChatUpdateStatusRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			err = bridge.UpdateChatStatus(ctx, request.ChatRunID, request.Status)
		}
	case HostCallChatReadHistory:
		var request ChatReadHistoryRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			payload, err = bridge.ReadChatHistory(ctx, request.ChatID, request.Limit)
		}
	case HostCallSQLQuery:
		var request domain.SQLRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			payload, err = bridge.ExecuteSQL(ctx, request)
		}
	case HostCallTwitchSend:
		var request domain.TwitchChatMessageRequest
		if err = decode(call.GetPayloadJson(), &request); err == nil {
			payload, err = bridge.SendTwitchChatMessage(ctx, request)
		}
	default:
		return &executorv1.HostResponse{Ok: false, Error: fmt.Sprintf("unknown host call kind %q", call.GetKind())}
	}
	if err != nil {
		response.Ok = false
		response.Error = err.Error()
		return response
	}
	response.Ok = true
	if payload != nil {
		data, encodeErr := encode(payload)
		if encodeErr != nil {
			response.Ok = false
			response.Error = encodeErr.Error()
			return response
		}
		response.PayloadJson = data
	}
	return response
}

// pumpEvents consumes run lifecycle events streamed by the executor.
func (c *conn) pumpEvents(ctx context.Context, done chan<- error) {
	if c.manager.onRunEvent == nil {
		done <- fmt.Errorf("run events are not configured")
		return
	}
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		done <- fmt.Errorf("connection closed")
		return
	}
	stream, err := client.StreamEvents(ctx, &executorv1.StreamEventsRequest{})
	if err != nil {
		done <- err
		return
	}
	handler := c.manager.onRunEvent
	for {
		event, err := stream.Recv()
		if err != nil {
			done <- err
			return
		}
		if snapshot := event.GetRun(); snapshot != nil {
			handler(c.target.ID, ExecutionFromSnapshot(snapshot, c.target.ID))
		}
	}
}
