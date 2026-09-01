package displayinput

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ opener nodes.InputDialogOpener }

func (r runtimeStub) InputDialogOpener() nodes.InputDialogOpener { return r.opener }

type openerStub struct {
	request  nodes.InputRequest
	response nodes.InputResponse
	err      error
}

func (o *openerStub) ShowInput(_ context.Context, request nodes.InputRequest) (nodes.InputResponse, error) {
	o.request = request
	return o.response, o.err
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:display_input")
	if !ok {
		t.Fatal("action:display_input was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:display_input", Data: map[string]any{"config": config}},
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

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:display_input" || definition.Mode != domain.NodeImpure || definition.Category != "Display" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "title", "message", "label"})
	assertPinIDs(t, definition.Outputs, []string{"continue", "canceled", "value", "result"})
}

func TestContinueEmitsTextValue(t *testing.T) {
	opener := &openerStub{response: nodes.InputResponse{Value: "Osaka"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{"type": "text"}, map[string]any{
		"title": "City", "message": "Enter a city:", "label": "Name",
	}), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "continue" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if result.Outputs["value"] != "Osaka" {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["title"] != "City" || output["message"] != "Enter a city:" || output["value"] != "Osaka" || output["canceled"] != false {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if opener.request.Title != "City" || opener.request.Message != "Enter a city:" || opener.request.Label != "Name" {
		t.Fatalf("request = %#v", opener.request)
	}
	if opener.request.InputType != "text" || opener.request.Continue != "Continue" || opener.request.Cancel != "Cancel" || opener.request.Placeholder != "" {
		t.Fatalf("request = %#v", opener.request)
	}
}

func TestNumberInputParsesFloatAndAdaptsRequest(t *testing.T) {
	opener := &openerStub{response: nodes.InputResponse{Value: "3.5"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{"type": "number"}, nil), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["value"] != float64(3.5) {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	if opener.request.InputType != "number" || opener.request.Placeholder != "0" {
		t.Fatalf("request = %#v", opener.request)
	}
}

func TestCancelRoutesToCanceledPortWithNilValue(t *testing.T) {
	opener := &openerStub{response: nodes.InputResponse{Canceled: true, Value: "ignored"}}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "canceled" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	if value, present := result.Outputs["value"]; present && value != nil {
		t.Fatalf("value = %#v, want nil", value)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["canceled"] != true || output["value"] != nil {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
}

func TestNumberValidationErrors(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	for _, raw := range []string{"", "   ", "not-a-number"} {
		opener := &openerStub{response: nodes.InputResponse{Value: raw}}
		_, err := module.Execute(context.Background(), invocation(definition, map[string]any{"type": "number"}, nil), runtimeStub{opener: opener})
		if err == nil || !strings.Contains(err.Error(), "number input") {
			t.Fatalf("value %q: err = %v", raw, err)
		}
	}
}

// A connected pin beats the inspector field, which beats the built-in default.
func TestTextResolutionPrecedence(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name      string
		config    map[string]any
		inputs    map[string]any
		wantTitle string
		wantLabel string
	}{
		{"defaults", map[string]any{}, nil, "Neuropipe", "Value"},
		{"config fallback", map[string]any{"title": "From config", "label": "Config label"}, nil, "From config", "Config label"},
		{"pin precedence", map[string]any{"title": "From config", "label": "Config label"}, map[string]any{"title": "From pin", "label": "Pin label"}, "From pin", "Pin label"},
		{"blank pin falls through", map[string]any{"title": "From config"}, map[string]any{"title": "   "}, "From config", "Value"},
	}
	for _, testCase := range cases {
		opener := &openerStub{response: nodes.InputResponse{Value: "x"}}
		if _, err := module.Execute(context.Background(), invocation(definition, testCase.config, testCase.inputs), runtimeStub{opener: opener}); err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if opener.request.Title != testCase.wantTitle || opener.request.Label != testCase.wantLabel {
			t.Fatalf("%s: request = %#v, want title %q label %q", testCase.name, opener.request, testCase.wantTitle, testCase.wantLabel)
		}
	}
}

func TestShowInputErrorPropagates(t *testing.T) {
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: &openerStub{err: errors.New("dialog crashed")}})
	if err == nil || !strings.Contains(err.Error(), "dialog crashed") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnavailableRuntime(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	for _, runtime := range []nodes.Runtime{nil, struct{}{}, runtimeStub{}} {
		_, err := module.Execute(context.Background(), invocation(definition, map[string]any{}, nil), runtime)
		if err == nil || !strings.Contains(err.Error(), "input dialogs are unavailable") {
			t.Fatalf("runtime %v: err = %v", runtime, err)
		}
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := registeredModule(t)
	_, err := module.Execute(ctx, invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: &openerStub{}})
	if err == nil || !strings.Contains(err.Error(), "display input cancelled") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveAdaptsValuePinToConfiguredType(t *testing.T) {
	module := registeredModule(t)
	definition, err := module.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{"type": "number"}}})
	if err != nil {
		t.Fatal(err)
	}
	if definition.Outputs[2].ID != "value" || definition.Outputs[2].DataType != domain.DataNumber {
		t.Fatalf("value pin = %#v", definition.Outputs[2])
	}
	definition, err = module.Resolve(domain.FlowNode{Data: map[string]any{"config": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if definition.Outputs[2].ID != "value" || definition.Outputs[2].DataType != domain.DataAny {
		t.Fatalf("value pin = %#v", definition.Outputs[2])
	}
}
