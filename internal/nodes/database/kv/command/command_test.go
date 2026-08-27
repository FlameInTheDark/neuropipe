package command

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

func TestDynamicArgumentsBecomeTypedPins(t *testing.T) {
	argument := map[string]any{"id": "member", "name": "member", "label": "Member", "type": map[string]any{"kind": "string"}, "required": true}
	node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"databaseId": "db-1", "command": "ZADD", "arguments": []any{argument}}}}
	module := New()
	definition, err := module.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(definition.Inputs) != 3 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "command" || definition.Inputs[2].ID != "member" || definition.Inputs[2].Type == nil || definition.Inputs[2].Type.Kind != domain.TypeString {
		t.Fatalf("Resolve() inputs = %#v", definition.Inputs)
	}
}

func TestReservedPinIDRejected(t *testing.T) {
	argument := map[string]any{"id": "command", "name": "value", "label": "Reserved", "type": map[string]any{"kind": "string"}}
	node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"command": "GET", "arguments": []any{argument}}}}
	if _, err := New().Resolve(node); err == nil {
		t.Fatal("Resolve() accepted a reserved pin ID")
	}
}

func TestExecuteUsesWiredCommandAndArguments(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "ok"}}
	argument := map[string]any{"id": "key", "name": "key", "label": "Key", "type": map[string]any{"kind": "string"}, "required": true}
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": []any{argument}, "allowDangerous": false}
	invocation := nodes.Invocation{
		Config:          config,
		Inputs:          map[string]any{commandInputPinID: "SET", "key": "k"},
		ConnectedInputs: map[string]bool{commandInputPinID: true},
	}
	result, err := New().Execute(context.Background(), invocation, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "SET" || len(executor.request.Args) != 1 || executor.request.Args[0] != "k" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["valueText"] != "ok" || result.Outputs["isNil"] != false {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecutePassesAllowDangerous(t *testing.T) {
	executor := &executorStub{}
	config := map[string]any{"databaseId": "db-1", "command": "CONFIG", "arguments": []any{}, "allowDangerous": true}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !executor.request.AllowDangerous {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestExecuteRequiresCommand(t *testing.T) {
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: map[string]any{"databaseId": "db-1"}, Inputs: map[string]any{}}, runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted an empty command")
	}
}

func TestNilReplySurfacesAsIsNil(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{IsNil: true}}
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": []any{}}
	result, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["isNil"] != true || result.Outputs["value"] != nil || result.Outputs["valueText"] != "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestListArgumentEncodesAsJSON(t *testing.T) {
	argument := map[string]any{"id": "members", "name": "members", "label": "Members", "type": map[string]any{"kind": "list", "element": map[string]any{"kind": "any"}}}
	config := map[string]any{"databaseId": "db-1", "command": "SADD", "arguments": []any{argument}}
	invocation := nodes.Invocation{Config: config, Inputs: map[string]any{"members": []any{"a", "b"}}}
	if _, err := New().Execute(context.Background(), invocation, runtimeStub{executor: &executorStub{}}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}
