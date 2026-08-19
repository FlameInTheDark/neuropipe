package sql

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type executorStub struct {
	request domain.SQLRequest
}

func (e *executorStub) ExecuteSQL(_ context.Context, request domain.SQLRequest) (domain.SQLResult, error) {
	e.request = request
	id := int64(9)
	return domain.SQLResult{Columns: []string{"name"}, Rows: []map[string]any{{"name": "Ada"}}, RowsAffected: 1, LastInsertID: &id}, nil
}

type runtimeStub struct{ executor nodes.SQLExecutor }

func (r runtimeStub) SQLExecutor() nodes.SQLExecutor { return r.executor }

func TestDynamicParametersAndExecution(t *testing.T) {
	parameter := map[string]any{"id": "minimum", "name": "min_id", "label": "Minimum ID", "type": map[string]any{"kind": "int"}, "required": true}
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT name FROM people WHERE id >= :min_id", "parameters": []any{parameter}}}}
	module := New()
	definition, err := module.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(definition.Inputs) != 3 || definition.Inputs[1].ID != "sql" || definition.Inputs[2].ID != "minimum" || definition.Inputs[2].Type == nil || definition.Inputs[2].Type.Kind != domain.TypeInt {
		t.Fatalf("Resolve() inputs = %#v", definition.Inputs)
	}
	executor := &executorStub{}
	result, err := module.Execute(context.Background(), nodes.Invocation{Node: node, Definition: definition, Config: node.Data["config"].(map[string]any), Inputs: map[string]any{"minimum": int64(2)}}, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.DatabaseID != "database-1" || len(executor.request.Parameters) != 1 || executor.request.Parameters[0].Name != "min_id" || executor.request.Parameters[0].Value != int64(2) {
		t.Fatalf("ExecuteSQL() request = %#v", executor.request)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" || result.Outputs["lastInsertId"] != int64(9) {
		t.Fatalf("Execute() result = %#v", result)
	}
}

// When the SQL input pin is connected, the wired statement must override the
// editor-configured SQL so pipelines can supply statements dynamically.
func TestSQLInputPinOverridesEditor(t *testing.T) {
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 'editor' AS source", "parameters": []any{}}}}
	module := New()
	definition, err := module.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	executor := &executorStub{}
	_, err = module.Execute(context.Background(), nodes.Invocation{
		Node:            node,
		Definition:      definition,
		Config:          node.Data["config"].(map[string]any),
		Inputs:          map[string]any{sqlInputPinID: "SELECT 'wire' AS source"},
		ConnectedInputs: map[string]bool{sqlInputPinID: true},
	}, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.SQL != "SELECT 'wire' AS source" {
		t.Fatalf("ExecuteSQL() SQL = %q, want wired statement", executor.request.SQL)
	}
}

// A reserved SQL pin ID cannot be reused for a user-configured parameter.
func TestSQLParameterRejectsReservedID(t *testing.T) {
	parameter := map[string]any{"id": "sql", "name": "value", "label": "Reserved", "type": map[string]any{"kind": "string"}, "required": false}
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{parameter}}}}
	module := New()
	_, err := module.Resolve(node)
	if err == nil {
		t.Fatal("Resolve() expected reserved parameter error")
	}
}
