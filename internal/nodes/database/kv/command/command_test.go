package command

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

// failingExecutor reports an error for every command.
type failingExecutor struct{ err error }

func (e *failingExecutor) ExecuteCommand(_ context.Context, _ domain.KVCommandRequest) (domain.KVCommandResult, error) {
	return domain.KVCommandResult{}, e.err
}

func TestRegisterMetadata(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("action:kv_command")
	if !ok {
		t.Fatal("action:kv_command was not registered")
	}
	definition := module.Definition()
	if definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
		t.Fatalf("definition = %#v", definition)
	}
	// The command pin is config-fallback-exempt: the editor value wins unless a
	// wire supplies the pin.
	for _, input := range definition.Inputs {
		if input.ID == commandInputPinID && !input.IgnoreConfigFallback {
			t.Fatalf("command pin = %#v", input)
		}
	}
	if definition.DefaultConfig["command"] != "" || definition.DefaultConfig["allowDangerous"] != false {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}
}

func TestExecuteRequiresDatabase(t *testing.T) {
	config := map[string]any{"command": "GET", "arguments": []any{}}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: &executorStub{}}); err == nil {
		t.Fatal("Execute() accepted a missing database")
	}
}

func TestExecuteWithoutRuntime(t *testing.T) {
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": []any{}}
	invocation := nodes.Invocation{Config: config, Inputs: map[string]any{}}
	if _, err := New().Execute(context.Background(), invocation, nil); err == nil {
		t.Fatal("Execute() accepted a nil runtime")
	}
	if _, err := New().Execute(context.Background(), invocation, struct{}{}); err == nil {
		t.Fatal("Execute() accepted a non-provider runtime")
	}
}

func TestExecutePropagatesExecutorError(t *testing.T) {
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": []any{}}
	_, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: &failingExecutor{err: errors.New("reset by peer")}})
	if err == nil || !strings.Contains(err.Error(), "reset by peer") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteRequiresConfiguredArgument(t *testing.T) {
	argument := map[string]any{"id": "key", "name": "key", "label": "Key", "type": map[string]any{"kind": "string"}, "required": true}
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": []any{argument}}
	_, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: &executorStub{}})
	if err == nil || !strings.Contains(err.Error(), `command argument "Key" is required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteOmitsUnwiredOptionalArguments(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	argument := map[string]any{"id": "key", "name": "key", "label": "Key", "type": map[string]any{"kind": "string"}, "required": false}
	config := map[string]any{"databaseId": "db-1", "command": "EXISTS", "arguments": []any{argument}}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
}

func TestExecuteEncodesNonStringRepliesAsValueText(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"a", 1.0}}}
	config := map[string]any{"databaseId": "db-1", "command": "LRANGE", "arguments": []any{}}
	result, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["valueText"] != `["a",1]` {
		t.Fatalf("valueText = %#v", result.Outputs["valueText"])
	}
	if values, ok := result.Outputs["value"].([]any); !ok || len(values) != 2 {
		t.Fatalf("value = %#v", result.Outputs["value"])
	}
}

func TestArgumentsMustBeList(t *testing.T) {
	node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"command": "GET", "arguments": "key"}}}
	if _, err := New().Resolve(node); err == nil || !strings.Contains(err.Error(), "command arguments must be a list") {
		t.Fatalf("Resolve() error = %v", err)
	}
	config := map[string]any{"databaseId": "db-1", "command": "GET", "arguments": "key"}
	if _, err := New().Execute(context.Background(), nodes.Invocation{Config: config, Inputs: map[string]any{}}, runtimeStub{executor: &executorStub{}}); err == nil || !strings.Contains(err.Error(), "command arguments must be a list") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestArgumentContractValidation(t *testing.T) {
	tests := []struct {
		name     string
		argument map[string]any
		want     string
	}{
		{
			name:     "invalid pin ID",
			argument: map[string]any{"id": "9bad", "name": "value", "type": map[string]any{"kind": "string"}},
			want:     "command argument 1 has an invalid pin ID",
		},
		{
			name:     "invalid name",
			argument: map[string]any{"id": "value", "name": "not a name", "type": map[string]any{"kind": "string"}},
			want:     "command argument 1 has an invalid name",
		},
		{
			name:     "reserved name",
			argument: map[string]any{"id": "value", "name": "command", "type": map[string]any{"kind": "string"}},
			want:     `command argument 1 uses a reserved name "command"`,
		},
		{
			name:     "invalid type",
			argument: map[string]any{"id": "value", "name": "value", "type": map[string]any{"kind": "bogus"}},
			want:     `command argument "value" type`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"command": "GET", "arguments": []any{test.argument}}}}
			_, err := New().Resolve(node)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestArgumentDuplicatesRejected(t *testing.T) {
	first := map[string]any{"id": "key", "name": "key", "type": map[string]any{"kind": "string"}}
	second := map[string]any{"id": "key", "name": "other", "type": map[string]any{"kind": "string"}}
	node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"command": "GET", "arguments": []any{first, second}}}}
	if _, err := New().Resolve(node); err == nil || !strings.Contains(err.Error(), `command arguments contain duplicate pin ID "key"`) {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestArgumentNamePrefixAndLabelDefaults(t *testing.T) {
	// A SQL-style ":name" or "@name" prefix is trimmed, and a missing label
	// falls back to the parameter name.
	argument := map[string]any{"id": "minimum", "name": ":min_id", "type": map[string]any{"kind": "int"}}
	node := domain.FlowNode{Type: "action:kv_command", Data: map[string]any{"config": map[string]any{"command": "GET", "arguments": []any{argument}}}}
	definition, err := New().Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	pin := definition.Inputs[len(definition.Inputs)-1]
	if pin.ID != "minimum" || pin.Label != "min_id" || pin.Type == nil || pin.Type.Kind != domain.TypeInt {
		t.Fatalf("pin = %#v", pin)
	}
}

func TestResolveWithoutConfig(t *testing.T) {
	node := domain.FlowNode{Type: "action:kv_command"}
	definition, err := New().Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != commandInputPinID {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
}
