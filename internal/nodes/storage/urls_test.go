package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// executorStub records the URL requests and returns canned results. The
// transfer methods are unused by the URL nodes and return zero values.
type executorStub struct {
	presignRequest  domain.StoragePresignRequest
	presignResult   domain.StoragePresignResult
	presignErr      error
	publicRequest   domain.StoragePublicURLRequest
	publicResult    domain.StoragePublicURLResult
	publicErr       error
	publicCalled    bool
	presignAttempts int
}

func (e *executorStub) StorageListFiles(context.Context, domain.StorageListRequest) (domain.StorageListResult, error) {
	return domain.StorageListResult{}, nil
}

func (e *executorStub) StorageUploadFile(context.Context, domain.StorageUploadFileRequest) (domain.StorageUploadResult, error) {
	return domain.StorageUploadResult{}, nil
}

func (e *executorStub) StorageUploadData(context.Context, domain.StorageUploadDataRequest) (domain.StorageUploadResult, error) {
	return domain.StorageUploadResult{}, nil
}

func (e *executorStub) StorageDownloadFile(context.Context, domain.StorageDownloadRequest) (domain.StorageDownloadResult, error) {
	return domain.StorageDownloadResult{}, nil
}

func (e *executorStub) StorageDelete(context.Context, domain.StorageDeleteRequest) (domain.StorageDeleteResult, error) {
	return domain.StorageDeleteResult{}, nil
}

func (e *executorStub) StorageMakeDir(context.Context, domain.StorageMakeDirRequest) (domain.StorageMakeDirResult, error) {
	return domain.StorageMakeDirResult{}, nil
}

func (e *executorStub) StorageMove(context.Context, domain.StorageMoveRequest) (domain.StorageMoveResult, error) {
	return domain.StorageMoveResult{}, nil
}

func (e *executorStub) StoragePresignURL(_ context.Context, request domain.StoragePresignRequest) (domain.StoragePresignResult, error) {
	e.presignAttempts++
	e.presignRequest = request
	return e.presignResult, e.presignErr
}

func (e *executorStub) StoragePublicURL(_ context.Context, request domain.StoragePublicURLRequest) (domain.StoragePublicURLResult, error) {
	e.publicCalled = true
	e.publicRequest = request
	return e.publicResult, e.publicErr
}

type runtimeStub struct{ executor nodes.StorageExecutor }

func (r runtimeStub) StorageExecutor() nodes.StorageExecutor { return r.executor }

func invoke(config map[string]any, inputs map[string]any, executor nodes.StorageExecutor, run func(context.Context, nodes.Invocation, nodes.Runtime) (nodes.ExecutionResult, error)) (nodes.ExecutionResult, error) {
	// The engine fills unconnected pins from the node configuration
	// (blueprintState.resolveInputs), so mirror that merge here: wired values
	// win, configuration backs everything else.
	merged := make(map[string]any, len(inputs)+len(config))
	for key, value := range config {
		merged[key] = value
	}
	for key, value := range inputs {
		merged[key] = value
	}
	return run(context.Background(), nodes.Invocation{Config: config, Inputs: merged}, runtimeStub{executor: executor})
}

func TestExecutePresignParsesInputs(t *testing.T) {
	executor := &executorStub{
		presignResult: domain.StoragePresignResult{
			URL: "https://example.com/signed", Method: "PUT", ExpiresInSeconds: 900,
			ExpiresAt: "2026-08-29T12:15:00Z", Headers: map[string]string{"Content-Type": "image/png"},
		},
	}
	result, err := invoke(
		map[string]any{"storageId": "stg-1", "method": "PUT", "path": "reports/chart.png"},
		map[string]any{
			"expires": "15m",
			"headers": "content-type=image/png\n# comment line\nx-amz-meta-owner=neuropipe",
			"params":  "response-content-disposition=attachment; filename=\"chart.png\"",
		},
		executor, executePresign,
	)
	if err != nil {
		t.Fatalf("executePresign() error = %v", err)
	}
	request := executor.presignRequest
	if request.StorageID != "stg-1" || request.Path != "reports/chart.png" || request.Method != "PUT" {
		t.Fatalf("presign request = %#v", request)
	}
	if request.ExpiresSeconds != 900 {
		t.Fatalf("ExpiresSeconds = %d, want 900", request.ExpiresSeconds)
	}
	if request.Headers["content-type"] != "image/png" || request.Headers["x-amz-meta-owner"] != "neuropipe" || len(request.Headers) != 2 {
		t.Fatalf("Headers = %#v", request.Headers)
	}
	if request.Params["response-content-disposition"] != `attachment; filename="chart.png"` {
		t.Fatalf("Params = %#v", request.Params)
	}
	if result.Outputs["url"] != "https://example.com/signed" {
		t.Fatalf("url output = %#v", result.Outputs["url"])
	}
	object := result.Outputs["result"].(map[string]any)
	if object["method"] != "PUT" || object["expiresInSeconds"] != int64(900) || object["expiresAt"] != "2026-08-29T12:15:00Z" {
		t.Fatalf("result output = %#v", object)
	}
	headers := object["headers"].(map[string]any)
	if headers["Content-Type"] != "image/png" {
		t.Fatalf("result headers = %#v", headers)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("Ports = %#v", result.Ports)
	}
}

