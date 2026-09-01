package setglobalvariable_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setglobalvariable"
)

// declaredTypes backs the package's declared-type hook so literal coercion and
// port typing can be exercised per variable name.
var declaredTypes = map[string]domain.DataType{
	"counter":  domain.DataNumber,
	"flag":     domain.DataBoolean,
	"cfg":      domain.DataObject,
	"items":    domain.DataList,
	"greeting": domain.DataText,
}

func init() {
	setglobalvariable.SetDeclaredType(func(name string) (domain.DataType, bool) {
		kind, ok := declaredTypes[name]
		return kind, ok
	})
}

type globalsStub struct {
	writes          map[string]any
	incrementDeltas []float64
	incrementResult float64
	appends         map[string]any
	appendResult    []any
	writeErr        error
	incrementErr    error
	appendErr       error
}

func (s *globalsStub) WriteGlobalVariable(name string, value any) error {
	if s.writes == nil {
		s.writes = map[string]any{}
	}
	s.writes[name] = value
	return s.writeErr
}

func (s *globalsStub) IncrementGlobalVariable(name string, delta float64) (float64, error) {
	s.incrementDeltas = append(s.incrementDeltas, delta)
	return s.incrementResult, s.incrementErr
}

func (s *globalsStub) AppendGlobalVariable(name string, item any) ([]any, error) {
	if s.appends == nil {
		s.appends = map[string]any{}
	}
	s.appends[name] = item
	return s.appendResult, s.appendErr
}

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := setglobalvariable.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:set_global_variable")
	if !ok {
		t.Fatal("flow:set_global_variable was not registered")
	}
	return registered
}

func invocation(registered nodes.Node, config map[string]any, inputs map[string]any, connected map[string]bool) nodes.Invocation {
	if connected == nil {
		connected = map[string]bool{}
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "global-1", Type: "flow:set_global_variable", Data: map[string]any{"config": config}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: connected,
	}
}

func flowNode(config map[string]any) domain.FlowNode {
	return domain.FlowNode{ID: "global-1", Type: "flow:set_global_variable", Data: map[string]any{"config": config}}
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:set_global_variable" || definition.Mode != domain.NodeImpure || definition.Category != "Data" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "value" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	if !definition.Inputs[1].IgnoreConfigFallback {
		t.Fatalf("value pin must not fall back to inspector config: %#v", definition.Inputs[1])
	}
	if len(definition.Outputs) != 2 || definition.Outputs[0].ID != "out" || definition.Outputs[1].ID != "result" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	kinds := map[string]domain.ConfigField{}
	for _, field := range definition.Fields {
		kinds[field.Name] = field
	}
	if kinds["name"].Kind != "select" || !kinds["name"].Required {
		t.Fatalf("name field = %#v", kinds["name"])
	}
	if kinds["operation"].Kind != "select" || len(kinds["operation"].Options) != 3 {
		t.Fatalf("operation field = %#v", kinds["operation"])
	}
	if kinds["value"].Kind != "string" {
		t.Fatalf("value field = %#v", kinds["value"])
	}
	if definition.DefaultConfig["operation"] != "set" {
		t.Fatalf("default config = %#v", definition.DefaultConfig)
	}
}

func TestResolveTypesPortsByOperation(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]any
		wantInputType  domain.DataType
		wantOutputType domain.DataType
	}{
		{
			"set keeps declared text type",
			map[string]any{"name": "greeting", "operation": "set"},
			domain.DataText,
			domain.DataText,
		},
		{
			"increment types value and result as number",
			map[string]any{"name": "anything", "operation": "increment"},
			domain.DataNumber,
			domain.DataNumber,
		},
		{
			"append keeps any input and lists result",
			map[string]any{"name": "anything", "operation": "append"},
			domain.DataAny,
			domain.DataList,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition, err := module(t).Resolve(flowNode(test.config))
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			var valuePort, resultPort *domain.NodePort
			for index := range definition.Inputs {
				if definition.Inputs[index].ID == "value" {
					valuePort = &definition.Inputs[index]
				}
			}
			for index := range definition.Outputs {
				if definition.Outputs[index].ID == "result" {
					resultPort = &definition.Outputs[index]
				}
			}
			if valuePort == nil || valuePort.DataType != test.wantInputType {
				t.Fatalf("value port = %#v, want %s", valuePort, test.wantInputType)
			}
			if resultPort == nil || resultPort.DataType != test.wantOutputType {
				t.Fatalf("result port = %#v, want %s", resultPort, test.wantOutputType)
			}
		})
	}
}

