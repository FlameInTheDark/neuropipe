package execution

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

type reportWriterStub struct {
	report domain.Report
	err    error
}

func (s reportWriterStub) CreateReport(context.Context, domain.Report) (domain.Report, error) {
	return s.report, s.err
}

func TestLimiterQueuesUntilContextIsCancelled(t *testing.T) {
	queue := newLimiter(1)
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() first slot error = %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queue.Acquire(cancelled); err == nil {
		t.Fatal("Acquire() while queue is full error = nil, want cancellation")
	}

	queue.Release()
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	queue.Release()
}

func TestLimiterNormalizesInvalidCapacity(t *testing.T) {
	queue := newLimiter(0)
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() with normalized capacity error = %v", err)
	}
	queue.Release()
}

func TestRunDraftExecutesAndPersistsConnectedNodes(t *testing.T) {
	store, err := persistence.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	definition := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{ID: "button", Type: "trigger:button", Data: map[string]any{"config": map[string]any{"label": "Run"}}},
			{ID: "set", Type: "action:notification", Data: map[string]any{"config": map[string]any{"title": "Done", "message": "done"}}},
		},
		Edges: []domain.FlowEdge{{ID: "button-to-set", Source: "button", SourceHandle: "out", Target: "set", TargetHandle: "in", Kind: domain.PinExec}},
	}
	item, err := store.CreatePipeline(context.Background(), "Draft run", "", definition)
	if err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	service := NewService(store, catalog.New(), nil, nil)

	execution, err := service.RunDraft(context.Background(), item.ID, "button", definition, pipeline.Packet{"trigger": "manual"})
	if err != nil {
		t.Fatalf("RunDraft() error = %v", err)
	}
	if execution.Status != domain.RunCompleted || len(execution.NodeRuns) != 2 {
		t.Fatalf("RunDraft() = %#v, want completed execution with two node runs", execution)
	}
	history, err := store.ListExecutions(context.Background(), item.ID, 20)
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if len(history) != 1 || len(history[0].NodeRuns) != 2 {
		t.Fatalf("execution history = %#v, want persisted node results", history)
	}
}

func TestEmittingReportWriterNotifiesOnlyAfterPersisting(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		writerErr error
		wantEvent bool
	}{
		{name: "created", wantEvent: true},
		{name: "writer failed", writerErr: errors.New("disk unavailable")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var events []string
			writer := emittingReportWriter{
				writer: reportWriterStub{report: domain.Report{ID: "report-1"}, err: test.writerErr},
				emit:   func(event string, _ any) { events = append(events, event) },
			}

			created, err := writer.CreateReport(context.Background(), domain.Report{Title: "Daily update"})
			if !errors.Is(err, test.writerErr) {
				t.Fatalf("CreateReport() error = %v, want %v", err, test.writerErr)
			}
			if created.ID != "report-1" {
				t.Fatalf("CreateReport() report ID = %q, want report-1", created.ID)
			}
			if got := len(events) == 1 && events[0] == "reports.updated"; got != test.wantEvent {
				t.Fatalf("events = %#v, want reports.updated emitted: %v", events, test.wantEvent)
			}
		})
	}
}
