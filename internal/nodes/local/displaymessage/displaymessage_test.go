package displaymessage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

type runtimeStub struct{ opener nodes.DialogOpener }

func (r runtimeStub) DialogOpener() nodes.DialogOpener { return r.opener }

type openerStub struct {
	title, message string
	err            error
}

func (o *openerStub) ShowMessage(_ context.Context, title, message string) error {
	o.title, o.message = title, message
	return o.err
}
func (o *openerStub) ShowQuestion(context.Context, string, string) (nodes.DialogChoice, error) {
	panic("unused")
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:display_message")
	if !ok {
		t.Fatal("action:display_message was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:display_message", Data: map[string]any{"config": config}},
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
	if definition.Type != "action:display_message" || definition.Mode != domain.NodeImpure || definition.Category != "Display" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "title", "message"})
	assertPinIDs(t, definition.Outputs, []string{"out", "result"})
}

func TestShowsMessageAndReportsDismissed(t *testing.T) {
	opener := &openerStub{}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"title": "Pipeline done", "message": "All steps finished",
	}), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["title"] != "Pipeline done" || output["message"] != "All steps finished" || output["dismissed"] != true {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if opener.title != "Pipeline done" || opener.message != "All steps finished" {
		t.Fatalf("dialog got %q / %q", opener.title, opener.message)
	}
}

// A connected pin beats the inspector field, which beats the built-in default.
func TestTextResolutionPrecedence(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	cases := []struct {
		name        string
		config      map[string]any
		inputs      map[string]any
		wantTitle   string
		wantMessage string
	}{
		{"defaults", map[string]any{}, nil, "Neuropipe", ""},
		{"config fallback", map[string]any{"title": "From config", "message": "Config body"}, nil, "From config", "Config body"},
		{"pin precedence", map[string]any{"title": "From config", "message": "Config body"}, map[string]any{"title": "From pin", "message": "Pin body"}, "From pin", "Pin body"},
		{"blank pin falls through", map[string]any{"message": "Config body"}, map[string]any{"message": "   "}, "Neuropipe", "Config body"},
	}
	for _, testCase := range cases {
		opener := &openerStub{}
		if _, err := module.Execute(context.Background(), invocation(definition, testCase.config, testCase.inputs), runtimeStub{opener: opener}); err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if opener.title != testCase.wantTitle || opener.message != testCase.wantMessage {
			t.Fatalf("%s: dialog got %q / %q, want %q / %q", testCase.name, opener.title, opener.message, testCase.wantTitle, testCase.wantMessage)
		}
	}
}

func TestShowMessageErrorPropagates(t *testing.T) {
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
		if err == nil || !strings.Contains(err.Error(), "native dialogs are unavailable") {
			t.Fatalf("runtime %v: err = %v", runtime, err)
		}
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := registeredModule(t)
	_, err := module.Execute(ctx, invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: &openerStub{}})
	if err == nil || !strings.Contains(err.Error(), "display message cancelled") {
		t.Fatalf("err = %v", err)
	}
}
