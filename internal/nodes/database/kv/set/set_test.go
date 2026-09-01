package set

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
		Node:            domain.FlowNode{ID: "set-node", Type: "action:kv_set_add", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegister(t *testing.T) {
	registry := nodes.New()
	for _, register := range []func(nodes.Registrar) error{RegisterAdd, RegisterMembers, RegisterRemove} {
		if err := register(registry); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	for _, nodeType := range []string{"action:kv_set_add", "action:kv_set_members", "action:kv_set_remove"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		if definition := module.Definition(); definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
	}
}

func TestAddForwardsMembers(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(2)}}
	module := nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "tags:article-7", "members": []any{"go", "redis", 7}}
	result, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "SADD" || executor.request.DatabaseID != "db-1" {
		t.Fatalf("request = %#v", executor.request)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"tags:article-7", "go", "redis", "7"}) {
		t.Fatalf("args = %#v", executor.request.Args)
	}
	if result.Outputs["added"] != int64(2) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAddConvertsScalarMember(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: int64(1)}}
	module := nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd}
	config := map[string]any{"databaseId": "db-1"}
	inputs := map[string]any{"key": "tags", "members": true}
	if _, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.request.Args, []string{"tags", "true"}) {
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
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{"members": []any{"a"}}, runtimeStub{executor: &executorStub{}}, "key is required"},
		{"empty members", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": []any{}}, runtimeStub{executor: &executorStub{}}, "at least one member is required"},
		{"blank scalar members", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": "  "}, runtimeStub{executor: &executorStub{}}, "at least one member is required"},
		{"unconvertible member", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": []any{make(chan int)}}, runtimeStub{executor: &executorStub{}}, "item 1"},
		{"missing database", map[string]any{}, map[string]any{"key": "k", "members": []any{"a"}}, runtimeStub{executor: &executorStub{}}, "select a KV database first"},
		{"nil runtime", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": []any{"a"}}, nil, "key/value database execution is unavailable"},
		{"executor error", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k", "members": []any{"a"}}, runtimeStub{executor: &executorStub{err: errors.New("sadd boom")}}, "sadd boom"},
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

func TestMembersListsAllMembers(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: []any{"go", "redis"}}}
	module := nodes.Implementation{Metadata: membersDefinition(), Executor: executeMembers}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "tags:article-7"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "SMEMBERS" || !reflect.DeepEqual(executor.request.Args, []string{"tags:article-7"}) {
		t.Fatalf("request = %#v", executor.request)
	}
	if !reflect.DeepEqual(result.Outputs["members"], []any{"go", "redis"}) || result.Outputs["count"] != int64(2) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestMembersNonListReplyYieldsEmpty(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: nil}}
	module := nodes.Implementation{Metadata: membersDefinition(), Executor: executeMembers}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(result.Outputs["members"], []any{}) || result.Outputs["count"] != int64(0) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestMembersErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing key", map[string]any{"databaseId": "db-1"}, map[string]any{}, "key is required"},
		{"missing database", map[string]any{}, map[string]any{"key": "k"}, "select a KV database first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: membersDefinition(), Executor: executeMembers}
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
	inputs := map[string]any{"key": "tags:article-7", "members": []any{"go"}}
	result, err := module.Execute(context.Background(), invocation(config, inputs), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.request.Command != "SREM" || !reflect.DeepEqual(executor.request.Args, []string{"tags:article-7", "go"}) {
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
		{"empty members", map[string]any{"databaseId": "db-1"}, map[string]any{"key": "k"}, "at least one member is required"},
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
