package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// uploadExecutorStub records upload requests so the mode-aware executor can be
// verified without a real storage connection.
type uploadExecutorStub struct {
	fileRequest domain.StorageUploadFileRequest
	fileCalls   int
	fileResult  domain.StorageUploadResult
	dataRequest domain.StorageUploadDataRequest
	dataCalls   int
	dataResult  domain.StorageUploadResult
}

func (e *uploadExecutorStub) StorageListFiles(context.Context, domain.StorageListRequest) (domain.StorageListResult, error) {
	return domain.StorageListResult{}, nil
}

func (e *uploadExecutorStub) StorageUploadFile(_ context.Context, request domain.StorageUploadFileRequest) (domain.StorageUploadResult, error) {
	e.fileCalls++
	e.fileRequest = request
	return e.fileResult, nil
}

func (e *uploadExecutorStub) StorageUploadData(_ context.Context, request domain.StorageUploadDataRequest) (domain.StorageUploadResult, error) {
	e.dataCalls++
	e.dataRequest = request
	return e.dataResult, nil
}

func (e *uploadExecutorStub) StorageDownloadFile(context.Context, domain.StorageDownloadRequest) (domain.StorageDownloadResult, error) {
	return domain.StorageDownloadResult{}, nil
}

func (e *uploadExecutorStub) StorageDelete(context.Context, domain.StorageDeleteRequest) (domain.StorageDeleteResult, error) {
	return domain.StorageDeleteResult{}, nil
}

func (e *uploadExecutorStub) StorageMakeDir(context.Context, domain.StorageMakeDirRequest) (domain.StorageMakeDirResult, error) {
	return domain.StorageMakeDirResult{}, nil
}

func (e *uploadExecutorStub) StorageMove(context.Context, domain.StorageMoveRequest) (domain.StorageMoveResult, error) {
	return domain.StorageMoveResult{}, nil
}

func (e *uploadExecutorStub) StoragePresignURL(context.Context, domain.StoragePresignRequest) (domain.StoragePresignResult, error) {
	return domain.StoragePresignResult{}, nil
}

func (e *uploadExecutorStub) StoragePublicURL(context.Context, domain.StoragePublicURLRequest) (domain.StoragePublicURLResult, error) {
	return domain.StoragePublicURLResult{}, nil
}

func uploadInvoke(config map[string]any, inputs map[string]any, executor nodes.StorageExecutor) (nodes.ExecutionResult, error) {
	return invoke(config, inputs, executor, executeUploadFile)
}

func uploadInputs(entries map[string]any) map[string]any {
	merged := map[string]any{"storageId": "stg-1", "remotePath": "reports/chart.png"}
	for key, value := range entries {
		merged[key] = value
	}
	return merged
}

/* ---------------- definition ---------------- */

func TestUploadFileDefinitionShape(t *testing.T) {
	definition := uploadFileDefinition()
	pins := map[string]bool{}
	for _, port := range definition.Inputs {
		pins[port.ID] = true
	}
	for _, id := range []string{"localPath", "data", "base64", "remotePath", "contentType"} {
		if !pins[id] {
			t.Fatalf("input pin %q missing from definition", id)
		}
	}
	dataPin := domain.NodePort{}
	for _, port := range definition.Inputs {
		if port.ID == "data" {
			dataPin = port
		}
	}
	if dataPin.DataType != domain.DataBytes || dataPin.Type == nil || dataPin.Type.Kind != domain.TypeBytes {
		t.Fatalf("data pin = %#v, want a bytes pin", dataPin)
	}
	fields := map[string]domain.ConfigField{}
	for _, field := range definition.Fields {
		fields[field.Name] = field
	}
	source, ok := fields["source"]
	if !ok {
		t.Fatal("source field missing")
	}
	if source.Kind != "select" || len(source.Options) != 4 {
		t.Fatalf("source field = %#v", source)
	}
	if source.Options[0].Value != "" || source.Options[1].Value != "file" || source.Options[2].Value != "bytes" || source.Options[3].Value != "base64" {
		t.Fatalf("source options = %#v", source.Options)
	}
	if want := "source=file|source="; fields["localPath"].VisibleWhen != want {
		t.Fatalf("localPath VisibleWhen = %q, want %q", fields["localPath"].VisibleWhen, want)
	}
	if want := "source=base64|source="; fields["base64"].VisibleWhen != want {
		t.Fatalf("base64 VisibleWhen = %q, want %q", fields["base64"].VisibleWhen, want)
	}
	if fields["remotePath"].Required != true {
		t.Fatal("remotePath must stay required")
	}
	if definition.DefaultConfig["source"] != "" {
		t.Fatalf("default source = %#v, want Auto", definition.DefaultConfig["source"])
	}
}

/* ---------------- resolver ---------------- */

func flowNodeWithSource(source any) domain.FlowNode {
	return domain.FlowNode{
		ID:   "n1",
		Type: "action:storage_upload_file",
		Data: map[string]any{"config": map[string]any{"source": source}},
	}
}

