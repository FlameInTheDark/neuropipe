package scan

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// executorStub records every command and answers from a per-command table so
// the info node's INFO + DBSIZE sequence can be stubbed in one runtime.
type executorStub struct {
	requests []domain.KVCommandRequest
	results  map[string]domain.KVCommandResult
	err      error
}

func (e *executorStub) ExecuteCommand(_ context.Context, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	e.requests = append(e.requests, request)
	if e.err != nil {
		return domain.KVCommandResult{}, e.err
	}
	if result, ok := e.results[request.Command]; ok {
		return result, nil
	}
	return domain.KVCommandResult{}, nil
}

type runtimeStub struct{ executor nodes.KVExecutor }

func (r runtimeStub) KVExecutor() nodes.KVExecutor { return r.executor }

func invocation(config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "scan-node", Type: "action:kv_scan", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegister(t *testing.T) {
	registry := nodes.New()
	for _, register := range []func(nodes.Registrar) error{RegisterScan, RegisterPublish, RegisterInfo} {
		if err := register(registry); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	for _, nodeType := range []string{"action:kv_scan", "action:kv_publish", "action:kv_info"} {
		module, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s was not registered", nodeType)
		}
		if definition := module.Definition(); definition.Category != "KV Store" || definition.Mode != domain.NodeImpure {
			t.Fatalf("%s definition = %#v", nodeType, definition)
		}
	}
}

func TestScanDefaultArguments(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{
		"SCAN": {Value: []any{"0", []any{"user:1", "user:2"}}},
	}}
	module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.requests[0].Command != "SCAN" || !reflect.DeepEqual(executor.requests[0].Args, []string{"0", "COUNT", "100"}) {
		t.Fatalf("request = %#v", executor.requests[0])
	}
	if !reflect.DeepEqual(result.Outputs["keys"], []any{"user:1", "user:2"}) {
		t.Fatalf("keys = %#v", result.Outputs["keys"])
	}
	if result.Outputs["nextCursor"] != uint64(0) || result.Outputs["done"] != true {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestScanPatternTypeAndCountArguments(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{
		"SCAN": {Value: []any{"17", []any{"user:1"}}},
	}}
	module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
	config := map[string]any{"databaseId": "db-1", "count": 250, "typeFilter": "hash"}
	result, err := module.Execute(context.Background(), invocation(config, map[string]any{
		"pattern": "user:*", "cursor": float64(3),
	}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(executor.requests[0].Args, []string{"3", "MATCH", "user:*", "COUNT", "250", "TYPE", "hash"}) {
		t.Fatalf("args = %#v", executor.requests[0].Args)
	}
	if result.Outputs["nextCursor"] != uint64(17) || result.Outputs["done"] != false {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestScanCountClamping(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		count  string
	}{
		{"below one is clamped to one", map[string]any{"databaseId": "db-1", "count": 0}, map[string]any{}, "1"},
		{"negative is clamped to one", map[string]any{"databaseId": "db-1", "count": -5}, map[string]any{}, "1"},
		{"above 500 is clamped to 500", map[string]any{"databaseId": "db-1", "count": 1000}, map[string]any{}, "500"},
		{"string count is parsed", map[string]any{"databaseId": "db-1", "count": "80"}, map[string]any{}, "80"},
		{"invalid string count falls back to 100", map[string]any{"databaseId": "db-1", "count": "many"}, map[string]any{}, "100"},
		{"cursor input is used", map[string]any{"databaseId": "db-1"}, map[string]any{"cursor": "42"}, "100"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{}
			module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
			if _, err := module.Execute(context.Background(), invocation(test.config, test.inputs), runtimeStub{executor: executor}); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			want := []string{"0", "COUNT", test.count}
			if test.inputs["cursor"] != nil {
				want[0] = "42"
			}
			if !reflect.DeepEqual(executor.requests[0].Args, want) {
				t.Fatalf("args = %#v, want %#v", executor.requests[0].Args, want)
			}
		})
	}
}

func TestScanNegativeCursorResetsToZero(t *testing.T) {
	executor := &executorStub{}
	module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
	if _, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"cursor": float64(-9)},
	), runtimeStub{executor: executor}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.requests[0].Args[0] != "0" {
		t.Fatalf("args = %#v", executor.requests[0].Args)
	}
}

func TestScanCursorReplyCoercions(t *testing.T) {
	tests := []struct {
		name       string
		reply      any
		nextCursor uint64
		done       bool
	}{
		{"int64 cursor", []any{int64(11), []any{}}, 11, false},
		{"float cursor", []any{float64(12), []any{}}, 12, false},
		{"string cursor", []any{"13", []any{}}, 13, false},
		{"zero cursor finishes", []any{"0", []any{}}, 0, true},
		{"negative cursor is zero", []any{int64(-1), []any{}}, 0, true},
		{"unparseable cursor is zero", []any{"abc", []any{}}, 0, true},
		{"short reply", []any{"17"}, 0, true},
		{"non-list reply", "unexpected", 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{results: map[string]domain.KVCommandResult{"SCAN": {Value: test.reply}}}
			module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
			result, err := module.Execute(context.Background(), invocation(
				map[string]any{"databaseId": "db-1"}, map[string]any{},
			), runtimeStub{executor: executor})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.Outputs["nextCursor"] != test.nextCursor || result.Outputs["done"] != test.done {
				t.Fatalf("outputs = %#v, want cursor %d done %v", result.Outputs, test.nextCursor, test.done)
			}
		})
	}
}

func TestScanErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		inputs  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"invalid cursor", map[string]any{"databaseId": "db-1"}, map[string]any{"cursor": true}, runtimeStub{executor: &executorStub{}}, `pin "cursor" requires a number`},
		{"missing database", map[string]any{}, map[string]any{}, runtimeStub{executor: &executorStub{}}, "select a KV database first"},
		{"nil runtime", map[string]any{"databaseId": "db-1"}, map[string]any{}, nil, "key/value database execution is unavailable"},
		{"executor error", map[string]any{"databaseId": "db-1"}, map[string]any{}, runtimeStub{executor: &executorStub{err: errors.New("scan failed")}}, "scan failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPublishSendsChannelAndMessage(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{"PUBLISH": {Value: int64(3)}}}
	module := nodes.Implementation{Metadata: publishDefinition(), Executor: executePublish}
	result, err := module.Execute(context.Background(), invocation(
		map[string]any{"databaseId": "db-1"}, map[string]any{"channel": "events:signup", "message": "hello"},
	), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.requests[0].Command != "PUBLISH" || !reflect.DeepEqual(executor.requests[0].Args, []string{"events:signup", "hello"}) {
		t.Fatalf("request = %#v", executor.requests[0])
	}
	if result.Outputs["receivers"] != int64(3) || result.Ports[0] != "out" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPublishReceiverCoercions(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int64
	}{
		{"float", float64(2.9), 2},
		{"string is zero", "3", 0},
		{"nil is zero", nil, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &executorStub{results: map[string]domain.KVCommandResult{"PUBLISH": {Value: test.value}}}
			module := nodes.Implementation{Metadata: publishDefinition(), Executor: executePublish}
			result, err := module.Execute(context.Background(), invocation(
				map[string]any{"databaseId": "db-1"}, map[string]any{"channel": "ch", "message": "m"},
			), runtimeStub{executor: executor})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.Outputs["receivers"] != test.want {
				t.Fatalf("receivers = %#v, want %d", result.Outputs["receivers"], test.want)
			}
		})
	}
}