func TestResolveInjectsDeclaredVariableOptions(t *testing.T) {
	setglobalvariable.SetDeclaredOptions(func() []domain.Option {
		return []domain.Option{{Value: "counter", Label: "Counter"}, {Value: "greeting", Label: "Greeting"}}
	})
	defer setglobalvariable.SetDeclaredOptions(nil)
	definition, err := module(t).Resolve(flowNode(map[string]any{"name": "counter", "operation": "set"}))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for _, field := range definition.Fields {
		if field.Name == "name" {
			if len(field.Options) != 2 || field.Options[0].Value != "counter" {
				t.Fatalf("name options = %#v", field.Options)
			}
			return
		}
	}
	t.Fatal("name field was not resolved")
}

func TestExecuteSetWritesConnectedValue(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{}
	result, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting", "operation": "set"},
		map[string]any{"value": "hello"},
		map[string]bool{"value": true},
	), globals)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if globals.writes["greeting"] != "hello" {
		t.Fatalf("writes = %#v", globals.writes)
	}
	if result.Outputs["result"] != "hello" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestExecuteSetCoercesLiteralToDeclaredType(t *testing.T) {
	tests := []struct {
		name        string
		variable    string
		literal     string
		wantWritten any
	}{
		{"number literal", "counter", "1.5", float64(1.5)},
		{"boolean literal", "flag", "true", true},
		{"object literal", "cfg", `{"a":1}`, map[string]any{"a": float64(1)}},
		{"list literal", "items", "[1,2]", []any{float64(1), float64(2)}},
		{"text passthrough", "greeting", "hello", "hello"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			globals := &globalsStub{}
			config := map[string]any{"name": test.variable, "operation": "set", "value": test.literal}
			if _, err := registered.Execute(context.Background(), invocation(registered, config, map[string]any{}, nil), globals); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(globals.writes[test.variable], test.wantWritten) {
				t.Fatalf("writes[%q] = %#v, want %#v", test.variable, globals.writes[test.variable], test.wantWritten)
			}
		})
	}
}

func TestExecuteSetRejectsInvalidLiteral(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		literal  string
		wantErr  string
	}{
		{"non-number", "counter", "abc", "is not a number"},
		{"non-boolean", "flag", "yes", "is not a boolean"},
		{"non-object", "cfg", "nope", "must be a JSON object"},
		{"non-list", "items", "1", "must be a JSON list"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			config := map[string]any{"name": test.variable, "operation": "set", "value": test.literal}
			_, err := registered.Execute(context.Background(), invocation(registered, config, map[string]any{}, nil), &globalsStub{})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Execute() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestExecuteSetRequiresValue(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{}
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting", "operation": "set"},
		map[string]any{},
		nil,
	), globals)
	if err == nil || !strings.Contains(err.Error(), "connect the Value pin or enter a literal value") {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(globals.writes) != 0 {
		t.Fatalf("writes = %#v", globals.writes)
	}
}

func TestExecuteIncrementDefaultsToOne(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{incrementResult: float64(4)}
	result, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "counter", "operation": "increment"},
		map[string]any{},
		nil,
	), globals)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(globals.incrementDeltas) != 1 || globals.incrementDeltas[0] != 1 {
		t.Fatalf("deltas = %#v, want [1]", globals.incrementDeltas)
	}
	if result.Outputs["result"] != float64(4) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
}