func TestResolveUploadFiltersPinsPerMode(t *testing.T) {
	tests := []struct {
		name   string
		source any
		want   []string
	}{
		{"auto keeps every pin", "", []string{"in", "localPath", "data", "base64", "remotePath", "contentType"}},
		{"missing key reads as auto", nil, []string{"in", "localPath", "data", "base64", "remotePath", "contentType"}},
		{"unknown value reads as auto", "url", []string{"in", "localPath", "data", "base64", "remotePath", "contentType"}},
		{"disk keeps localPath only", "file", []string{"in", "localPath", "remotePath", "contentType"}},
		{"node keeps data only", "bytes", []string{"in", "data", "remotePath", "contentType"}},
		{"base64 keeps base64 only", "base64", []string{"in", "base64", "remotePath", "contentType"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := resolveUpload(flowNodeWithSource(test.source))
			if err != nil {
				t.Fatalf("resolveUpload() error = %v", err)
			}
			got := make([]string, 0, len(resolved.Inputs))
			for _, port := range resolved.Inputs {
				got = append(got, port.ID)
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("pins = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResolveUploadReadsFlatNodeData(t *testing.T) {
	// Graphs saved outside the editor may store config keys directly on Data.
	resolved, err := resolveUpload(domain.FlowNode{Type: "action:storage_upload_file", Data: map[string]any{"source": "file"}})
	if err != nil {
		t.Fatalf("resolveUpload() error = %v", err)
	}
	for _, port := range resolved.Inputs {
		if port.ID == "data" || port.ID == "base64" {
			t.Fatalf("pin %q should be filtered in disk mode", port.ID)
		}
	}
}

/* ---------------- executor: explicit modes ---------------- */

func TestExecuteUploadDiskModeStreamsFile(t *testing.T) {
	executor := &uploadExecutorStub{fileResult: domain.StorageUploadResult{Key: "reports/chart.png", Size: 128, Driver: "s3"}}
	result, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "source": "file", "remotePath": "reports/chart.png", "contentType": "image/png"},
		map[string]any{"localPath": "/tmp/chart.png", "base64": "aGVsbG8="},
		executor,
	)
	if err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.fileCalls != 1 || executor.dataCalls != 0 {
		t.Fatalf("calls = file %d / data %d, want 1/0", executor.fileCalls, executor.dataCalls)
	}
	if executor.fileRequest.LocalPath != "/tmp/chart.png" || executor.fileRequest.StorageID != "stg-1" || executor.fileRequest.ContentType != "image/png" {
		t.Fatalf("file request = %#v", executor.fileRequest)
	}
	// base64 is set but the disk mode must ignore it (stale hidden values).
	object := result.Outputs["result"].(map[string]any)
	if object["key"] != "reports/chart.png" || object["size"] != int64(128) || object["driver"] != "s3" {
		t.Fatalf("result = %#v", object)
	}
}

func TestExecuteUploadNodeModeSendsBytes(t *testing.T) {
	executor := &uploadExecutorStub{dataResult: domain.StorageUploadResult{Key: "reports/out.bin", Size: 5, Driver: "ftp"}}
	result, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "source": "bytes", "remotePath": "reports/out.bin"},
		map[string]any{"data": []byte("hello")},
		executor,
	)
	if err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.fileCalls != 0 || executor.dataCalls != 1 {
		t.Fatalf("calls = file %d / data %d, want 0/1", executor.fileCalls, executor.dataCalls)
	}
	if string(executor.dataRequest.Data) != "hello" || executor.dataRequest.StorageID != "stg-1" {
		t.Fatalf("data request = %#v", executor.dataRequest)
	}
	object := result.Outputs["result"].(map[string]any)
	if object["driver"] != "ftp" || object["size"] != int64(5) {
		t.Fatalf("result = %#v", object)
	}
}

func TestExecuteUploadBase64ModeDecodesText(t *testing.T) {
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "source": "base64", "remotePath": "reports/a.txt", "base64": "aGVsbG8gd29ybGQ="},
		nil, executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.dataCalls != 1 || string(executor.dataRequest.Data) != "hello world" {
		t.Fatalf("data request = %#v", executor.dataRequest)
	}
}

func TestExecuteUploadBase64ModeAcceptsDataURL(t *testing.T) {
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "source": "base64", "remotePath": "reports/a.txt", "base64": "data:text/plain;base64,aGVsbG8="},
		nil, executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if string(executor.dataRequest.Data) != "hello" {
		t.Fatalf("data = %q", executor.dataRequest.Data)
	}
}

func TestExecuteUploadModeErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"disk without path", map[string]any{"storageId": "stg-1", "source": "file", "remotePath": "a"}, nil, "local path is required when the upload source is From disk"},
		{"bytes without data", map[string]any{"storageId": "stg-1", "source": "bytes", "remotePath": "a"}, nil, "data is required when the upload source is From node"},
		{"bytes with blank data", map[string]any{"storageId": "stg-1", "source": "bytes", "remotePath": "a"}, map[string]any{"data": " "}, "data is required when the upload source is From node"},
		{"base64 without value", map[string]any{"storageId": "stg-1", "source": "base64", "remotePath": "a"}, nil, "base64 is required when the upload source is From base64"},
		{"base64 invalid", map[string]any{"storageId": "stg-1", "source": "base64", "remotePath": "a", "base64": "!!!not-base64!!!"}, nil, "value must be base64 text"},
		{"disk mode ignores base64-only config", map[string]any{"storageId": "stg-1", "source": "file", "remotePath": "a", "base64": "aGVsbG8="}, nil, "local path is required when the upload source is From disk"},
		{"missing remote path", map[string]any{"storageId": "stg-1", "source": "file", "localPath": "/tmp/a"}, nil, "remote path is required"},
		{"missing storage", map[string]any{"source": "file", "remotePath": "a"}, nil, "select a storage first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := uploadInvoke(test.config, test.inputs, &uploadExecutorStub{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

/* ---------------- executor: auto mode ---------------- */

func TestExecuteUploadAutoPrefersDiskFile(t *testing.T) {
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "remotePath": "r/a.png"},
		map[string]any{"localPath": "/tmp/a.png", "data": []byte("bytes"), "base64": "aGVsbG8="},
		executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.fileCalls != 1 || executor.dataCalls != 0 || executor.fileRequest.LocalPath != "/tmp/a.png" {
		t.Fatalf("calls = file %d / data %d", executor.fileCalls, executor.dataCalls)
	}
}

func TestExecuteUploadAutoFallsBackToBytes(t *testing.T) {
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "remotePath": "r/a.png", "base64": "aGVsbG8="},
		map[string]any{"data": []byte("raw-bytes")},
		executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.dataCalls != 1 || string(executor.dataRequest.Data) != "raw-bytes" {
		t.Fatalf("data request = %#v", executor.dataRequest)
	}
}

func TestExecuteUploadAutoFallsBackToBase64(t *testing.T) {
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "remotePath": "r/a.txt", "base64": "aGVsbG8="},
		nil, executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.dataCalls != 1 || string(executor.dataRequest.Data) != "hello" {
		t.Fatalf("data request = %#v", executor.dataRequest)
	}
}

func TestExecuteUploadAutoDecodesBase64OnDataPin(t *testing.T) {
	// The legacy Upload Data shape: raw base64 text wired into the data pin.
	executor := &uploadExecutorStub{}
	if _, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "remotePath": "r/a.txt"},
		map[string]any{"data": "aGVsbG8="},
		executor,
	); err != nil {
		t.Fatalf("executeUploadFile() error = %v", err)
	}
	if executor.dataCalls != 1 || string(executor.dataRequest.Data) != "hello" {
		t.Fatalf("data request = %#v", executor.dataRequest)
	}
}

func TestExecuteUploadAutoRequiresAnySource(t *testing.T) {
	_, err := uploadInvoke(map[string]any{"storageId": "stg-1", "remotePath": "r/a.png"}, nil, &uploadExecutorStub{})
	if err == nil || !strings.Contains(err.Error(), "a local path, data, or base64 is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteUploadAutoErrorsOnGarbageDataPin(t *testing.T) {
	// A wired data pin carrying undecodable text is a wiring mistake, not a
	// silent fall-through to base64.
	_, err := uploadInvoke(
		map[string]any{"storageId": "stg-1", "remotePath": "r/a.png"},
		map[string]any{"data": "not base64 !!"},
		&uploadExecutorStub{},
	)
	if err == nil || !strings.Contains(err.Error(), `pin "data" requires bytes or base64 text`) {
		t.Fatalf("error = %v", err)
	}
}

/* ---------------- helpers ---------------- */

func TestUploadSourceModeNormalises(t *testing.T) {
	tests := map[any]string{
		"file": "file", "bytes": "bytes", "base64": "base64",
		"": "", nil: "", "url": "", "disk": "", 42: "",
	}
	for value, want := range tests {
		if got := uploadSourceMode(value); got != want {
			t.Fatalf("uploadSourceMode(%#v) = %q, want %q", value, got, want)
		}
	}
}

func TestBase64Text(t *testing.T) {
	if data, err := Base64Text(""); err != nil || data != nil {
		t.Fatalf("Base64Text(\"\") = %#v, %v", data, err)
	}
	if data, err := Base64Text("   "); err != nil || data != nil {
		t.Fatalf("Base64Text(blank) = %#v, %v", data, err)
	}
	if data, err := Base64Text("aGVsbG8="); err != nil || string(data) != "hello" {
		t.Fatalf("Base64Text(plain) = %#v, %v", data, err)
	}
	if data, err := Base64Text("data:image/png;base64,aGVsbG8="); err != nil || string(data) != "hello" {
		t.Fatalf("Base64Text(data URL) = %#v, %v", data, err)
	}
	if _, err := Base64Text("***"); err == nil {
		t.Fatal("Base64Text(garbage) should error")
	}
}
