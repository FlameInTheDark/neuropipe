package hash

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

// failingExecutor reports an error for every command.
type failingExecutor struct{ err error }

func (e *failingExecutor) ExecuteCommand(_ context.Context, _ domain.KVCommandRequest) (domain.KVCommandResult, error) {
	return domain.KVCommandResult{}, e.err
}

func TestRegisterNodes(t *testing.T) {
	registry := nodes.New()
	if err := RegisterGet(registry); err != nil {
		t.Fatalf("RegisterGet() error = %v", err)
	}
	if err := RegisterSet(registry); err != nil {
		t.Fatalf("RegisterSet() error = %v", err)
	}
	for _, nodeType := range []string{"action:kv_hash_get", "action:kv_hash_set"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		if definition := module.Definition(); definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
	}
}

func TestGetReadsWholeHash(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: map[string]any{"email": "ada@example.com", "name": "Ada"}}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "user:42"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "HGETALL" || !reflect.DeepEqual(executor.request.Args, []string{"user:42"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	fields, ok := result.Outputs["fields"].(map[string]any)
	if !ok || len(fields) != 2 || fields["email"] != "ada@example.com" {
		t.Fatalf("fields = %#v", result.Outputs["fields"])
	}
	if result.Outputs["found"] != true || result.Outputs["value"] != "" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestGetSingleFieldHashExposesValue(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: map[string]any{"email": "ada@example.com"}}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "user:42"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["value"] != "ada@example.com" || result.Outputs["found"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestGetEmptyHash(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: map[string]any{}}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "user:42"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["found"] != false || result.Outputs["value"] != "" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if fields := result.Outputs["fields"].(map[string]any); len(fields) != 0 {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestGetNilFieldReply(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{IsNil: true}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "user:42", "field": "email"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["found"] != false || result.Outputs["value"] != "" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestGetRequiresKey(t *testing.T) {
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"field": "email"},
	), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted an empty key")
	}
}

func TestHashGetPropagatesErrors(t *testing.T) {
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "user:42"}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), nil); err == nil || !strings.Contains(err.Error(), "key/value database execution is unavailable") {
		t.Fatalf("nil runtime error = %v", err)
	}
	_, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: &failingExecutor{err: errors.New("hash boom")}})
	if err == nil || !strings.Contains(err.Error(), "hash boom") {
		t.Fatalf("executor error = %v", err)
	}
}

func TestSetSortsFieldArguments(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{"key": "user:42", "fields": map[string]any{"name": "Ada", "email": "ada@example.com"}}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"user:42", "email", "ada@example.com", "name", "Ada"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
}

func TestSetAddedFromFloatReply(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: float64(2)}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{"key": "user:42", "fields": map[string]any{"email": "ada@example.com"}}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["added"] != int64(2) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSetRequiresKeyAndDatabase(t *testing.T) {
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{"fields": map[string]any{"email": "ada@example.com"}}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted an empty key")
	}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{}, map[string]any{"key": "user:42", "fields": map[string]any{"email": "ada@example.com"}},
	), runtimeStub{executor: &executorStub{}}); err == nil || !strings.Contains(err.Error(), "select a KV database first") {
		t.Fatalf("missing database error = %v", err)
	}
}

func TestSetNonObjectFieldsRejected(t *testing.T) {
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	inputs := map[string]any{"key": "user:42", "fields": "email=ada@example.com"}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, inputs,
	), runtimeStub{executor: &executorStub{}}); err == nil || !strings.Contains(err.Error(), `pin "fields" requires an object value`) {
		t.Fatalf("error = %v", err)
	}
}
