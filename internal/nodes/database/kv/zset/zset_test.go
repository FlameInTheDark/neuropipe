package zset

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
		Node:            domain.FlowNode{ID: "zset-node", Type: "action:kv_zset_add", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegister(t *testing.T) {
	registry := nodes.New()
	for _, register := range []func(nodes.Registrar) error{RegisterAdd, RegisterRange, RegisterRemove} {
		if err := register(registry); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	for _, nodeType := range []string{"action:kv_zset_add", "action:kv_zset_range", "action:kv_zset_remove"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		if definition := module.Definition(); definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
	}
}

func TestAddBuildsScoreMemberArguments(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "leaderboard:weekly", "entries": []any{
		map[string]any{"member": "ada", "score": 1.5},
		map[string]any{"member": "grace", "score": 2.0},
	}}
	result, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "ZADD" || executor.request.DatabaseID != "db-1" {
		t.Fatalf("request = %#v", executor.request)
	}
	// Whole-number scores format without a decimal point.
	if !reflect.DeepEqual(executor.request.Args, []string{"leaderboard:weekly", "1.5", "ada", "2", "grace"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
	if result.Outputs["added"] != int64(2) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAddConvertsScoreForms(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "k", "entries": []any{
		map[string]any{"member": int64(7), "score": "4.5"},
		map[string]any{"member": "lin", "score": 3},
	}}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"k", "4.5", "7", "3", "lin"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
}

func TestAddErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		inputs  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{"entries": []any{map[string]any{"member": "a", "score": 1.0}}}, runtimeStub{executor: &executorStub{}}, "key is required"},
		{"empty entries", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": []any{}}, runtimeStub{executor: &executorStub{}}, "at least one member/score entry is required"},
		{"missing entries", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"}, runtimeStub{executor: &executorStub{}}, "at least one member/score entry is required"},
		{"non-list entries", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": "ada"}, runtimeStub{executor: &executorStub{}}, `pin "entries" requires a list of member/score entries`},
		{"entry without score", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": []any{map[string]any{"member": "a"}}}, runtimeStub{executor: &executorStub{}}, "entry 1 score is required"},
		{"entry without object", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": []any{"a"}}, runtimeStub{executor: &executorStub{}}, "entry 1 requires an object with member and score"},
		{"missing database", map[string]any{}, map[string]any{"key": "k", "entries": []any{map[string]any{"member": "a", "score": 1.0}}}, runtimeStub{executor: &executorStub{}}, "select a KV database first"},
		{"nil runtime", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": []any{map[string]any{"member": "a", "score": 1.0}}}, nil, "key/value database execution is unavailable"},
		{"executor error", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "entries": []any{map[string]any{"member": "a", "score": 1.0}}}, runtimeStub{executor: &executorStub{err: errors.New("zadd boom")}}, "zadd boom"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRangeReshapesFlatReply(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"ada", "1.5", "grace", "2"}}}
	module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1", "order": "asc"}, map[string]any{"key": "leaderboard"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "ZRANGE" || !reflect.DeepEqual(executor.request.Args, []string{"leaderboard", "0", "-1", "WITHSCORES"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	want := []any{
		map[string]any{"member": "ada", "score": 1.5},
		map[string]any{"member": "grace", "score": 2.0},
	}
	if !reflect.DeepEqual(result.Outputs["entries"], want) || result.Outputs["count"] != int64(2) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestRangeDescendingAndCustomBounds(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{}}}
	module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
	config := map[string]any{"databaseId": "db-1", "order": "desc"}
	inputs := map[string]any{"key": "k", "start": float64(1), "stop": float64(5)}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "ZREVRANGE" || !reflect.DeepEqual(executor.request.Args, []string{"k", "1", "5", "WITHSCORES"}) {
		t.Fatalf("request = %#v", executor.request)
	}
}

func TestRangeReplyScoreCoercions(t *testing.T) {
	tests := []struct {
		name  string
		reply []any
		want  []any
	}{
		{"numeric scores", []any{"a", 1.5, "b", int64(2)}, []any{map[string]any{"member": "a", "score": 1.5}, map[string]any{"member": "b", "score": 2.0}}},
		{"non-numeric score becomes zero", []any{"a", "high"}, []any{map[string]any{"member": "a", "score": 0.0}}},
		{"odd trailing element is dropped", []any{"a", "1", "b"}, []any{map[string]any{"member": "a", "score": 1.0}}},
		{"empty reply", []any{}, []any{}},
		{"non-list reply", nil, []any{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{result: domain.KVCommandResult{Value: test.reply}}
			module := nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange}
			result, err := module.Execute(context.Background(), invocation(
				map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
			), runtimeStub{executor: executor})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs["entries"], test.want) {
				t.Fatalf("entries = %#v, want %#v", result.Outputs["entries"], test.want)
			}
			if result.Outputs["count"] != int64(len(test.want)) {
				t.Fatalf("count = %#v", result.Outputs["count"])
			}
		})
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
		{"invalid start", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "start": "abc"}, `pin "start" requires a number`},
		{"invalid stop", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "stop": true}, `pin "stop" requires a number`},
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

func TestRemoveForwardsMembers(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: removeDefinition(), Executor: executeRemove}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "leaderboard", "members": []any{"ada", 7}}
	result, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "ZREM" || !reflect.DeepEqual(executor.request.Args, []string{"leaderboard", "ada", "7"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Outputs["removed"] != int64(1) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRemoveErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{"members": []any{"a"}}, "key is required"},
		{"empty members", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": []any{}}, "at least one member is required"},
		{"missing database", map[string]any{}, map[string]any{"key": "k", "members": []any{"a"}}, "select a KV database first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: removeDefinition(), Executor: executeRemove}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), runtimeStub{executor: &executorStub{}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
