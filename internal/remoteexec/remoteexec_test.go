package remoteexec

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExecutionFromSnapshotRoundTrip(t *testing.T) {
	started := time.Now().UTC().Add(-time.Minute)
	runStarted := started.Add(10 * time.Millisecond)
	finished := time.Now().UTC()
	snapshot := &executorv1.RunSnapshot{
		ExecutionId:      "exec-7",
		PipelineId:       "pipe-2",
		TriggerNodeId:    "button",
		TriggerBindingId: "binding-4",
		Revision:         5,
		Status:           string(domain.RunFailed),
		Error:            "boom",
		StartedAt:        timestamppb.New(started),
		RunStartedAt:     timestamppb.New(runStarted),
		FinishedAt:       timestamppb.New(finished),
		NodeRuns: []*executorv1.NodeRun{{
			NodeId:    "button",
			NodeType:  "trigger:button",
			Status:    string(domain.RunCompleted),
			StartedAt: timestamppb.New(runStarted),
		}},
	}
	execution := ExecutionFromSnapshot(snapshot, "exec-host-1")
	if execution.ID != "exec-7" || execution.PipelineID != "pipe-2" || execution.TriggerID != "binding-4" {
		t.Fatalf("execution identity = %#v", execution)
	}
	if execution.Status != domain.RunFailed || execution.Error != "boom" || execution.ExecutorID != "exec-host-1" {
		t.Fatalf("execution state = %#v", execution)
	}
	if execution.RunStartedAt == nil || !execution.RunStartedAt.Equal(runStarted) {
		t.Fatalf("run started = %#v, want %v", execution.RunStartedAt, runStarted)
	}
	if len(execution.NodeRuns) != 1 || execution.NodeRuns[0].NodeID != "button" {
		t.Fatalf("node runs = %#v", execution.NodeRuns)
	}
}

func TestNodeRunToProtoOmitsPreviews(t *testing.T) {
	nodeRun := domain.NodeRun{
		NodeID:   "n1",
		NodeType: "action:http",
		Status:   domain.RunCompleted,
		Input:    map[string]any{"url": "https://example.com"},
		Output:   map[string]any{"body": "secret-ish"},
	}
	// The wire type has no input/output fields at all, so packet previews
	// cannot leak even if a caller forgets to redact. That property is what
	// this test pins down: the conversion below compiles while the struct
	// carries only identity and timing fields.
	proto := NodeRunToProto(nodeRun)
	if proto.NodeId != "n1" || proto.Status != string(domain.RunCompleted) {
		t.Fatalf("proto identity = %+v", proto)
	}
	data, err := json.Marshal(proto)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, forbidden := range []string{"url", "body", "secret-ish"} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("wire form leaks %q: %s", forbidden, data)
		}
	}
}

func TestConfigConversionsPreserveKeySemantics(t *testing.T) {
	config := domain.RemoteExecutorConfig{
		LLMMode: domain.ExecutorLLMLocal,
		Providers: []domain.RemoteExecutorProvider{
			{ID: "p1", Name: "Hosted", Kind: "openai-compatible", BaseURL: "https://x.example.com", Model: "m", Enabled: true, APIKey: "sk-once"},
			{ID: "p2", Name: "Local ollama", Kind: "ollama", BaseURL: "http://127.0.0.1:11434", Model: "", Enabled: true, APIKeySet: true},
		},
		DefaultProviderID: "p1",
		MaxConcurrentRuns: 3,
	}
	proto := configToProto(config)
	if proto.Providers[0].ApiKey != "sk-once" {
		t.Fatalf("write path must carry the new key")
	}
	back := configFromProto(proto)
	if back.Providers[0].APIKeySet != true {
		t.Fatalf("key set flag lost on write round-trip")
	}
	if back.Providers[1].APIKey != "" {
		t.Fatalf("read path must not invent key material")
	}
}
