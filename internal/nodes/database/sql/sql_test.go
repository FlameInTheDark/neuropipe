package sql

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

// resultExecutor returns a canned result for every statement.
type resultExecutor struct {
	result domain.SQLResult
}

func (e *resultExecutor) ExecuteSQL(_ context.Context, _ domain.SQLRequest) (domain.SQLResult, error) {
	return e.result, nil
}

// failingExecutor reports an error for every statement.
type failingExecutor struct{ err error }

func (e *failingExecutor) ExecuteSQL(_ context.Context, _ domain.SQLRequest) (domain.SQLResult, error) {
	return domain.SQLResult{}, e.err
}

func plainConfig(t *testing.T, config map[string]any) nodes.Invocation {
	t.Helper()
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "sql-1", Type: "action:sql", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterMetadata(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("action:sql")
	if !ok {
		t.Fatal("action:sql was not registered")
	}
	definition := module.Definition()
	if definition.Category != "Database" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != sqlInputPinID || !definition.Inputs[1].IgnoreConfigFallback {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	wantOutputs := []string{"out", "columns", "rows", "rowsAffected", "lastInsertId", "truncated"}
	if len(definition.Outputs) != len(wantOutputs) {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	for index, want := range wantOutputs {
		if definition.Outputs[index].ID != want {
			t.Fatalf("outputs = %#v, want %q at %d", definition.Outputs, want, index)
		}
	}
	if definition.DefaultConfig["sql"] != "" || definition.DefaultConfig["databaseId"] != "" {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}
}

func TestExecuteMapsResultOutputs(t *testing.T) {
	executor := &resultExecutor{result: domain.SQLResult{
		Columns: []string{"id", "name"}, Rows: []map[string]any{{"id": 1.0, "name": "Ada"}},
		RowsAffected: 5, Truncated: true,
	}}
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{}}
	result, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs["columns"], []string{"id", "name"}) {
		t.Fatalf("columns = %#v", result.Outputs["columns"])
	}
	if rows, ok := result.Outputs["rows"].([]any); !ok || len(rows) != 1 {
		t.Fatalf("rows = %#v", result.Outputs["rows"])
	}
	if result.Outputs["rowsAffected"] != int64(5) || result.Outputs["truncated"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	// A nil last-insert-id reports zero.
	if result.Outputs["lastInsertId"] != int64(0) {
		t.Fatalf("lastInsertId = %#v", result.Outputs["lastInsertId"])
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestExecuteTrimsDatabaseIDAndCapsRows(t *testing.T) {
	executor := &executorStub{}
	config := map[string]any{"databaseId": "  database-1  ", "sql": "SELECT 1", "parameters": []any{}}
	if _, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.DatabaseID != "database-1" {
		t.Fatalf("databaseID = %q", executor.request.DatabaseID)
	}
	if executor.request.MaxRows != maxNodeRows {
		t.Fatalf("maxRows = %d, want %d", executor.request.MaxRows, maxNodeRows)
	}
}

func TestExecuteRequiresRuntime(t *testing.T) {
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{}}
	if _, err := New().Execute(context.Background(), plainConfig(t, config), nil); err == nil || !strings.Contains(err.Error(), "database execution is unavailable") {
		t.Fatalf("nil runtime error = %v", err)
	}
	if _, err := New().Execute(context.Background(), plainConfig(t, config), struct{}{}); err == nil || !strings.Contains(err.Error(), "database execution is unavailable") {
		t.Fatalf("non-provider runtime error = %v", err)
	}
}

func TestExecutePropagatesExecutorError(t *testing.T) {
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT boom", "parameters": []any{}}
	_, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: &failingExecutor{err: errors.New("syntax error")}})
	if err == nil || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteRequiresConfiguredParameter(t *testing.T) {
	parameter := map[string]any{"id": "minimum", "name": "min_id", "label": "Minimum ID", "type": map[string]any{"kind": "int"}, "required": true}
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{parameter}}
	_, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: &executorStub{}})
	if err == nil || !strings.Contains(err.Error(), `SQL parameter "Minimum ID" is required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteForwardsOptionalMissingParametersAsNil(t *testing.T) {
	executor := &executorStub{}
	parameter := map[string]any{"id": "minimum", "name": "min_id", "type": map[string]any{"kind": "int"}, "required": false}
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{parameter}}
	if _, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(executor.request.Parameters) != 1 || executor.request.Parameters[0].Name != "min_id" || executor.request.Parameters[0].Value != nil {
		t.Fatalf("parameters = %#v", executor.request.Parameters)
	}
}

func TestExecuteUsesEditorStatementWhenPinUnconnected(t *testing.T) {
	executor := &executorStub{}
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 'editor' AS source", "parameters": []any{}}
	invocation := plainConfig(t, config)
	// An unconnected sql input pin must not override the editor value.
	invocation.Inputs[sqlInputPinID] = "SELECT 'wire' AS source"
	if _, err := New().Execute(context.Background(), invocation, runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.SQL != "SELECT 'editor' AS source" {
		t.Fatalf("SQL = %q", executor.request.SQL)
	}
}

func TestParameterContractValidation(t *testing.T) {
	tests := []struct {
		name      string
		parameter map[string]any
		want      string
	}{
		{
			name:      "invalid pin ID",
			parameter: map[string]any{"id": "9bad", "name": "value", "type": map[string]any{"kind": "string"}},
			want:      "SQL parameter 1 has an invalid pin ID",
		},
		{
			name:      "invalid name",
			parameter: map[string]any{"id": "value", "name": "not a name", "type": map[string]any{"kind": "string"}},
			want:      "SQL parameter 1 has an invalid name",
		},
		{
			name:      "reserved name",
			parameter: map[string]any{"id": "value", "name": "sql", "type": map[string]any{"kind": "string"}},
			want:      `SQL parameter 1 uses a reserved name "sql"`,
		},
		{
			name:      "invalid type",
			parameter: map[string]any{"id": "value", "name": "value", "type": map[string]any{"kind": "bogus"}},
			want:      `SQL parameter "value" type`,
		},
		{
			name:      "list without element type",
			parameter: map[string]any{"id": "value", "name": "value", "type": map[string]any{"kind": "list"}},
			want:      `SQL parameter "value" type`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{test.parameter}}}}
			if _, err := New().Resolve(node); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParametersMustBeList(t *testing.T) {
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": "min_id"}}}
	if _, err := New().Resolve(node); err == nil || !strings.Contains(err.Error(), "SQL parameters must be a list") {
		t.Fatalf("Resolve() error = %v", err)
	}
	config := map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": "min_id"}
	if _, err := New().Execute(context.Background(), plainConfig(t, config), runtimeStub{executor: &executorStub{}}); err == nil || !strings.Contains(err.Error(), "SQL parameters must be a list") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDuplicateParameterNamesRejected(t *testing.T) {
	first := map[string]any{"id": "minimum", "name": "min_id", "type": map[string]any{"kind": "int"}}
	second := map[string]any{"id": "maximum", "name": "min_id", "type": map[string]any{"kind": "int"}}
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{first, second}}}}
	if _, err := New().Resolve(node); err == nil || !strings.Contains(err.Error(), `SQL parameters contain duplicate name "min_id"`) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestParameterNamePrefixAndLabelDefaults(t *testing.T) {
	parameter := map[string]any{"id": "minimum", "name": ":min_id", "type": map[string]any{"kind": "float"}}
	node := domain.FlowNode{Type: "action:sql", Data: map[string]any{"config": map[string]any{"databaseId": "database-1", "sql": "SELECT 1", "parameters": []any{parameter}}}}
	definition, err := New().Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	pin := definition.Inputs[len(definition.Inputs)-1]
	if pin.ID != "minimum" || pin.Label != "min_id" || pin.Type == nil || pin.Type.Kind != domain.TypeFloat {
		t.Fatalf("pin = %#v", pin)
	}
}

func TestResolveWithoutConfig(t *testing.T) {
	node := domain.FlowNode{Type: "action:sql"}
	definition, err := New().Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != sqlInputPinID {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
}
