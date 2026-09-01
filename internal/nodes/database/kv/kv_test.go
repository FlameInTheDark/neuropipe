package kv

import (
	"context"
	"encoding/json"
	"errors"
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
		Node:            domain.FlowNode{ID: "kv-node", Type: "action:kv_get", Data: map[string]any{"config": config}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestPinBuilders(t *testing.T) {
	exec := Exec("in", "Exec", domain.PinInput)
	if exec.ID != "in" || exec.Kind != domain.PinExec || exec.Direction != domain.PinInput || exec.MaxConnections != 1 {
		t.Fatalf("Exec pin = %#v", exec)
	}
	tests := []struct {
		name     string
		pin      domain.NodePort
		dataType domain.DataType
		kind     domain.TypeKind
		required bool
	}{
		{"Text", Text("key", "Key", domain.PinInput, true), domain.DataText, domain.TypeString, true},
		{"Number", Number("count", "Count", domain.PinOutput, false), domain.DataNumber, domain.TypeFloat, false},
		{"Bool", Bool("ok", "OK", domain.PinOutput, false), domain.DataBoolean, domain.TypeBool, false},
		{"List", List("keys", "Keys", domain.PinInput, true), domain.DataList, domain.TypeList, true},
		{"Object", Object("fields", "Fields", domain.PinInput, true), domain.DataObject, domain.TypeMap, true},
		{"Any", Any("value", "Value", domain.PinOutput), domain.DataAny, domain.TypeAny, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.pin.Kind != domain.PinData || test.pin.Direction == "" || test.pin.MaxConnections != 1 {
				t.Fatalf("pin = %#v", test.pin)
			}
			if test.pin.DataType != test.dataType {
				t.Fatalf("dataType = %v, want %v", test.pin.DataType, test.dataType)
			}
			if test.pin.Type == nil || test.pin.Type.Kind != test.kind {
				t.Fatalf("type = %#v", test.pin.Type)
			}
			if test.pin.Required != test.required {
				t.Fatalf("required = %v", test.pin.Required)
			}
		})
	}
	if list := List("keys", "Keys", domain.PinInput, false); list.Type == nil || list.Type.Element == nil || list.Type.Element.Kind != domain.TypeAny {
		t.Fatalf("list element type = %#v", list.Type)
	}
	object := Object("fields", "Fields", domain.PinInput, false)
	if object.Type == nil || object.Type.Key == nil || object.Type.Key.Kind != domain.TypeString || object.Type.Value == nil || object.Type.Value.Kind != domain.TypeAny {
		t.Fatalf("object map type = %#v", object.Type)
	}
}

func TestDatabaseField(t *testing.T) {
	field := DatabaseField()
	if field.Name != "databaseId" || field.Kind != "kv-database-select" || !field.Required {
		t.Fatalf("database field = %#v", field)
	}
}

func TestDefinitionSkeleton(t *testing.T) {
	definition := Definition("action:kv_test", "Test", "Test node", nil, nil, nil, nil)
	if definition.Type != "action:kv_test" || definition.Category != "KV Store" || definition.Mode != domain.NodeImpure || !definition.PortContractOwned {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
		t.Fatalf("capabilities = %#v", definition.Capabilities)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "databaseId" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["databaseId"] != "" {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}

	extra := Definition("action:kv_test", "Test", "Test node", nil, nil,
		[]domain.ConfigField{{Name: "key", Label: "Key", Kind: "string"}}, map[string]any{"key": "k"})
	if len(extra.Fields) != 2 || extra.Fields[1].Name != "key" || extra.DefaultConfig["key"] != "k" || extra.DefaultConfig["databaseId"] != "" {
		t.Fatalf("fields = %#v, defaults = %#v", extra.Fields, extra.DefaultConfig)
	}
}

func TestDatabaseID(t *testing.T) {
	if id, err := DatabaseID(invocation(map[string]any{"databaseId": " db-1 "}, nil)); err != nil || id != "db-1" {
		t.Fatalf("DatabaseID() = %q, %v", id, err)
	}
	tests := []struct {
		name   string
		config map[string]any
	}{
		{"missing", map[string]any{}},
		{"blank", map[string]any{"databaseId": "   "}},
		{"non-string", map[string]any{"databaseId": 42}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DatabaseID(invocation(test.config, nil)); err == nil || !strings.Contains(err.Error(), "select a KV database first") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestExecuteCommandResolvesExecutor(t *testing.T) {
	executor := &executorStub{result: domain.KVCommandResult{Value: "ok"}}
	result, err := ExecuteCommand(context.Background(), invocation(map[string]any{"databaseId": "db-1"}, nil), runtimeStub{executor: executor}, domain.KVCommandRequest{Command: "GET", Args: []string{"k"}})
	if err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}
	if executor.request.DatabaseID != "db-1" || executor.request.Command != "GET" || executor.request.Args[0] != "k" {
		t.Fatalf("request = %#v", executor.request)
	}
	if result.Value != "ok" {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteCommandErrors(t *testing.T) {
	tests := []struct {
		name    string
		runtime nodes.Runtime
		config  map[string]any
		exec    *executorStub
		want    string
	}{
		{"nil runtime", nil, map[string]any{"databaseId": "db-1"}, &executorStub{}, "key/value database execution is unavailable"},
		{"non-provider runtime", struct{}{}, map[string]any{"databaseId": "db-1"}, &executorStub{}, "key/value database execution is unavailable"},
		{"nil executor", runtimeStub{}, map[string]any{"databaseId": "db-1"}, nil, "key/value database execution is unavailable"},
		{"missing database", runtimeStub{executor: &executorStub{}}, map[string]any{}, &executorStub{}, "select a KV database first"},
		{"executor error", runtimeStub{executor: &executorStub{err: errors.New("connection refused")}}, map[string]any{"databaseId": "db-1"}, nil, "connection refused"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ExecuteCommand(context.Background(), invocation(test.config, nil), test.runtime, domain.KVCommandRequest{Command: "GET"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	if value := String(invocation(nil, map[string]any{"key": "user:1"}), "key"); value != "user:1" {
		t.Fatalf("String() = %q", value)
	}
	if value := String(invocation(nil, map[string]any{}), "key"); value != "" {
		t.Fatalf("missing String() = %q", value)
	}
	if value := String(invocation(nil, map[string]any{"key": 42}), "key"); value != "" {
		t.Fatalf("non-string String() = %q", value)
	}
}

func TestStrings(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    []string
		wantErr string
	}{
		{"list with scalars", []any{"a", 7, true}, []string{"a", "7", "true"}, ""},
		{"single string", "solo", []string{"solo"}, ""},
		{"empty string", "", nil, ""},
		{"blank string", "   ", nil, ""},
		{"missing", nil, nil, ""},
		{"unconvertible item", []any{"a", make(chan int)}, nil, "item 2"},
		{"unconvertible scalar", make(chan int), nil, "encode argument"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Strings(invocation(nil, map[string]any{"keys": test.input}), "keys")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Strings() error = %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("Strings() = %#v, want %#v", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("Strings() = %#v, want %#v", got, test.want)
				}
			}
		})
	}
}

func TestStringMap(t *testing.T) {
	got, err := StringMap(invocation(nil, map[string]any{"fields": map[string]any{"a": 1.0, "b": true}}), "fields")
	if err != nil {
		t.Fatalf("StringMap() error = %v", err)
	}
	if got["a"] != "1" || got["b"] != "true" || len(got) != 2 {
		t.Fatalf("StringMap() = %#v", got)
	}
	if empty, err := StringMap(invocation(nil, map[string]any{}), "fields"); err != nil || len(empty) != 0 {
		t.Fatalf("missing StringMap() = %#v, %v", empty, err)
	}
	if _, err := StringMap(invocation(nil, map[string]any{"fields": "text"}), "fields"); err == nil || !strings.Contains(err.Error(), `pin "fields" requires an object value`) {
		t.Fatalf("non-object error = %v", err)
	}
	if _, err := StringMap(invocation(nil, map[string]any{"fields": map[string]any{"k": make(chan int)}}), "fields"); err == nil || !strings.Contains(err.Error(), `field "k"`) {
		t.Fatalf("field error = %v", err)
	}
}

func TestInt(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    int64
		wantErr bool
	}{
		{"float64", float64(42), 42, false},
		{"float64 truncated", float64(42.9), 42, false},
		{"int64", int64(7), 7, false},
		{"int", 9, 9, false},
		{"json number", json.Number("13"), 13, false},
		{"numeric string", "21", 21, false},
		{"blank string falls back", "   ", 5, false},
		{"invalid string", "abc", 0, true},
		{"bool", true, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Int(invocation(nil, map[string]any{"count": test.input}), "count", 5)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), `pin "count" requires a number`) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Int() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Int() = %d, want %d", got, test.want)
			}
		})
	}
	if fallback, err := Int(invocation(nil, map[string]any{}), "count", 3); err != nil || fallback != 3 {
		t.Fatalf("missing Int() = %d, %v", fallback, err)
	}
	if fallback, err := Int(invocation(nil, map[string]any{"count": nil}), "count", 3); err != nil || fallback != 3 {
		t.Fatalf("nil Int() = %d, %v", fallback, err)
	}
}

func TestFlag(t *testing.T) {
	if !Flag(invocation(nil, map[string]any{"ok": true}), "ok", false) {
		t.Fatal("Flag() = false for a wired true")
	}
	if Flag(invocation(nil, map[string]any{"ok": false}), "ok", true) {
		t.Fatal("Flag() = true for a wired false")
	}
	if !Flag(invocation(nil, map[string]any{}), "ok", true) {
		t.Fatal("Flag() did not fall back when unconnected")
	}
	if !Flag(invocation(nil, map[string]any{"ok": "yes"}), "ok", true) {
		t.Fatal("Flag() did not fall back for a non-bool value")
	}
}

func TestConfigFlag(t *testing.T) {
	if !ConfigFlag(invocation(map[string]any{"onlyIfMissing": true}, nil), "onlyIfMissing") {
		t.Fatal("ConfigFlag() = false for a configured true")
	}
	if ConfigFlag(invocation(map[string]any{"onlyIfMissing": false}, nil), "onlyIfMissing") {
		t.Fatal("ConfigFlag() = true for a configured false")
	}
	if ConfigFlag(invocation(map[string]any{}, nil), "onlyIfMissing") {
		t.Fatal("ConfigFlag() = true when unconfigured")
	}
	if ConfigFlag(invocation(map[string]any{"onlyIfMissing": "true"}, nil), "onlyIfMissing") {
		t.Fatal("ConfigFlag() = true for a non-bool value")
	}
}

func TestConfigStrings(t *testing.T) {
	got := ConfigStrings(invocation(map[string]any{"keys": []any{"user:1", "  ", 7, "user:2"}}, nil), "keys")
	if len(got) != 3 || got[0] != "user:1" || got[1] != "7" || got[2] != "user:2" {
		t.Fatalf("ConfigStrings() = %#v", got)
	}
	if empty := ConfigStrings(invocation(map[string]any{"keys": []any{}}, nil), "keys"); len(empty) != 0 {
		t.Fatalf("empty list = %#v", empty)
	}
	// Legacy newline-separated textareas were removed: only the array shape
	// persisted by the visual list editor is accepted.
	if legacy := ConfigStrings(invocation(map[string]any{"keys": "a\nb"}, nil), "keys"); legacy != nil {
		t.Fatalf("string config produced %#v, want nil", legacy)
	}
	if missing := ConfigStrings(invocation(map[string]any{}, nil), "keys"); missing != nil {
		t.Fatalf("missing config produced %#v, want nil", missing)
	}
	if wrong := ConfigStrings(invocation(map[string]any{"keys": map[string]any{"a": "b"}}, nil), "keys"); wrong != nil {
		t.Fatalf("map config produced %#v, want nil", wrong)
	}
	skipped := ConfigStrings(invocation(map[string]any{"keys": []any{"a", make(chan int), "b"}}, nil), "keys")
	if len(skipped) != 2 || skipped[0] != "a" || skipped[1] != "b" {
		t.Fatalf("unconvertible items must be skipped: %#v", skipped)
	}
}

func TestArg(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{"nil", nil, "", false},
		{"string", "text", "text", false},
		{"true", true, "true", false},
		{"false", false, "false", false},
		{"float fraction", 1.5, "1.5", false},
		{"float whole", float64(2), "2", false},
		{"int64", int64(3), "3", false},
		{"int", 4, "4", false},
		{"json number", json.Number("5.25"), "5.25", false},
		{"list", []any{"a", 1}, `["a",1]`, false},
		{"object", map[string]any{"k": "v"}, `{"k":"v"}`, false},
		{"other types JSON-encoded", struct{ A int }{A: 1}, `{"A":1}`, false},
		{"unmarshalable", make(chan int), "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Arg(test.value)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "encode argument") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Arg() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Arg() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScoredEntries(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    []ScoredEntry
		wantErr string
	}{
		{
			name:  "float and converted scores",
			input: []any{map[string]any{"member": "ada", "score": 1.5}, map[string]any{"member": 7, "score": int64(2)}, map[string]any{"member": "grace", "score": 3}, map[string]any{"member": "lin", "score": "4.5"}},
			want:  []ScoredEntry{{Member: "ada", Score: 1.5}, {Member: "7", Score: 2}, {Member: "grace", Score: 3}, {Member: "lin", Score: 4.5}},
		},
		{name: "missing score", input: []any{map[string]any{"member": "ada"}}, wantErr: "entry 1 score is required"},
		{name: "nil score", input: []any{map[string]any{"member": "ada", "score": nil}}, wantErr: "entry 1 score is required"},
		{name: "blank score", input: []any{map[string]any{"member": "ada", "score": "  "}}, wantErr: "entry 1 score is required"},
		{name: "bad score text", input: []any{map[string]any{"member": "ada", "score": "high"}}, wantErr: `entry 1 score`},
		{name: "bool score", input: []any{map[string]any{"member": "ada", "score": true}}, wantErr: "entry 1 score must be a number"},
		{name: "entry not an object", input: []any{"ada"}, wantErr: "entry 1 requires an object with member and score"},
		{name: "pin not a list", input: "ada", wantErr: `pin "entries" requires a list of member/score entries`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ScoredEntries(invocation(nil, map[string]any{"entries": test.input}), "entries")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ScoredEntries() error = %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("ScoredEntries() = %#v, want %#v", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("ScoredEntries() = %#v, want %#v", got, test.want)
				}
			}
		})
	}
	if got, err := ScoredEntries(invocation(nil, map[string]any{}), "entries"); err != nil || got != nil {
		t.Fatalf("missing ScoredEntries() = %#v, %v", got, err)
	}
	if got, err := ScoredEntries(invocation(nil, map[string]any{"entries": nil}), "entries"); err != nil || got != nil {
		t.Fatalf("nil ScoredEntries() = %#v, %v", got, err)
	}
	if _, err := ScoredEntries(invocation(nil, map[string]any{"entries": []any{map[string]any{"member": make(chan int), "score": 1.0}}}), "entries"); err == nil || !strings.Contains(err.Error(), "entry 1 member") {
		t.Fatalf("member error = %v", err)
	}
}
