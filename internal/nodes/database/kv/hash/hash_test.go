package hash

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type executorStub struct {
	request domain.KVCommandRequest
	result  domain.KVCommandResult
}

func (e *executorStub) ExecuteCommand(_ context.Context, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	e.request = request
	return e.result, nil
}

type runtimeStub struct{ executor nodes.KVExecutor }

func (r runtimeStub) KVExecutor() nodes.KVExecutor { return r.executor }

func invocation(config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:kv_hash_set", Data: map[string]any{"config": config}},
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

// TestSetDefinitionContract pins the fields editor contract: an object pin so
// the inspector-configured map survives engine config-fallback validation,
// plus the visual kv-hash-fields editor kind.
func TestSetDefinitionContract(t *testing.T) {
	definition := setDefinition()
	fieldsPin := definition.Inputs[2]
	if fieldsPin.ID != "fields" || fieldsPin.DataType != domain.DataObject {
		t.Fatalf("fields pin = %#v", fieldsPin)
	}
	if fieldsPin.Type == nil || fieldsPin.Type.Kind != domain.TypeMap {
		t.Fatalf("fields pin type = %#v", fieldsPin.Type)
	}
	kinds := map[string]string{}
	for _, field := range definition.Fields {
		kinds[field.Name] = field.Kind
	}
	if kinds["fields"] != "kv-hash-fields" {
		t.Fatalf("fields config kind = %#v", definition.Fields)
	}
}

func TestSetWritesFieldsFromObject(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	// The engine copies the inspector map into the unwired object pin.
	inputs := map[string]any{
		"key":    "user:42",
		"fields": map[string]any{"email": "ada@example.com", "name": "Ada"},
	}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "HSET" {
		t.Fatalf("command = %s", executor.request.Command)
	}
	args := executor.request.Args
	if len(args) != 5 || args[0] != "user:42" || args[1] != "email" || args[2] != "ada@example.com" {
		t.Fatalf("args = %#v", args)
	}
}

func TestSetRemoveModeDeletesFields(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{
		"key":    "user:42",
		"fields": map[string]any{"email": "ada@example.com"},
	}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1", "mode": "remove"}, inputs,
	), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "HDEL" || executor.request.Args[1] != "email" {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestSetRequiresAtLeastOneField(t *testing.T) {
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{"key": "user:42", "fields": map[string]any{}}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted an empty field map")
	}
}

func TestGetReadsSingleField(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "ada@example.com"}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"},
		map[string]any{"key": "user:42", "field": "email"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "HGET" || executor.request.Args[1] != "email" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["value"] != "ada@example.com" || result.Outputs["found"] != true {
		t.Fatalf("result = %#v", result.Outputs)
	}
}