func TestPublishErrors(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing channel", map[string]any{"databaseId": "db-1"}, map[string]any{"message": "m"}, "channel is required"},
		{"missing database", map[string]any{}, map[string]any{"channel": "ch", "message": "m"}, "select a KV database first"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: publishDefinition(), Executor: executePublish}
			_, err := module.Execute(context.Background(), invocation(test.config, test.inputs), runtimeStub{executor: &executorStub{}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInfoParsesRedisServerSection(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{
		"INFO":   {Value: "# Server\r\nredis_version:7.2.4\r\nuptime_in_seconds:1200\r\nconnected_clients:3\r\nused_memory:1048576\r\nused_memory_human:1M\r\n# Keyspace\r\ndb0:keys=10,expires=0\r\n"},
		"DBSIZE": {Value: int64(42)},
	}}
	module := nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, map[string]any{}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(executor.requests) != 2 || executor.requests[0].Command != "INFO" || executor.requests[1].Command != "DBSIZE" {
		t.Fatalf("requests = %#v", executor.requests)
	}
	if result.Outputs["version"] != "7.2.4" || result.Outputs["flavor"] != "redis" || result.Outputs["keyCount"] != int64(42) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	info, ok := result.Outputs["info"].(map[string]any)
	if !ok {
		t.Fatalf("info = %#v", result.Outputs["info"])
	}
	if info["uptimeSeconds"] != int64(1200) || info["connectedClients"] != int64(3) || info["usedMemory"] != int64(1048576) || info["usedMemoryHuman"] != "1M" || info["totalKeys"] != int64(42) {
		t.Fatalf("info = %#v", info)
	}
	if result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestInfoDetectsValkeyFlavor(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{
		"INFO":   {Value: "valkey_version:8.0.0\n"},
		"DBSIZE": {Value: int64(0)},
	}}
	module := nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, map[string]any{}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["flavor"] != "valkey" || result.Outputs["version"] != "8.0.0" || result.Outputs["keyCount"] != int64(0) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestInfoWithNonStringReply(t *testing.T) {
	executor := &executorStub{results: map[string]domain.KVCommandResult{
		"INFO":   {Value: nil},
		"DBSIZE": {Value: float64(7)},
	}}
	module := nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo}
	result, err := module.Execute(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, map[string]any{}), runtimeStub{executor: executor})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	info := result.Outputs["info"].(map[string]any)
	if len(info) != 1 || info["totalKeys"] != int64(7) {
		t.Fatalf("info = %#v", info)
	}
	// A non-string INFO reply leaves the parsed fields empty; only the
	// DBSIZE-driven totalKeys is reported.
	if result.Outputs["version"] != "" || result.Outputs["flavor"] != "" || result.Outputs["keyCount"] != int64(7) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestInfoErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		runtime nodes.Runtime
		want    string
	}{
		{"missing database", map[string]any{}, runtimeStub{executor: &executorStub{}}, "select a KV database first"},
		{"nil runtime", map[string]any{"databaseId": "db-1"}, nil, "key/value database execution is unavailable"},
		{"info failure", map[string]any{"databaseId": "db-1"}, runtimeStub{executor: &executorStub{err: errors.New("info boom")}}, "info boom"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo}
			_, err := module.Execute(context.Background(), invocation(test.config, map[string]any{}), test.runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveInfoDocumentsInfoFields(t *testing.T) {
	module := nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo}
	node := domain.FlowNode{Type: "action:kv_info", Data: map[string]any{"config": map[string]any{"databaseId": "db-1"}}}
	definition, err := module.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	infoPin := definition.Outputs[1]
	if infoPin.ID != "info" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	paths := map[string]domain.DataType{}
	for _, field := range infoPin.Fields {
		paths[field.Path] = field.DataType
	}
	want := map[string]domain.DataType{
		"flavor": domain.DataText, "version": domain.DataText, "uptimeSeconds": domain.DataNumber,
		"connectedClients": domain.DataNumber, "usedMemory": domain.DataNumber, "usedMemoryHuman": domain.DataText,
		"totalKeys": domain.DataNumber, "databases": domain.DataList,
	}
	if len(paths) != len(want) {
		t.Fatalf("info fields = %#v", paths)
	}
	for path, dataType := range want {
		if paths[path] != dataType {
			t.Errorf("info field %q = %v, want %v", path, paths[path], dataType)
		}
	}
}
