package javascript

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestNodeResolvesTypedDynamicPortsAndCapabilities(t *testing.T) {
	module := New()
	definition, err := module.Resolve(flowNode(configFor(
		"return { total: count + 1 };",
		[]any{pin("count", "Count", domain.TypeSpec{Kind: domain.TypeInt}, true)},
		[]any{pin("total", "Total", domain.TypeSpec{Kind: domain.TypeInt}, true)},
		[]any{string(domain.CapabilityNetwork), string(domain.CapabilityFileRead)},
	)))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	if got, want := definition.Type, "action:javascript"; got != want {
		t.Fatalf("definition type = %q, want %q", got, want)
	}
	if got, want := len(definition.Inputs), 3; got != want || definition.Inputs[1].ID != "code" || definition.Inputs[1].Type.Kind != domain.TypeString || definition.Inputs[2].ID != "count" || definition.Inputs[2].Type.Kind != domain.TypeInt {
		t.Fatalf("unexpected resolved inputs: %#v", definition.Inputs)
	}
	if got, want := len(definition.Outputs), 2; got != want || definition.Outputs[1].ID != "total" || definition.Outputs[1].Type.Kind != domain.TypeInt {
		t.Fatalf("unexpected resolved outputs: %#v", definition.Outputs)
	}
	if got, want := definition.Capabilities, []domain.Capability{domain.CapabilityFileRead, domain.CapabilityNetwork}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func TestNodeExecutesWithExactContracts(t *testing.T) {
	module := New()
	record := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "label", Name: "label", Type: domain.TypeSpec{Kind: domain.TypeString}},
		{ID: "active", Name: "active", Type: domain.TypeSpec{Kind: domain.TypeBool}, Optional: true},
	}}
	config := configFor(
		"return { total: count + 1, summary: { label: name, active: true } };",
		[]any{
			pin("count", "Count", domain.TypeSpec{Kind: domain.TypeInt}, true),
			pin("name", "Name", domain.TypeSpec{Kind: domain.TypeString}, true),
		},
		[]any{
			pin("total", "Total", domain.TypeSpec{Kind: domain.TypeInt}, true),
			pin("summary", "Summary", record, true),
		},
		nil,
	)
	definition, err := module.Resolve(flowNode(config))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Node:       flowNode(config),
		Definition: definition,
		Config:     config,
		Inputs:     map[string]any{"count": int64(4), "name": "Ada"},
	}, struct{}{})
	if err != nil {
		t.Fatalf("execute JavaScript node: %v", err)
	}
	if got, want := result.Ports, []string{"out"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("ports = %#v, want %#v", got, want)
	}
	if got := result.Outputs["total"]; got != int64(5) {
		t.Fatalf("total = %#v (%T), want int64(5)", got, got)
	}
	summary, ok := result.Outputs["summary"].(map[string]any)
	if !ok || summary["label"] != "Ada" || summary["active"] != true {
		t.Fatalf("summary = %#v", result.Outputs["summary"])
	}
}

func TestNodeRejectsInvalidContractsAndOutputValues(t *testing.T) {
	module := New()
	_, err := module.Resolve(flowNode(configFor("return {};", []any{pin("np", "Bad", domain.TypeSpec{Kind: domain.TypeString}, false)}, nil, nil)))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved pin error = %v", err)
	}

	config := configFor("return { total: 1.5 };", nil, []any{pin("total", "Total", domain.TypeSpec{Kind: domain.TypeInt}, true)}, nil)
	definition, err := module.Resolve(flowNode(config))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	_, err = module.Execute(context.Background(), nodes.Invocation{Node: flowNode(config), Definition: definition, Config: config, Inputs: map[string]any{}}, struct{}{})
	if err == nil || !strings.Contains(err.Error(), "safe integer") {
		t.Fatalf("invalid output error = %v", err)
	}
}

func TestNodeProvidesScopedVariablesButGuardsFilesystem(t *testing.T) {
	module := New()
	config := configFor(
		`np.variables.set("saved", name); return { copy: np.variables.get("saved") };`,
		[]any{pin("name", "Name", domain.TypeSpec{Kind: domain.TypeString}, true)},
		[]any{pin("copy", "Copy", domain.TypeSpec{Kind: domain.TypeString}, true)},
		nil,
	)
	definition, err := module.Resolve(flowNode(config))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	runtime := &testRuntime{variables: map[string]any{}}
	result, err := module.Execute(context.Background(), nodes.Invocation{Node: flowNode(config), Definition: definition, Config: config, Inputs: map[string]any{"name": "Ada"}}, runtime)
	if err != nil {
		t.Fatalf("execute JavaScript variables: %v", err)
	}
	if got := result.Outputs["copy"]; got != "Ada" || runtime.variables["saved"] != "Ada" {
		t.Fatalf("variables result = %#v, stored = %#v", got, runtime.variables)
	}

	blocked := configFor("np.files.readText('C:/private.txt'); return {};", nil, nil, nil)
	blockedDefinition, err := module.Resolve(flowNode(blocked))
	if err != nil {
		t.Fatalf("resolve guarded JavaScript node: %v", err)
	}
	_, err = module.Execute(context.Background(), nodes.Invocation{Node: flowNode(blocked), Definition: blockedDefinition, Config: blocked, Inputs: map[string]any{}}, runtime)
	if err == nil || !strings.Contains(err.Error(), "file-read") {
		t.Fatalf("filesystem without capability error = %v", err)
	}
}

