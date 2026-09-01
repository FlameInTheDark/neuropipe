package list

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
	err     error
}

func (e *executorStub) ExecuteCommand(_ context.Context, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	e.request = request
	return e.result, e.err
}

type runtimeStub struct{ executor nodes.KVExecutor }

func (r runtimeStub) KVExecutor() nodes.KVExecutor { return r.executor }

func invocation(config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "list-node", Type: "action:kv_list_push", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegister(t *testing.T) {
	registry := nodes.New()
	for _, register := range []func(nodes.Registrar) error{RegisterPush, RegisterPop, RegisterRange} {
		if err := register(registry); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	for _, nodeType := range []string{"action:kv_list_push", "action:kv_list_pop", "action:kv_list_range"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		definition := module.Definition()
		if definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
		if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
			t.Fatalf("%s capabilities = %#v", nodeType, definition.Capabilities)
		}
	}
}

func TestPushDefaultsToTail(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(3)}}
	module := nodes.Implementation{Metadata: pushDefinition(), Executor: executePush}
	config := map[string]any{"databaseId": "db-1", "side": "right"}
	inputs := map[string]any{"key": "queue:jobs", "values": []any{"a", "b"}}
	result, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "RPUSH" || executor.request.DatabaseID != "db-1" {
		t.Fatalf("request = %#v", executor.request)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"queue:jobs", "a", "b"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
	if result.Outputs["length"] != int64(3) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPushLeftSideConfig(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: pushDefinition(), Executor: executePush}
	config := map[string]any{"databaseId": "db-1", "side": "left"}
	inputs := map[string]any{"key": "queue:jobs", "values": []any{"a"}}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "LPUSH" {
		t.Fatalf("command = %s", executor.request.Command)
	}
}

func TestPushConvertsScalarValues(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: pushDefinition(), Executor: executePush}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "queue:jobs", "values": 7}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"queue:jobs", "7"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
}

func TestPushLengthCoercions(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int64
	}{
		{"int64", int64(3), 3},
		{"float64", float64(2), 2},
		{"numeric text", "7", 7},
		{"other", nil, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{result: domain.KVCommandResult{Value: test.value}}
			module := nodes.Implementation{Metadata: pushDefinition(), Executor: executePush}
			result, err := module.Execute(context.Background(), invocation(
				map[string]any{"databaseId": "db-1"},
				map[string]any{"key": "k", "values": []any{"a"}},
			), runtimeStub{executor: executor})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.Outputs["length"] != test.want {
				t.Fatalf("length = %#v, want %d", result.Outputs["length"], test.want)
			}
		})
	}
}

func TestPushErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		inputs  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{"values": []any{"a"}}, runtimeStub{executor: &executorStub{}}, "key is required"},
		{"empty values", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "values": []any{}}, runtimeStub{executor: &executorStub{}}, "at least one value is required"},
		{"missing values", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"}, runtimeStub{executor: &executorStub{}}, "at least one value is required"},
		{"blank scalar values", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "values": "   "}, runtimeStub{executor: &executorStub{}}, "at least one value is required"},
		{"unconvertible value", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "values": []any{make(chan int)}}, runtimeStub{executor: &executorStub{}}, "item 1"},
		{"missing database", map[string]any{}, map[string]any{"key": "k", "values": []any{"a"}}, runtimeStub{executor: &executorStub{}}, "select a KV database first"},
		{"nil runtime", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "values": []any{"a"}}, nil, "key/value database execution is unavailable"},
		{"executor error", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "values": []any{"a"}}, runtimeStub{executor: &executorStub{err: errors.New("flush failed")}}, "flush failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: pushDefinition(), Executor: executePush}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPopSingleStringReply(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "job-1"}}
	module := nodes.Implementation{Metadata: popDefinition(), Executor: executePop}
	config := map[string]any{"databaseId": "db-1", "side": "left"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "queue:jobs"}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "LPOP" || !reflect.DeepEqual(executor.request.Args, []string{"queue:jobs"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["value"] != "job-1" || result.Outputs["found"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if values, ok := result.Outputs["values"].([]any); !ok || len(values) != 1 || values[0] != "job-1" {
		t.Fatalf("values = %#v", result.Outputs["values"])
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestPopRightSideAndCountArgument(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"a", "b"}}}
	module := nodes.Implementation{Metadata: popDefinition(), Executor: executePop}
	config := map[string]any{"databaseId": "db-1", "side": "right"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{"key": "k", "count": float64(2)}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "RPOP" || !reflect.DeepEqual(executor.request.Args, []string{"k", "2"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["value"] != "a" || result.Outputs["found"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if values, ok := result.Outputs["values"].([]any); !ok || len(values) != 2 {
		t.Fatalf("values = %#v", result.Outputs["values"])
	}
}

func TestPopCountOmittedForSingleAndNonPositive(t *testing.T) {
	tests := []struct {
		name  string
		count any
		args  []string
	}{
		{"default one", nil, []string{"k"}},
		{"explicit one", float64(1), []string{"k"}},
		{"zero", float64(0), []string{"k"}},
		{"negative", float64(-3), []string{"k"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{}
			module := nodes.Implementation{Metadata: popDefinition(), Executor: executePop}
			inputs := map[string]any{"key": "k"}
			if test.count != nil {
				inputs["count"] = test.count
			}
			if _, err := module.Execute(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, inputs), runtimeStub{executor: executor}); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(executor.request.Args, test.args) {
				t.Fatalf("args = %#v, want %#v", executor.request.Args, test.args)
			}
		})
	}
}

func TestPopEmptyAndNilReplies(t *testing.T) {
	tests := []struct {
		name   string
		result domain.KVCommandResult
	}{
		{"nil reply", domain.KVCommandResult{IsNil: true}},
		{"empty list", domain.KVCommandResult{Value: []any{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{result: test.result}
			module := nodes.Implementation{Metadata: popDefinition(), Executor: executePop}
			result, err := module.Execute(context.Background(), invocation(
				map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
			), runtimeStub{executor: executor})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.Outputs["found"] != false || result.Outputs["value"] != "" {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
			if values, ok := result.Outputs["values"].([]any); !ok || len(values) != 0 {
				t.Fatalf("values = %#v", result.Outputs["values"])
			}
		})
	}
}

func TestPopErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{}, "key is required"},
		{"invalid count", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "count": "abc"}, `pin "count" requires a number`},
		{"missing database", map[string]any{}, map[string]any{"key": "k"}, "select a KV database first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: popDefinition(), Executor: executePop}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), runtimeStub{executor: &executorStub{}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRangeDefaultsToWholeList(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"a", "b", "c"}}}
	module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "LRANGE" || !reflect.DeepEqual(executor.request.Args, []string{"k", "0", "-1"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	if !reflect.DeepEqual(result.Outputs["items"], []any{"a", "b", "c"}) || result.Outputs["count"] != int64(3) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestRangeCustomBounds(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"b"}}}
	module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
	inputs := map[string]any{"key": "k", "start": float64(1), "stop": float64(1)}
	if _, err := module.Execute(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"k", "1", "1"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
}

func TestRangeNonListReplyYieldsEmptyItems(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "oops"}}
	module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs["items"], []any{}) || result.Outputs["count"] != int64(0) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestRangeErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{}, "key is required"},
		{"invalid start", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "start": true}, `pin "start" requires a number`},
		{"invalid stop", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "stop": "far"}, `pin "stop" requires a number`},
		{"missing database", map[string]any{}, map[string]any{"key": "k"}, "select a KV database first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), runtimeStub{executor: &executorStub{}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
