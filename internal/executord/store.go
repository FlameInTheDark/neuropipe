package executord

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// atomicWrite writes data through a sibling temp file so a crash never
// leaves a truncated record behind.
func atomicWrite(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// DeployedPipeline is one published bundle stored on the executor. It is the
// complete execution contract: graph, functions, triggers with their trust
// decisions, and the capability allowlist captured at publish time.
type DeployedPipeline struct {
	PipelineID             string                    `json:"pipelineId"`
	Name                   string                    `json:"name"`
	Description            string                    `json:"description,omitempty"`
	Icon                   string                    `json:"icon,omitempty"`
	Revision               int64                     `json:"revision"`
	BlueprintSchemaVersion int32                     `json:"blueprintSchemaVersion"`
	Definition             domain.FlowDefinition     `json:"definition"`
	Functions              []domain.CustomFunction   `json:"functions,omitempty"`
	Triggers               []DeployedTriggerSnapshot `json:"triggers"`
	Capabilities           []string                  `json:"capabilities"`
	DeployedAt             time.Time                 `json:"deployedAt"`
}

// DeployedTriggerSnapshot mirrors executorv1.DeployedTrigger in JSON so
// bundles survive restarts without a proto dependency on disk.
type DeployedTriggerSnapshot struct {
	BindingID      string `json:"bindingId"`
	NodeID         string `json:"nodeId"`
	NodeType       string `json:"nodeType,omitempty"`
	Kind           string `json:"kind"`
	Label          string `json:"label,omitempty"`
	Enabled        bool   `json:"enabled"`
	Trusted        bool   `json:"trusted"`
	Cron           string `json:"cron,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	WatchPath      string `json:"watchPath,omitempty"`
	WatchRecursive bool   `json:"watchRecursive,omitempty"`
}

// RunRecord is one execution history entry stored on the executor.
type RunRecord struct {
	ExecutionID      string           `json:"executionId"`
	PipelineID       string           `json:"pipelineId"`
	Name             string           `json:"name,omitempty"`
	Revision         int64            `json:"revision"`
	TriggerNodeID    string           `json:"triggerNodeId,omitempty"`
	TriggerBindingID string           `json:"triggerBindingId,omitempty"`
	ChatRunID        string           `json:"chatRunId,omitempty"`
	Status           domain.RunStatus `json:"status"`
	Error            string           `json:"error,omitempty"`
	StartedAt        time.Time        `json:"startedAt"`
	RunStartedAt     *time.Time       `json:"runStartedAt,omitempty"`
	FinishedAt       *time.Time       `json:"finishedAt,omitempty"`
	NodeRuns         []domain.NodeRun `json:"nodeRuns,omitempty"`
}

// store persists deployed bundles and run records as JSON files under the
// executor data directory. File-per-record keeps writes atomic and makes
// cross-compiling trivial (no cgo SQLite).
type store struct {
	dataDir string
	mu      sync.RWMutex
}

// NewStore creates the bundle/run record store.
func NewStore(dataDir string) *store { return &store{dataDir: dataDir} }

func bundlePath(dataDir, pipelineID string) string {
	return filepath.Join(dataDir, "deployed", sanitize(pipelineID)+".json")
}

func runPath(dataDir, executionID string) string {
	return filepath.Join(dataDir, "runs", sanitize(executionID)+".json")
}

// sanitize prevents traversal through user-controlled identifiers.
func sanitize(id string) string {
	var builder strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

func (s *store) Deploy(bundle DeployedPipeline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bundle.DeployedAt = time.Now().UTC()
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("encode deployment: %w", err)
	}
	if err := atomicWrite(bundlePath(s.dataDir, bundle.PipelineID), data); err != nil {
		return fmt.Errorf("persist deployment: %w", err)
	}
	return nil
}

func (s *store) Undeploy(pipelineID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(bundlePath(s.dataDir, pipelineID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove deployment: %w", err)
	}
	return nil
}

func (s *store) GetBundle(pipelineID string) (DeployedPipeline, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readBundle(bundlePath(s.dataDir, pipelineID))
}

func (s *store) ListBundles() []DeployedPipeline {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(s.dataDir, "deployed"))
	if err != nil {
		return nil
	}
	bundles := make([]DeployedPipeline, 0, len(entries))
	for _, entry := range entries {
		if bundle, ok := readBundle(filepath.Join(s.dataDir, "deployed", entry.Name())); ok {
			bundles = append(bundles, bundle)
		}
	}
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })
	return bundles
}

func readBundle(path string) (DeployedPipeline, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DeployedPipeline{}, false
	}
	var bundle DeployedPipeline
	if err := json.Unmarshal(data, &bundle); err != nil {
		return DeployedPipeline{}, false
	}
	return bundle, true
}

func (s *store) SaveRun(record RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode run record: %w", err)
	}
	if err := atomicWrite(runPath(s.dataDir, record.ExecutionID), data); err != nil {
		return fmt.Errorf("persist run record: %w", err)
	}
	return nil
}

func (s *store) GetRun(executionID string) (RunRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(runPath(s.dataDir, executionID))
	if err != nil {
		return RunRecord{}, false
	}
	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return RunRecord{}, false
	}
	return record, true
}

// recordToExecution converts a stored record into the shared Execution shape.
func recordToExecution(record RunRecord) domain.Execution {
	execution := domain.Execution{
		ID:           record.ExecutionID,
		PipelineID:   record.PipelineID,
		TriggerID:    record.TriggerBindingID,
		Status:       record.Status,
		Error:        record.Error,
		StartedAt:    record.StartedAt,
		RunStartedAt: record.RunStartedAt,
		FinishedAt:   record.FinishedAt,
		NodeRuns:     record.NodeRuns,
	}
	return execution
}

// RecentRuns returns up to limit records, newest first.
func (s *store) RecentRuns(limit int) []RunRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(s.dataDir, "runs"))
	if err != nil {
		return nil
	}
	records := make([]RunRecord, 0, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(s.dataDir, "runs", entry.Name()))
		if err != nil {
			continue
		}
		var record RunRecord
		if json.Unmarshal(data, &record) == nil {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.After(records[j].StartedAt) })
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records
}
