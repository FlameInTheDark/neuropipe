package executord

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/remoteexec"
)

// executorHost implements the JavaScript np.* surface against executor-local
// state. Pipeline, function, trigger, and execution listings read deployed
// bundles and run records; reports and chat services route to the desktop;
// filesystem and HTTP access execute on the executor machine by design.
type executorHost struct {
	runner   *Runner
	record   RunRecord
	pipeline DeployedPipeline
	http     *http.Client
}

func newExecutorHost(runner *Runner, record RunRecord) nodes.JavaScriptHost {
	bundle, _ := runner.store.GetBundle(record.PipelineID)
	return executorHost{runner: runner, record: record, pipeline: bundle, http: &http.Client{Timeout: 30 * time.Second}}
}

func (host executorHost) ExecutionContext() nodes.JavaScriptExecutionContext {
	return nodes.JavaScriptExecutionContext{PipelineID: host.record.PipelineID, ExecutionID: host.record.ExecutionID}
}

func (host executorHost) ListPipelines(context.Context) ([]domain.PipelineSummary, error) {
	bundles := host.runner.store.ListBundles()
	result := make([]domain.PipelineSummary, 0, len(bundles))
	for _, bundle := range bundles {
		result = append(result, domain.PipelineSummary{
			ID:                bundle.PipelineID,
			Name:              bundle.Name,
			Description:       bundle.Description,
			Icon:              bundle.Icon,
			Status:            domain.PipelineActive,
			PublishedRevision: int(bundle.Revision),
			UpdatedAt:         bundle.DeployedAt,
		})
	}
	return result, nil
}

func (host executorHost) GetPipeline(_ context.Context, id string) (domain.Pipeline, error) {
	bundle, ok := host.runner.store.GetBundle(id)
	if !ok {
		return domain.Pipeline{}, os.ErrNotExist
	}
	return domain.Pipeline{
		ID:                bundle.PipelineID,
		Name:              bundle.Name,
		Description:       bundle.Description,
		Icon:              bundle.Icon,
		Status:            domain.PipelineActive,
		DraftDefinition:   bundle.Definition,
		PublishedRevision: int(bundle.Revision),
	}, nil
}

func (host executorHost) ListFunctions(context.Context) ([]domain.FunctionSummary, error) {
	seen := make(map[string]bool)
	result := make([]domain.FunctionSummary, 0)
	for _, bundle := range host.runner.store.ListBundles() {
		for _, function := range bundle.Functions {
			if seen[function.ID] {
				continue
			}
			seen[function.ID] = true
			result = append(result, domain.FunctionSummary{
				ID:                function.ID,
				Name:              function.Name,
				Description:       function.Description,
				Category:          function.Category,
				Icon:              function.Icon,
				IconColor:         function.IconColor,
				IconBackground:    function.IconBackground,
				PublishedRevision: function.PublishedRevision,
				UpdatedAt:         function.UpdatedAt,
			})
		}
	}
	return result, nil
}

func (host executorHost) ListTriggers(context.Context) ([]domain.TriggerBinding, error) {
	result := make([]domain.TriggerBinding, 0)
	for _, bundle := range host.runner.store.ListBundles() {
		for _, trigger := range bundle.Triggers {
			result = append(result, domain.TriggerBinding{
				ID:         trigger.BindingID,
				PipelineID: bundle.PipelineID,
				NodeID:     trigger.NodeID,
				NodeType:   trigger.NodeType,
				Kind:       domain.TriggerKind(trigger.Kind),
				Label:      trigger.Label,
				Revision:   int(bundle.Revision),
				Cron:       trigger.Cron,
				Timezone:   trigger.Timezone,
				Enabled:    trigger.Enabled,
				Trusted:    trigger.Trusted,
			})
		}
	}
	return result, nil
}

func (host executorHost) ListExecutions(_ context.Context, limit int) ([]domain.Execution, error) {
	records := host.runner.store.RecentRuns(limit)
	result := make([]domain.Execution, 0, len(records))
	for _, record := range records {
		result = append(result, recordToExecution(record))
	}
	return result, nil
}

// Report and chat operations stay desktop-authoritative over the tunnel.

func (host executorHost) ListReports(ctx context.Context, limit int) ([]domain.Report, error) {
	var reports []domain.Report
	request := remoteexec.ReportListRequest{Limit: limit}
	if err := host.runner.tunnel.Call(ctx, remoteexec.HostCallReportList, request, &reports); err != nil {
		return nil, err
	}
	return reports, nil
}

func (host executorHost) GetReport(ctx context.Context, id string) (domain.Report, error) {
	var report domain.Report
	if err := host.runner.tunnel.Call(ctx, remoteexec.HostCallReportGet, id, &report); err != nil {
		return domain.Report{}, err
	}
	return report, nil
}

func (host executorHost) CreateReport(ctx context.Context, nodeID, title, markdown string, tags []string) (domain.Report, error) {
	writer := proxiedReports{tunnel: host.runner.tunnel}
	return writer.CreateReport(ctx, domain.Report{
		PipelineID:   host.record.PipelineID,
		PipelineName: host.pipeline.Name,
		ExecutionID:  host.record.ExecutionID,
		NodeID:       nodeID,
		Title:        title,
		Tags:         tags,
		Markdown:     markdown,
	})
}

func (host executorHost) ReadChatHistory(ctx context.Context, chatID string, limit int) ([]domain.ChatMessage, error) {
	return proxiedChat{tunnel: host.runner.tunnel}.ReadChatHistory(ctx, chatID, limit)
}

func (host executorHost) AppendChatReply(ctx context.Context, chatRunID, content string) (domain.ChatMessage, error) {
	return proxiedChat{tunnel: host.runner.tunnel}.AppendChatReply(ctx, chatRunID, content)
}

func (host executorHost) UpdateChatStatus(ctx context.Context, chatRunID, status string) error {
	return proxiedChat{tunnel: host.runner.tunnel}.UpdateChatStatus(ctx, chatRunID, status)
}

// Filesystem access executes on the executor machine.

func (host executorHost) ListDirectory(ctx context.Context, path string) ([]map[string]any, error) {
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

func (host executorHost) ReadFile(ctx context.Context, path string) ([]byte, error) {
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

func (host executorHost) WriteFile(ctx context.Context, path string, data []byte) (string, error) {
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

func (host executorHost) HTTPRequest(ctx context.Context, request nodes.JavaScriptHTTPRequest) (nodes.JavaScriptHTTPResponse, error) {
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

func (host executorHost) Notify(ctx context.Context, title, message string) error {
	if host.runner.notifier == nil {
		return nil
	}
	return host.runner.notifier.Send(ctx, title, message)
}

var _ nodes.JavaScriptHost = executorHost{}