func TestValidateRejectsSyntax(t *testing.T) {
	if err := Validate("return ("); err == nil {
		t.Fatal("Validate accepted incomplete JavaScript")
	}
}

// When the Code input pin is connected, the wired value must override the
// editor-configured source so pipelines can supply JavaScript dynamically.
func TestCodeInputPinOverridesEditor(t *testing.T) {
	module := New()
	editorConfig := configFor(
		"return { message: 'from editor' };",
		nil,
		[]any{pin("message", "Message", domain.TypeSpec{Kind: domain.TypeString}, true)},
		nil,
	)
	definition, err := module.Resolve(flowNode(editorConfig))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	wired := "return { message: 'from wire' };"
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Node:            flowNode(editorConfig),
		Definition:      definition,
		Config:          editorConfig,
		Inputs:          map[string]any{codeInputPinID: wired},
		ConnectedInputs: map[string]bool{codeInputPinID: true},
	}, struct{}{})
	if err != nil {
		t.Fatalf("execute JavaScript node with wired code: %v", err)
	}
	if got := result.Outputs["message"]; got != "from wire" {
		t.Fatalf("wired code did not override editor; got %#v, want %q", got, "from wire")
	}
}

// A reserved Code identifier cannot be reused for a dynamic input or output.
func TestCodeIdentifierIsReserved(t *testing.T) {
	module := New()
	_, err := module.Resolve(flowNode(configFor(
		"return {};",
		[]any{pin("code", "Code", domain.TypeSpec{Kind: domain.TypeString}, false)},
		nil,
		nil,
	)))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved 'code' pin error = %v", err)
	}
}

func configFor(code string, inputs, outputs, capabilities []any) map[string]any {
	if inputs == nil {
		inputs = []any{}
	}
	if outputs == nil {
		outputs = []any{}
	}
	if capabilities == nil {
		capabilities = []any{}
	}
	return map[string]any{
		codeConfigKey:         code,
		inputsConfigKey:       inputs,
		outputsConfigKey:      outputs,
		capabilitiesConfigKey: capabilities,
	}
}

func pin(id, label string, typeSpec domain.TypeSpec, required bool) map[string]any {
	return map[string]any{"id": id, "label": label, "type": typeSpec, "required": required}
}

func flowNode(config map[string]any) domain.FlowNode {
	return domain.FlowNode{ID: "javascript", Type: "action:javascript", Data: map[string]any{"config": config}}
}

type testRuntime struct{ variables map[string]any }

func (runtime *testRuntime) LookupVariable(name string) (any, bool) {
	value, exists := runtime.variables[name]
	return value, exists
}

func (runtime *testRuntime) StoreVariable(name string, value any) { runtime.variables[name] = value }
func (runtime *testRuntime) DeleteVariable(name string)           { delete(runtime.variables, name) }

func TestNodeResolvesBytesContractAsBytesPin(t *testing.T) {
	module := New()
	definition, err := module.Resolve(flowNode(configFor(
		"return { picture };",
		[]any{pin("picture", "Picture", domain.TypeSpec{Kind: domain.TypeBytes}, true)},
		[]any{pin("blob", "Blob", domain.TypeSpec{Kind: domain.TypeBytes}, false)},
		nil,
	)))
	if err != nil {
		t.Fatalf("resolve JavaScript node: %v", err)
	}
	var input, output *domain.NodePort
	for index := range definition.Inputs {
		if definition.Inputs[index].ID == "picture" {
			input = &definition.Inputs[index]
		}
	}
	for index := range definition.Outputs {
		if definition.Outputs[index].ID == "blob" {
			output = &definition.Outputs[index]
		}
	}
	if input == nil || output == nil {
		t.Fatalf("bytes pins missing: %#v / %#v", definition.Inputs, definition.Outputs)
	}
	if input.DataType != domain.DataBytes || input.Type == nil || input.Type.Kind != domain.TypeBytes {
		t.Fatalf("picture pin = %#v, want dataType bytes", input)
	}
	if output.DataType != domain.DataBytes || output.Type == nil || output.Type.Kind != domain.TypeBytes {
		t.Fatalf("blob pin = %#v, want dataType bytes", output)
	}
}
