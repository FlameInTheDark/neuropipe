package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/Masterminds/squirrel"
)

// ListRemoteExecutors returns every registered remote executor.
func (s *Store) ListRemoteExecutors(ctx context.Context) ([]domain.RemoteExecutor, error) {
	rows, err := statements(s.db).Select("id", "name", "address", "token_ref", "use_tls", "llm_mode", "created_at", "updated_at").From("remote_executors").OrderBy("name ASC").QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list remote executors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.RemoteExecutor, 0)
	for rows.Next() {
		item, err := scanRemoteExecutor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetRemoteExecutor loads one registration by ID.
func (s *Store) GetRemoteExecutor(ctx context.Context, id string) (domain.RemoteExecutor, error) {
	rows, err := statements(s.db).Select("id", "name", "address", "token_ref", "use_tls", "llm_mode", "created_at", "updated_at").From("remote_executors").Where(squirrel.Eq{"id": id}).QueryContext(ctx)
	if err != nil {
		return domain.RemoteExecutor{}, fmt.Errorf("get remote executor: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return domain.RemoteExecutor{}, fmt.Errorf("remote executor %q not found", id)
	}
	executor, err := scanRemoteExecutor(rows)
	if err != nil {
		return domain.RemoteExecutor{}, err
	}
	return executor, rows.Err()
}

func scanRemoteExecutor(rows *sql.Rows) (domain.RemoteExecutor, error) {
	var item domain.RemoteExecutor
	var useTLS int
	var created, updated string
	if err := rows.Scan(&item.ID, &item.Name, &item.Address, &item.TokenRef, &useTLS, &item.LLMMode, &created, &updated); err != nil {
		return domain.RemoteExecutor{}, fmt.Errorf("scan remote executor: %w", err)
	}
	item.UseTLS = useTLS == 1
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}

// SaveRemoteExecutor upserts one registration. TokenRef only identifies a
// vault record; the shared secret itself never reaches this store.
func (s *Store) SaveRemoteExecutor(ctx context.Context, executor domain.RemoteExecutor) error {
	now := time.Now().UTC()
	useTLS := 0
	if executor.UseTLS {
		useTLS = 1
	}
	_, err := statements(s.db).Insert("remote_executors").Columns("id", "name", "address", "token_ref", "use_tls", "llm_mode", "created_at", "updated_at").
		Values(executor.ID, executor.Name, executor.Address, executor.TokenRef, useTLS, executor.LLMMode, stamp(now), stamp(now)).
		Suffix("ON CONFLICT(id) DO UPDATE SET name=excluded.name, address=excluded.address, token_ref=excluded.token_ref, use_tls=excluded.use_tls, llm_mode=excluded.llm_mode, updated_at=excluded.updated_at").
		ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("save remote executor: %w", err)
	}
	return nil
}

// DeleteRemoteExecutor removes one registration.
func (s *Store) DeleteRemoteExecutor(ctx context.Context, id string) error {
	result, err := statements(s.db).Delete("remote_executors").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("delete remote executor: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("remote executor %q not found", id)
	}
	return nil
}

// CountPipelinesForExecutor reports how many pipelines target an executor;
// the Settings UI shows this before detaching on removal.
func (s *Store) CountPipelinesForExecutor(ctx context.Context, id string) (int, error) {
	var count int
	if err := statements(s.db).Select("COUNT(*)").From("pipelines").Where(squirrel.Eq{"executor_id": id}).QueryRowContext(ctx).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pipelines for executor: %w", err)
	}
	return count, nil
}

// DetachExecutorFromPipelines clears the executor target so its pipelines
// become ordinary local pipelines again.
func (s *Store) DetachExecutorFromPipelines(ctx context.Context, id string) error {
	if _, err := statements(s.db).Update("pipelines").Set("executor_id", "").Set("updated_at", stamp(time.Now().UTC())).Where(squirrel.Eq{"executor_id": id}).ExecContext(ctx); err != nil {
		return fmt.Errorf("detach pipelines from executor: %w", err)
	}
	return nil
}

// PipelineExecutorID resolves one pipeline's executor target. An empty
// result means the pipeline runs locally.
func (s *Store) PipelineExecutorID(ctx context.Context, pipelineID string) string {
	var executorID string
	if err := statements(s.db).Select("COALESCE(executor_id, '')").From("pipelines").Where(squirrel.Eq{"id": pipelineID}).QueryRowContext(ctx).Scan(&executorID); err != nil {
		return ""
	}
	return executorID
}

// GetChatRunByExecutionID finds the chat run linked to one execution so
// remotely-completed runs refresh their conversation status.
func (s *Store) GetChatRunByExecutionID(ctx context.Context, executionID string) (domain.ChatRun, bool) {
	var run domain.ChatRun
	err := statements(s.db).Select("id", "conversation_id", "execution_id", "status", "status_text", "error", "created_at", "updated_at").From("chat_runs").Where(squirrel.Eq{"execution_id": executionID}).QueryRowContext(ctx).
		Scan(&run.ID, &run.ConversationID, &run.ExecutionID, &run.Status, &run.StatusText, &run.Error, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		return domain.ChatRun{}, false
	}
	return run, true
}

// AdoptRemoteExecution inserts an execution that ran on a remote executor
// (typically an autonomous schedule fire) into local history. Runs already
// known are left untouched via the idempotent insert.
func (s *Store) AdoptRemoteExecution(ctx context.Context, execution domain.Execution) error {
	if execution.ID == "" || execution.PipelineID == "" {
		return nil
	}
	finished := sql.NullString{}
	if execution.FinishedAt != nil {
		finished = sql.NullString{String: stamp(*execution.FinishedAt), Valid: true}
	}
	runStarted := sql.NullString{}
	if execution.RunStartedAt != nil {
		runStarted = sql.NullString{String: stamp(*execution.RunStartedAt), Valid: true}
	}
	if _, err := statements(s.db).Insert("executions").Columns("id", "pipeline_id", "trigger_id", "status", "started_at", "finished_at", "run_started_at", "error", "executor_id").
		Values(execution.ID, execution.PipelineID, execution.TriggerID, execution.Status, stamp(execution.StartedAt), finished, runStarted, execution.Error, execution.ExecutorID).
		Suffix("ON CONFLICT(id) DO NOTHING").ExecContext(ctx); err != nil {
		return fmt.Errorf("adopt remote execution: %w", err)
	}
	for ordinal, nodeRun := range execution.NodeRuns {
		input, err := encode(nodeRun.Input)
		if err != nil {
			continue
		}
		output, err := encode(nodeRun.Output)
		if err != nil {
			continue
		}
		if _, err := statements(s.db).Insert("node_runs").Columns("execution_id", "ordinal", "node_id", "node_type", "status", "input_json", "output_json", "error", "started_at", "finished_at").
			Values(execution.ID, ordinal, nodeRun.NodeID, nodeRun.NodeType, nodeRun.Status, input, output, nodeRun.Error, stamp(nodeRun.StartedAt), stamp(nodeRun.FinishedAt)).
			Suffix("ON CONFLICT(execution_id, ordinal) DO NOTHING").ExecContext(ctx); err != nil {
			continue
		}
	}
	return nil
}