func TestExecuteIncrementUsesProvidedDelta(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		wantDelta float64
	}{
		{"connected float delta", float64(2.5), 2.5},
		{"connected int delta", 3, 3},
		{"declared number literal", "2", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			globals := &globalsStub{}
			config := map[string]any{"name": "counter", "operation": "increment"}
			inputs := map[string]any{"value": test.value}
			connected := map[string]bool{"value": true}
			if test.name == "declared number literal" {
				config["value"] = test.value
				inputs = map[string]any{}
				connected = nil
			}
			_, err := registered.Execute(context.Background(), invocation(registered, config, inputs, connected), globals)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(globals.incrementDeltas) != 1 || globals.incrementDeltas[0] != test.wantDelta {
				t.Fatalf("deltas = %#v, want [%v]", globals.incrementDeltas, test.wantDelta)
			}
		})
	}
}

func TestExecuteIncrementRejectsNonNumericValue(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{}
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting", "operation": "increment"},
		map[string]any{"value": "soon"},
		map[string]bool{"value": true},
	), globals)
	if err == nil || !strings.Contains(err.Error(), "increment value") {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(globals.incrementDeltas) != 0 {
		t.Fatalf("deltas = %#v", globals.incrementDeltas)
	}
}

func TestExecuteAppendAppendsItemAndReturnsList(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{appendResult: []any{"old", "new"}}
	result, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "items", "operation": "append"},
		map[string]any{"value": "new"},
		map[string]bool{"value": true},
	), globals)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if globals.appends["items"] != "new" {
		t.Fatalf("appends = %#v", globals.appends)
	}
	if !reflect.DeepEqual(result.Outputs["result"], []any{"old", "new"}) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
}

func TestExecuteDefaultsToSetOperation(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{}
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting"},
		map[string]any{"value": "hello"},
		map[string]bool{"value": true},
	), globals)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if globals.writes["greeting"] != "hello" {
		t.Fatalf("writes = %#v", globals.writes)
	}
}

func TestExecuteRejectsUnknownOperation(t *testing.T) {
	registered := module(t)
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting", "operation": "merge"},
		map[string]any{"value": "hello"},
		map[string]bool{"value": true},
	), &globalsStub{})
	if err == nil || !strings.Contains(err.Error(), `unknown operation "merge"`) {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestExecuteRequiresVariableName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		t.Run("name "+name, func(t *testing.T) {
			registered := module(t)
			_, err := registered.Execute(context.Background(), invocation(registered,
				map[string]any{"name": name, "operation": "set"},
				map[string]any{"value": "hello"},
				map[string]bool{"value": true},
			), &globalsStub{})
			if err == nil || !strings.Contains(err.Error(), "select a variable to write") {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestExecuteTrimsVariableName(t *testing.T) {
	registered := module(t)
	globals := &globalsStub{}
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "  greeting  ", "operation": "set"},
		map[string]any{"value": "hello"},
		map[string]bool{"value": true},
	), globals)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, written := globals.writes["greeting"]; !written {
		t.Fatalf("writes = %#v, want trimmed name", globals.writes)
	}
}

func TestExecutePropagatesHostErrors(t *testing.T) {
	tests := []struct {
		name    string
		globals *globalsStub
		config  map[string]any
	}{
		{
			"write failure",
			&globalsStub{writeErr: context.Canceled},
			map[string]any{"name": "greeting", "operation": "set"},
		},
		{
			"increment failure",
			&globalsStub{incrementErr: context.Canceled},
			map[string]any{"name": "counter", "operation": "increment"},
		},
		{
			"append failure",
			&globalsStub{appendErr: context.Canceled},
			map[string]any{"name": "items", "operation": "append"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			_, err := registered.Execute(context.Background(), invocation(registered, test.config,
				map[string]any{"value": "hello"},
				map[string]bool{"value": true},
			), test.globals)
			if err == nil {
				t.Fatal("Execute() did not propagate the host error")
			}
		})
	}
}

func TestExecuteRequiresGlobalVariableWriter(t *testing.T) {
	registered := module(t)
	_, err := registered.Execute(context.Background(), invocation(registered,
		map[string]any{"name": "greeting", "operation": "set"},
		map[string]any{"value": "hello"},
		map[string]bool{"value": true},
	), nil)
	if err == nil || !strings.Contains(err.Error(), "global variable runtime is unavailable") {
		t.Fatalf("Execute() error = %v", err)
	}
}
