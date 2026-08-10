package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestListDirectoryFlowsIntoForEachWithTypedRecords(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "report.txt"), []byte("ready"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "list", Type: "action:list_directory", Data: map[string]any{"config": map[string]any{"path": directory}}},
			{ID: "each", Type: "flow:for_each", Data: map[string]any{"config": map[string]any{}}},
		},
		Edges: []domain.FlowEdge{
			execEdge("start-list", "start", "out", "list", "in"),
			execEdge("list-each", "list", "out", "each", "in"),
			dataEdge("list-each-items", "list", "result", "each", "items"),
		},
	}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var listed map[string]any
	for _, run := range result.NodeRuns {
		if run.NodeID == "list" {
			var ok bool
			listed, ok = run.Output.(map[string]any)
			if !ok {
				t.Fatalf("List Directory run output type = %T, want map[string]any", run.Output)
			}
		}
	}
	files, ok := listed["result"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("List Directory output = %#v", listed)
	}
	entry, ok := files[0].(map[string]any)
	if !ok || entry["name"] != "report.txt" || entry["type"] != "file" {
		t.Fatalf("List Directory entry = %#v", files[0])
	}
}

func TestReadFileBytesFlowIntoBytesWriteFile(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.bin")
	target := filepath.Join(directory, "target.bin")
	want := []byte{0xff, 0x00, 0x01, 0x7f}
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}

	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "read", Type: "action:file_read", Data: map[string]any{"config": map[string]any{"path": source, "outputType": "bytes"}}},
			{ID: "write", Type: "action:file_write", Data: map[string]any{"config": map[string]any{"path": target, "contentType": "bytes"}}},
		},
		Edges: []domain.FlowEdge{
			execEdge("start-read", "start", "out", "read", "in"),
			execEdge("read-write", "read", "out", "write", "in"),
			dataEdge("read-content", "read", "result", "write", "content"),
		},
	}
	if err := Validate(flow, catalog.New()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("target = %v, want %v", got, want)
	}
}

func TestValidatorRejectsBytesReadIntoTextWrite(t *testing.T) {
	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "start", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "read", Type: "action:file_read", Data: map[string]any{"config": map[string]any{"path": "C:\\source.bin", "outputType": "bytes"}}},
			{ID: "write", Type: "action:file_write", Data: map[string]any{"config": map[string]any{"path": "C:\\target.txt", "contentType": "text"}}},
		},
		Edges: []domain.FlowEdge{
			execEdge("start-read", "start", "out", "read", "in"),
			execEdge("read-write", "read", "out", "write", "in"),
			dataEdge("read-content", "read", "result", "write", "content"),
		},
	}
	if err := Validate(flow, catalog.New()); err == nil {
		t.Fatal("Validate() error = nil, want strict bytes-to-text rejection")
	}
}
