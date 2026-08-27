package keys

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
		Node:            domain.FlowNode{Type: "action:kv_get", Data: map[string]any{"config": config}},
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestGetOutputsValueAndFound(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "ada"}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1", "key": "user:1"},
		map[string]any{"key": "user:1"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "GET" || executor.request.Args[0] != "user:1" || executor.request.DatabaseID != "db-1" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["value"] != "ada" || result.Outputs["found"] != true || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGetReportsMissingKey(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{IsNil: true}}
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1", "key": "missing"},
		map[string]any{"key": "missing"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["found"] != false || result.Outputs["value"] != "" {
		t.Fatalf("missing key result = %#v", result)
	}
}

func TestGetRequiresKey(t *testing.T) {
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{},
	), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted an empty key")
	}
}

func TestGetRequiresDatabase(t *testing.T) {
	module := nodes.Implementation{Metadata: getDefinition(), Executor: executeGet}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"key": "user:1"}, map[string]any{"key": "user:1"},
	), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted a missing database")
	}
}

func TestSetBuildsConditionalArguments(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "OK"}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	config := map[string]any{"databaseId": "db-1", "condition": "if-not-exists", "returnPrevious": true}
	inputs := map[string]any{"key": "k", "value": "v", "ttlSeconds": float64(30)}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	expected := []string{"k", "v", "EX", "30", "NX", "GET"}
	if executor.request.Command != "SET" || len(executor.request.Args) != len(expected) {
		t.Fatalf("request = %#v", executor.request)
	}
	for index, want := range expected {
		if executor.request.Args[index] != want {
			t.Fatalf("args = %#v, want %v at %d", executor.request.Args, want, index)
		}
	}
}

func TestSetNxFailureReportedThroughOk(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{IsNil: true}}
	module := nodes.Implementation{Metadata: setDefinition(), Executor: executeSet}
	config := map[string]any{"databaseId": "db-1", "condition": "if-not-exists"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "k", "value": "v"}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["ok"] != false {
		t.Fatalf("ok output = %#v", result.Outputs["ok"])
	}
}

func TestDeleteUsesWiredKeyList(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: deleteDefinition(), Executor: executeDelete}
	config := map[string]any{"databaseId": "db-1", "keys": "ignored\nwhen wired"}
	inputs := map[string]any{"keys": []any{"a", "b"}}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: config, Inputs: inputs, ConnectedInputs: map[string]bool{"keys": true},
	}, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "DEL" || executor.request.Args[0] != "a" || executor.request.Args[1] != "b" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["deleted"] != int64(2) {
		t.Fatalf("deleted output = %#v", result.Outputs["deleted"])
	}
}

func TestDeleteFallsBackToTextareaConfig(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: deleteDefinition(), Executor: executeDelete}
	config := map[string]any{"databaseId": "db-1", "keys": "a\n\n b \n"}
	if _, err := module.Execute(context.Background(), invocation(config, map[string]any{}), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(executor.request.Args) != 2 || executor.request.Args[0] != "a" || executor.request.Args[1] != "b" {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestDeleteFallsBackToListEditorConfig(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: deleteDefinition(), Executor: executeDelete}
	config := map[string]any{"databaseId": "db-1", "keys": []any{"user:42", " ", 7}}
	if _, err := module.Execute(context.Background(), invocation(config, map[string]any{}), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(executor.request.Args) != 2 || executor.request.Args[0] != "user:42" || executor.request.Args[1] != "7" {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestSetDefinitionExposesInspectorFields(t *testing.T) {
	definition := setDefinition()
	kinds := map[string]string{}
	for _, field := range definition.Fields {
		kinds[field.Name] = field.Kind
	}
	if kinds["key"] != "string" || kinds["value"] != "textarea" || kinds["ttlSeconds"] != "number" {
		t.Fatalf("kv_set fields = %#v", definition.Fields)
	}
}

func TestIncrementDeltaFromConfig(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(8)}}
	module := nodes.Implementation{Metadata: incrementDefinition(), Executor: executeIncrement}
	// The engine copies an inspector-configured delta into the unwired pin.
	config := map[string]any{"databaseId": "db-1", "key": "counter", "delta": float64(3)}
	if _, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "counter", "delta": float64(3)}), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Args[1] != "3" {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestIncrementUsesIntModeByDefault(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(6)}}
	module := nodes.Implementation{Metadata: incrementDefinition(), Executor: executeIncrement}
	config := map[string]any{"databaseId": "db-1", "mode": "integer"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "counter", "delta": float64(5)}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "INCRBY" || executor.request.Args[1] != "5" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["value"] != int64(6) {
		t.Fatalf("value output = %#v", result.Outputs["value"])
	}
}

func TestExpireRejectsNonPositiveTTL(t *testing.T) {
	module := nodes.Implementation{Metadata: expireDefinition(), Executor: executeExpire}
	config := map[string]any{"databaseId": "db-1", "mode": "expire"}
	if _, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "k"}), runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted a zero TTL")
	}
}

func TestExpirePersistMode(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: expireDefinition(), Executor: executeExpire}
	config := map[string]any{"databaseId": "db-1", "mode": "persist"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "k"}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "PERSIST" || result.Outputs["ok"] != true {
		t.Fatalf("request = %#v, result = %#v", executor.request, result)
	}
}

func TestTTLOutputsSeconds(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(42)}}
	module := nodes.Implementation{Metadata: ttlDefinition(), Executor: executeTTL}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["ttl"] != int64(42) {
		t.Fatalf("ttl output = %#v", result.Outputs["ttl"])
	}
}

func TestRenameUsesRenamENXWhenConfigured(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: renameDefinition(), Executor: executeRename}
	config := map[string]any{"databaseId": "db-1", "onlyIfMissing": true}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "a", "newKey": "b"}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "RENAMENX" || result.Outputs["ok"] != true {
		t.Fatalf("request = %#v result = %#v", executor.request, result)
	}
}

func TestDefinitionShape(t *testing.T) {
	definition := getDefinition()
	if definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
		t.Fatalf("capabilities = %#v", definition.Capabilities)
	}
	if definition.Fields[0].Kind != "kv-database-select" || !definition.Fields[0].Required {
		t.Fatalf("fields = %#v", definition.Fields)
	}
}
