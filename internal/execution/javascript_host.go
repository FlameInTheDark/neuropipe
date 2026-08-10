package execution

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// javascriptHost adapts application services to the small JavaScript host
// port. It is created for one execution, so reports and execution history are
// always scoped to the pipeline currently running.
type javascriptHost struct {
	store       *persistence.Store
	reports     pipeline.ReportWriter
	chat        pipeline.ChatWriter
	notifier    pipeline.NotificationSender
	pipelineID  string
	executionID string
	http        *http.Client
}

func newJavaScriptHost(store *persistence.Store, reports pipeline.ReportWriter, chat pipeline.ChatWriter, notifier pipeline.NotificationSender, pipelineID, executionID string) nodes.JavaScriptHost {
	return javascriptHost{store: store, reports: reports, chat: chat, notifier: notifier, pipelineID: pipelineID, executionID: executionID, http: &http.Client{Timeout: 25 * time.Second}}
}

func (host javascriptHost) ExecutionContext() nodes.JavaScriptExecutionContext {
	return nodes.JavaScriptExecutionContext{PipelineID: host.pipelineID, ExecutionID: host.executionID}
}

func (host javascriptHost) ListPipelines(ctx context.Context) ([]domain.PipelineSummary, error) {
	return host.store.ListPipelines(ctx)
}

func (host javascriptHost) GetPipeline(ctx context.Context, id string) (domain.Pipeline, error) {
	return host.store.GetPipeline(ctx, id)
}

func (host javascriptHost) ListFunctions(ctx context.Context) ([]domain.FunctionSummary, error) {
	return host.store.ListFunctions(ctx)
}

func (host javascriptHost) ListTriggers(ctx context.Context) ([]domain.TriggerBinding, error) {
	return host.store.ListAllTriggers(ctx)
}

func (host javascriptHost) ListExecutions(ctx context.Context, limit int) ([]domain.Execution, error) {
	return host.store.ListExecutions(ctx, host.pipelineID, limit)
}

func (host javascriptHost) ListReports(ctx context.Context, limit int) ([]domain.Report, error) {
	return host.store.ListReports(ctx, limit)
}

func (host javascriptHost) GetReport(ctx context.Context, id string) (domain.Report, error) {
	return host.store.GetReport(ctx, id)
}

func (host javascriptHost) CreateReport(ctx context.Context, nodeID, title, markdown string, tags []string) (domain.Report, error) {
	if host.reports == nil || host.pipelineID == "" || host.executionID == "" {
		return domain.Report{}, fmt.Errorf("report storage is unavailable for this execution")
	}
	return host.reports.CreateReport(ctx, domain.Report{PipelineID: host.pipelineID, ExecutionID: host.executionID, NodeID: nodeID, Title: title, Tags: tags, Markdown: markdown})
}

func (host javascriptHost) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	if host.chat == nil {
		return nil, fmt.Errorf("chat history is unavailable for this execution")
	}
	return host.chat.ReadChatHistory(ctx, chatID, limit)
}

func (host javascriptHost) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	if host.chat == nil {
		return domain.ChatMessage{}, fmt.Errorf("chat delivery is unavailable for this execution")
	}
	return host.chat.AppendChatReply(ctx, chatRunID, content)
}

func (host javascriptHost) UpdateChatStatus(ctx context.Context, chatRunID, status string) error {
	if host.chat == nil {
		return fmt.Errorf("chat delivery is unavailable for this execution")
	}
	return host.chat.UpdateChatStatus(ctx, chatRunID, status)
}

func (host javascriptHost) ListDirectory(ctx context.Context, path string) ([]map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		entryType := "file"
		if entry.Type()&os.ModeSymlink != 0 {
			entryType = "symlink"
		} else if info.IsDir() {
			entryType = "directory"
		}
		item := map[string]any{"name": entry.Name(), "path": filepath.Join(path, entry.Name()), "size": info.Size(), "type": entryType, "updatedAt": info.ModTime().UTC().Format(time.RFC3339Nano)}
		result = append(result, item)
	}
	return result, nil
}

func (host javascriptHost) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func (host javascriptHost) WriteFile(ctx context.Context, path string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(cleanPath, data, 0o600); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func (host javascriptHost) HTTPRequest(ctx context.Context, request nodes.JavaScriptHTTPRequest) (nodes.JavaScriptHTTPResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, strings.TrimSpace(request.URL), strings.NewReader(string(request.Body)))
	if err != nil {
		return nodes.JavaScriptHTTPResponse{}, err
	}
	for name, values := range request.Headers {
		for _, value := range values {
			httpRequest.Header.Add(name, value)
		}
	}
	response, err := host.http.Do(httpRequest)
	if err != nil {
		return nodes.JavaScriptHTTPResponse{}, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 5*1024*1024))
	if err != nil {
		return nodes.JavaScriptHTTPResponse{}, err
	}
	return nodes.JavaScriptHTTPResponse{Status: response.StatusCode, Headers: response.Header, Body: body}, nil
}

func (host javascriptHost) Notify(ctx context.Context, title, message string) error {
	if host.notifier == nil {
		return nil
	}
	return host.notifier.Send(ctx, title, message)
}
