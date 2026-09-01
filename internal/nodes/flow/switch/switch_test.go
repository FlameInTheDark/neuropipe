package switchnode_test

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	switchnode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/switch"
)

func module(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := switchnode.Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	registered, ok := registry.Get("flow:switch")
	if !ok {
		t.Fatal("flow:switch was not registered")
	}
	return registered
}

func switchConfig(comparator string, cases ...map[string]any) map[string]any {
	// Cases arrive from persisted JSON as []any, the shape the parser expects.
	items := make([]any, 0, len(cases))
	for _, item := range cases {
		items = append(items, item)
	}
	return map[string]any{"switch": map[string]any{"comparator": comparator, "cases": items}}
}

func textCase(id, label, value string) map[string]any {
	return map[string]any{"id": id, "label": label, "valueType": "text", "value": value}
}

func numberCase(id, label string, value float64) map[string]any {
	return map[string]any{"id": id, "label": label, "valueType": "number", "value": value}
}

func booleanCase(id, label string, value bool) map[string]any {
	return map[string]any{"id": id, "label": label, "valueType": "boolean", "value": value}
}

func invocation(registered nodes.Node, config map[string]any, selection any) nodes.Invocation {
	inputs := map[string]any{}
	if selection != nil {
		inputs["selection"] = selection
	}
	return nodes.Invocation{
		Node:            domain.FlowNode{ID: "switch-1", Type: "flow:switch", Data: map[string]any{"config": config}},
		Definition:      registered.Definition(),
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func outputIDs(definition domain.NodeDefinition) []string {
	ids := make([]string, 0, len(definition.Outputs))
	for _, port := range definition.Outputs {
		ids = append(ids, port.ID)
	}
	return ids
}

func TestRegisterMetadata(t *testing.T) {
	definition := module(t).Definition()
	if definition.Type != "flow:switch" || definition.Mode != domain.NodeImpure || definition.Category != "Flow" {
		t.Fatalf("definition = %#v", definition)
	}
	if len(definition.Inputs) != 2 || definition.Inputs[0].ID != "in" || definition.Inputs[1].ID != "selection" {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	// The static contract carries only the default port; cases resolve ports.
	if len(definition.Outputs) != 1 || definition.Outputs[0].ID != "default" {
		t.Fatalf("outputs = %#v", definition.Outputs)
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Name != "switch" || definition.Fields[0].Kind != "switch-cases" || !definition.Fields[0].Required {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["switch"] == nil {
		t.Fatalf("default config = %#v", definition.DefaultConfig)
	}
}

func TestResolveProducesCasePortsPlusDefault(t *testing.T) {
	config := switchConfig("equals",
		textCase("case-a", "Alpha", "a"),
		textCase("case-b", "Beta", "b"),
		textCase("case-c", "Gamma", "c"),
	)
	definition, err := module(t).Resolve(domain.FlowNode{ID: "switch-1", Type: "flow:switch", Data: map[string]any{"config": config}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	got := outputIDs(definition)
	want := []string{"case-a", "case-b", "case-c", "default"}
	if len(got) != len(want) {
		t.Fatalf("outputs = %#v, want %v", got, want)
	}
	for index, id := range want {
		if got[index] != id {
			t.Fatalf("outputs = %#v, want %v", got, want)
		}
	}
	for index, port := range definition.Outputs {
		if port.Kind != domain.PinExec {
			t.Fatalf("port %q kind = %q", port.ID, port.Kind)
		}
		if index < 3 && port.Label != []string{"Alpha", "Beta", "Gamma"}[index] {
			t.Fatalf("port %q label = %q", port.ID, port.Label)
		}
	}
}

func TestResolveFallsBackToDefaultConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
	}{
		{"empty config", map[string]any{}},
		// Legacy V2 `options` configuration is no longer read; only the
		// `switch` object with comparator and cases is valid.
		{"legacy options ignored", map[string]any{"options": []any{map[string]any{"id": "legacy", "label": "Legacy"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition, err := module(t).Resolve(domain.FlowNode{ID: "switch-1", Type: "flow:switch", Data: map[string]any{"config": test.config}})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			got := outputIDs(definition)
			want := []string{"case-a", "case-b", "default"}
			if len(got) != len(want) {
				t.Fatalf("outputs = %#v, want %v", got, want)
			}
			for index, id := range want {
				if got[index] != id {
					t.Fatalf("outputs = %#v, want %v", got, want)
				}
			}
		})
	}
}

func TestResolveRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr string
	}{
		{
			"non-object switch",
			map[string]any{"switch": "nope"},
			"switch configuration must be an object",
		},
		{
			"unsupported comparator",
			switchConfig("like", textCase("a", "A", "x")),
			`switch comparator "like" is not supported`,
		},
		{
			"cases missing",
			map[string]any{"switch": map[string]any{"comparator": "equals"}},
			"switch cases must be a list",
		},
		{
			"empty cases",
			switchConfig("equals"),
			"add at least one switch case",
		},
		{
			"case not an object",
			map[string]any{"switch": map[string]any{"comparator": "equals", "cases": []any{"not-an-object"}}},
			"switch case 1 must be an object",
		},
		{
			"blank case id",
			switchConfig("equals", map[string]any{"id": "   ", "label": "A", "valueType": "text", "value": "x"}),
			"switch case 1 needs an ID",
		},
		{
			"blank case label",
			switchConfig("equals", map[string]any{"id": "a", "label": "", "valueType": "text", "value": "x"}),
			`switch case "a" needs a pin name`,
		},
		{
			"duplicate case id",
			switchConfig("equals", textCase("a", "A", "x"), textCase("a", "B", "y")),
			`switch cases contain duplicate ID "a"`,
		},
		{
			"duplicate case label",
			switchConfig("equals", textCase("a", "Alpha", "x"), textCase("b", "alpha", "y")),
			`switch cases contain duplicate pin name "alpha"`,
		},
		{
			"text case with numeric comparator",
			switchConfig("greater_than", textCase("a", "A", "x")),
			`switch case "a" cannot use text with comparator "greater_than"`,
		},
		{
			"number case with text comparator",
			switchConfig("contains", numberCase("a", "A", 1)),
			`switch case "a" cannot use number with comparator "contains"`,
		},
		{
			"text literal mismatch",
			switchConfig("equals", map[string]any{"id": "a", "label": "A", "valueType": "text", "value": 3}),
			`switch case "a" has invalid text value: must be text`,
		},
		{
			"number literal mismatch",
			switchConfig("equals", map[string]any{"id": "a", "label": "A", "valueType": "number", "value": "x"}),
			`switch case "a" has invalid number value: must be a finite number`,
		},
		{
			"boolean literal mismatch",
			switchConfig("equals", map[string]any{"id": "a", "label": "A", "valueType": "boolean", "value": "true"}),
			`switch case "a" has invalid boolean value: must be true or false`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module(t).Resolve(domain.FlowNode{ID: "switch-1", Type: "flow:switch", Data: map[string]any{"config": test.config}})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Resolve() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestExecuteEqualsRouting(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		selection any
		wantPort  string
		wantMatch bool
	}{
		{
			"text equals matches second case",
			switchConfig("equals", textCase("case-a", "Alpha", "a"), textCase("case-b", "Beta", "b")),
			"b", "case-b", true,
		},
		{
			"text equals falls through to default",
			switchConfig("equals", textCase("case-a", "Alpha", "a")),
			"z", "default", false,
		},
		{
			"number equals matches",
			switchConfig("equals", numberCase("case-a", "Alpha", 5)),
			float64(5), "case-a", true,
		},
		{
			"int input matches float literal",
			switchConfig("equals", numberCase("case-a", "Alpha", 5)),
			5, "case-a", true,
		},
		{
			"number equals misses on different value",
			switchConfig("equals", numberCase("case-a", "Alpha", 5)),
			float64(6), "default", false,
		},
		{
			"boolean equals matches",
			switchConfig("equals", booleanCase("case-on", "On", true), booleanCase("case-off", "Off", false)),
			true, "case-on", true,
		},
		{
			"boolean equals routes false case",
			switchConfig("equals", booleanCase("case-on", "On", true), booleanCase("case-off", "Off", false)),
			false, "case-off", true,
		},
		{
			"text case does not match number input",
			switchConfig("equals", textCase("case-a", "Alpha", "a")),
			float64(1), "default", false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, test.config, test.selection), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
			reported, ok := result.Outputs["result"].(map[string]any)
			if !ok {
				t.Fatalf("outputs = %#v", result.Outputs)
			}
			if reported["comparator"] != "equals" || reported["value"] != test.selection {
				t.Fatalf("result = %#v", reported)
			}
			matched, ok := reported["matchedCase"].(map[string]any)
			if test.wantMatch && (!ok || matched["id"] != test.wantPort) {
				t.Fatalf("matched case = %#v, want id %s", reported["matchedCase"], test.wantPort)
			}
			if !test.wantMatch && reported["matchedCase"] != nil {
				t.Fatalf("matched case = %#v, want none", reported["matchedCase"])
			}
		})
	}
}

func TestExecuteNotEqualsRoutesFirstNonMatchingCase(t *testing.T) {
	config := switchConfig("not_equals", numberCase("case-low", "Low", 1), numberCase("case-high", "High", 2))
	tests := []struct {
		name      string
		selection any
		wantPort  string
	}{
		{"input equal to first case skips to second", float64(1), "case-high"},
		{"input equal to second case stops at first", float64(2), "case-low"},
		{"input equal to neither stops at first", float64(3), "case-low"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, config, test.selection), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
		})
	}
}

func TestExecuteContainsRouting(t *testing.T) {
	config := switchConfig("contains", textCase("case-hello", "Hello", "ell"))
	tests := []struct {
		name      string
		selection any
		wantPort  string
	}{
		{"contained text matches", "hello", "case-hello"},
		{"unrelated text falls to default", "world", "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, config, test.selection), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
		})
	}
}

func TestExecuteContainsRejectsNonTextInput(t *testing.T) {
	registered := module(t)
	_, err := registered.Execute(context.Background(), invocation(registered,
		switchConfig("contains", textCase("case-hello", "Hello", "ell")),
		42,
	), nil)
	if err == nil || !strings.Contains(err.Error(), "switch contains comparator requires a Text value") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestExecuteGreaterThanRouting(t *testing.T) {
	config := switchConfig("greater_than", numberCase("case-big", "Big", 10), numberCase("case-huge", "Huge", 100))
	tests := []struct {
		name      string
		selection any
		wantPort  string
	}{
		{"above first threshold matches first case", float64(15), "case-big"},
		{"above both thresholds matches first case in order", float64(150), "case-big"},
		{"below all thresholds routes default", float64(5), "default"},
		{"equal threshold is not greater", float64(10), "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registered := module(t)
			result, err := registered.Execute(context.Background(), invocation(registered, config, test.selection), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(result.Ports) != 1 || result.Ports[0] != test.wantPort {
				t.Fatalf("ports = %#v, want [%s]", result.Ports, test.wantPort)
			}
		})
	}
}

func TestExecuteGreaterThanInclusiveBoundary(t *testing.T) {
	registered := module(t)
	config := switchConfig("greater_than_or_equal", numberCase("case-big", "Big", 10))
	result, err := registered.Execute(context.Background(), invocation(registered, config, float64(10)), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "case-big" {
		t.Fatalf("ports = %#v, want [case-big]", result.Ports)
	}
}

func TestExecuteGreaterThanRejectsNonNumberInput(t *testing.T) {
	registered := module(t)
	_, err := registered.Execute(context.Background(), invocation(registered,
		switchConfig("greater_than", numberCase("case-big", "Big", 10)),
		"15",
	), nil)
	if err == nil || !strings.Contains(err.Error(), "switch greater_than comparator requires a Number value") {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestExecuteUsesDefinitionDefaultsWithoutConfig(t *testing.T) {
	registered := module(t)
	result, err := registered.Execute(context.Background(), invocation(registered, map[string]any{}, "case-a"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "case-a" {
		t.Fatalf("ports = %#v, want [case-a]", result.Ports)
	}
}

func TestExecuteFailsWithoutAnyConfiguration(t *testing.T) {
	registered := module(t)
	invocation := nodes.Invocation{
		Node:            domain.FlowNode{ID: "switch-1", Type: "flow:switch", Data: map[string]any{}},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{},
		Inputs:          map[string]any{"selection": "a"},
		ConnectedInputs: map[string]bool{},
	}
	if _, err := registered.Execute(context.Background(), invocation, nil); err == nil {
		t.Fatal("Execute() accepted a definition without switch configuration")
	}
}