func TestExecutePresignDefaults(t *testing.T) {
	executor := &executorStub{}
	if _, err := invoke(map[string]any{"storageId": "stg-1", "path": "a.txt"}, nil, executor, executePresign); err != nil {
		t.Fatalf("executePresign(defaults) error = %v", err)
	}
	request := executor.presignRequest
	if request.Method != "GET" {
		t.Fatalf("default Method = %q, want GET", request.Method)
	}
	if request.ExpiresSeconds != 3600 {
		t.Fatalf("default ExpiresSeconds = %d, want 3600", request.ExpiresSeconds)
	}
	if len(request.Headers) != 0 || len(request.Params) != 0 {
		t.Fatalf("default maps = %#v / %#v, want empty", request.Headers, request.Params)
	}
}

func TestExecutePresignRejectsBadInput(t *testing.T) {
	executor := &executorStub{}
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{name: "no storage", config: map[string]any{"path": "a.txt"}, want: "select a storage"},
		{name: "no path", config: map[string]any{"storageId": "stg-1"}, want: "path is required"},
		{
			name:   "bad expiry",
			config: map[string]any{"storageId": "stg-1", "path": "a.txt"},
			inputs: map[string]any{"expires": "quickly"},
			want:   "not a duration",
		},
		{
			name:   "malformed header line",
			config: map[string]any{"storageId": "stg-1", "path": "a.txt"},
			inputs: map[string]any{"headers": "justakey"},
			want:   "key=value",
		},
		{
			name:   "malformed param line",
			config: map[string]any{"storageId": "stg-1", "path": "a.txt"},
			inputs: map[string]any{"params": "=value"},
			want:   "key must not be blank",
		},
	}
	for _, test := range tests {
		_, err := invoke(test.config, test.inputs, executor, executePresign)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: error = %v, want %q", test.name, err, test.want)
		}
	}
	if executor.presignAttempts != 0 {
		t.Fatalf("executor was called %d times despite input errors", executor.presignAttempts)
	}
}

func TestExecutePresignWithoutExecutor(t *testing.T) {
	_, err := executePresign(context.Background(), nodes.Invocation{Config: map[string]any{"storageId": "stg-1", "path": "a.txt"}}, runtimeStub{})
	if err == nil || !strings.Contains(err.Error(), "storage execution is unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecutePublicURL(t *testing.T) {
	executor := &executorStub{
		publicResult: domain.StoragePublicURLResult{URL: "https://cdn.example.com/a.png", Kind: "public-base"},
	}
	result, err := invoke(map[string]any{"storageId": "stg-1", "path": "a.png"}, nil, executor, executePublicURL)
	if err != nil {
		t.Fatalf("executePublicURL() error = %v", err)
	}
	if executor.publicRequest.StorageID != "stg-1" || executor.publicRequest.Path != "a.png" {
		t.Fatalf("public request = %#v", executor.publicRequest)
	}
	if result.Outputs["url"] != "https://cdn.example.com/a.png" {
		t.Fatalf("url output = %#v", result.Outputs["url"])
	}
	object := result.Outputs["result"].(map[string]any)
	if object["kind"] != "public-base" || object["url"] != "https://cdn.example.com/a.png" {
		t.Fatalf("result output = %#v", object)
	}

	// The root path is allowed (empty string), unlike the presign node.
	executor = &executorStub{}
	if _, err := invoke(map[string]any{"storageId": "stg-1"}, nil, executor, executePublicURL); err != nil {
		t.Fatalf("executePublicURL(root) error = %v", err)
	}
	if !executor.publicCalled || executor.publicRequest.Path != "" {
		t.Fatalf("public request = %#v, called = %v", executor.publicRequest, executor.publicCalled)
	}

	// Executor errors flow through untouched.
	executor = &executorStub{publicErr: context.Canceled}
	if _, err := invoke(map[string]any{"storageId": "stg-1"}, nil, executor, executePublicURL); err == nil {
		t.Fatal("executePublicURL() expected the executor error")
	}
}

func TestParseExpirySeconds(t *testing.T) {
	tests := []struct {
		raw  string
		want int64
	}{
		{"", 3600},
		{"3600", 3600},
		{"90s", 90},
		{"15m", 900},
		{"1h30m", 5400},
		{"7d", 604800},
		{"0.5d", 43200},
		{"  10m ", 600},
	}
	for _, test := range tests {
		got, err := ParseExpirySeconds(test.raw)
		if err != nil || got != test.want {
			t.Fatalf("ParseExpirySeconds(%q) = %d, %v; want %d", test.raw, got, err, test.want)
		}
	}
	for _, invalid := range []string{"soon", "m15", "1hour", "1h_"} {
		if _, err := ParseExpirySeconds(invalid); err == nil {
			t.Fatalf("ParseExpirySeconds(%q) expected an error", invalid)
		}
	}
}

func TestParseKeyValueLines(t *testing.T) {
	parsed, err := ParseKeyValueLines("A=1\r\n  b = two  \n\n# comment\nx=y=z", "params")
	if err != nil {
		t.Fatalf("ParseKeyValueLines() error = %v", err)
	}
	if len(parsed) != 3 || parsed["A"] != "1" || parsed["b"] != "two" || parsed["x"] != "y=z" {
		t.Fatalf("parsed = %#v", parsed)
	}
	if empty, err := ParseKeyValueLines("  \n#", "params"); err != nil || len(empty) != 0 {
		t.Fatalf("ParseKeyValueLines(blank) = %#v, %v", empty, err)
	}
	if _, err := ParseKeyValueLines("no-separator", "headers"); err == nil || !strings.Contains(err.Error(), "key=value") {
		t.Fatalf("ParseKeyValueLines(malformed) error = %v", err)
	}
	if _, err := ParseKeyValueLines("=value", "headers"); err == nil {
		t.Fatal("ParseKeyValueLines(blank key) expected an error")
	}
}
