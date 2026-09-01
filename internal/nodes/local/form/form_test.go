package form

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ opener nodes.FormDialogOpener }

func (r runtimeStub) FormDialogOpener() nodes.FormDialogOpener { return r.opener }

type openerStub struct {
	request  nodes.FormRequest
	response nodes.FormResponse
	err      error
}

func (o *openerStub) ShowForm(_ context.Context, request nodes.FormRequest) (nodes.FormResponse, error) {
	o.request = request
	return o.response, o.err
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:form")
	if !ok {
		t.Fatal("action:form was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:form", Data: map[string]any{"config": config}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func assertPinIDs(t *testing.T, ports []domain.NodePort, want []string) {
	t.Helper()
	got := make([]string, 0, len(ports))
	for _, port := range ports {
		got = append(got, port.ID)
	}
	if len(got) != len(want) {
		t.Fatalf("pin ids = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", got, want)
		}
	}
}

// formLayout builds a config with a text panel, a text input, a number
// input, and a dropdown, in that order.
func formLayout() map[string]any {
	return map[string]any{"form": map[string]any{"items": []any{
		map[string]any{"id": "intro", "kind": "text", "label": "Intro"},
		map[string]any{"id": "name", "kind": "input", "label": "Name", "col": 0, "row": 0, "span": 2, "inputType": "text"},
		map[string]any{"id": "amount", "kind": "input", "label": "Amount", "col": 2, "row": 0, "span": 2, "inputType": "number"},
		map[string]any{"id": "color", "kind": "dropdown", "label": "Color", "options": []any{
			map[string]any{"value": "red", "label": "Red"},
			map[string]any{"value": "blue"},
		}},
	}}}
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:form" || definition.Mode != domain.NodeImpure || definition.Category != "Display" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "title", "message"})
	assertPinIDs(t, definition.Outputs, []string{"submit", "canceled", "result"})
}

// Text panels produce no pin; inputs and dropdowns become typed output pins
// prepended before the static exec and result pins.
func TestResolveAddsTypedFieldPins(t *testing.T) {
	module := registeredModule(t)
	definition, err := module.Resolve(domain.FlowNode{Data: map[string]any{"config": formLayout()}})
	if err != nil {
		t.Fatal(err)
	}
	assertPinIDs(t, definition.Outputs, []string{"name", "amount", "color", "submit", "canceled", "result"})
	pins := map[string]domain.NodePort{}
	for _, port := range definition.Outputs {
		pins[port.ID] = port
	}
	if pins["amount"].DataType != domain.DataNumber {
		t.Fatalf("number input pin = %#v", pins["amount"])
	}
	if pins["name"].DataType != domain.DataText || pins["color"].DataType != domain.DataText {
		t.Fatalf("text pins = %#v %#v", pins["name"], pins["color"])
	}
}

func TestSubmitEmitsOneOutputPerField(t *testing.T) {
	opener := &openerStub{response: nodes.FormResponse{Values: map[string]any{
		"name": "Ada", "amount": 3, "color": "red",
	}}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), formLayout(), map[string]any{
		"title": "Deploy", "message": "Fill the form",
	}), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "submit" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["name"] != "Ada" || result.Outputs["amount"] != 3 || result.Outputs["color"] != "red" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["title"] != "Deploy" || output["canceled"] != false {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if values, ok := output["values"].(map[string]any); !ok || values["name"] != "Ada" {
		t.Fatalf("values = %#v", output["values"])
	}

	request := opener.request
	if request.Title != "Deploy" || request.Message != "Fill the form" || request.Continue != "Submit" || request.Cancel != "Cancel" {
		t.Fatalf("request = %#v", request)
	}
	if len(request.Items) != 4 {
		t.Fatalf("items = %#v", request.Items)
	}
	text, input, number, dropdown := request.Items[0], request.Items[1], request.Items[2], request.Items[3]
	if text.ID != "intro" || text.Kind != "text" {
		t.Fatalf("text item = %#v", text)
	}
	if input.ID != "name" || input.Kind != "input" || input.InputType != "text" || input.Span != 2 || input.RowSpan != 1 {
		t.Fatalf("text input item = %#v", input)
	}
	if number.ID != "amount" || number.InputType != "number" {
		t.Fatalf("number input item = %#v", number)
	}
	if len(dropdown.Options) != 2 || dropdown.Options[0].Value != "red" || dropdown.Options[0].Label != "Red" || dropdown.Options[1].Value != "blue" || dropdown.Options[1].Label != "" {
		t.Fatalf("dropdown item = %#v", dropdown)
	}
}

func TestCancelRoutesToCanceledPortWithoutFieldOutputs(t *testing.T) {
	opener := &openerStub{response: nodes.FormResponse{Canceled: true, Values: map[string]any{"name": "ignored"}}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), formLayout(), nil), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "canceled" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["canceled"] != true {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
}

// Graphs saved before a form layout was configured fall back to the
// definition's default single-input layout.
func TestDefaultLayoutWhenFormConfigOmitted(t *testing.T) {
	opener := &openerStub{response: nodes.FormResponse{Values: map[string]any{"field_1": "x"}}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["field_1"] != "x" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if len(opener.request.Items) != 1 || opener.request.Items[0].ID != "field_1" || opener.request.Items[0].Kind != "input" || opener.request.Items[0].InputType != "text" {
		t.Fatalf("items = %#v", opener.request.Items)
	}
}

func TestLayoutValidationErrors(t *testing.T) {
	module := registeredModule(t)
	input := func(item any) map[string]any {
		return map[string]any{"form": map[string]any{"items": []any{item}}}
	}
	cases := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{"item is not an object", map[string]any{"form": map[string]any{"items": []any{"nope"}}}, "form item 0 is not an object"},
		{"item without id", input(map[string]any{"kind": "input"}), "form item 0 has no id"},
		{"unknown kind", input(map[string]any{"id": "x", "kind": "checkbox"}), `form item "x" has unknown kind "checkbox"`},
		{"unknown input type", input(map[string]any{"id": "n", "kind": "input", "inputType": "email"}), `form item "n" has unknown inputType "email"`},
		{"dropdown without options", input(map[string]any{"id": "d", "kind": "dropdown"}), `form item "d" (dropdown) needs at least one option`},
		{"duplicate ids", map[string]any{"form": map[string]any{"items": []any{
			map[string]any{"id": "x", "kind": "input"},
			map[string]any{"id": "x", "kind": "input"},
		}}}, `duplicate form item id "x"`},
	}
	for _, testCase := range cases {
		_, err := module.Resolve(domain.FlowNode{Data: map[string]any{"config": testCase.config}})
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

// Layout errors must also abort execution instead of silently showing a
// broken form.
func TestLayoutErrorPropagatesFromExecute(t *testing.T) {
	module := registeredModule(t)
	config := map[string]any{"form": map[string]any{"items": []any{
		map[string]any{"id": "x", "kind": "input"},
		map[string]any{"id": "x", "kind": "input"},
	}}}
	_, err := module.Execute(context.Background(), invocation(module.Definition(), config, nil), runtimeStub{opener: &openerStub{}})
	if err == nil || !strings.Contains(err.Error(), "duplicate form item id") {
		t.Fatalf("err = %v", err)
	}
}

func TestShowFormErrorPropagates(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), formLayout(), nil), runtimeStub{opener: &openerStub{err: errors.New("window closed")}})
	if err == nil || !strings.Contains(err.Error(), "window closed") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnavailableRuntime(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	for _, runtime := range []nodes.Runtime{nil, struct{}{}, runtimeStub{}} {
		_, err := module.Execute(context.Background(), invocation(definition, map[string]any{}, nil), runtime)
		if err == nil || !strings.Contains(err.Error(), "form dialogs are unavailable") {
			t.Fatalf("runtime %v: err = %v", runtime, err)
		}
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := registeredModule(t)
	_, err := module.Execute(ctx, invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: &openerStub{}})
	if err == nil || !strings.Contains(err.Error(), "form cancelled") {
		t.Fatalf("err = %v", err)
	}
}
