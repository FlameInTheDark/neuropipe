package displayquestion

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
	choice         nodes.DialogChoice
	err            error
}

func (o *openerStub) ShowMessage(context.Context, string, string) error {
	panic("unused")
}
func (o *openerStub) ShowQuestion(_ context.Context, title, message string) (nodes.DialogChoice, error) {
	o.title, o.message = title, message
	return o.choice, o.err
}

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:display_question")
	if !ok {
		t.Fatal("action:display_question was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:display_question", Data: map[string]any{"config": config}},
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
	if definition.Type != "action:display_question" || definition.Mode != domain.NodeImpure || definition.Category != "Display" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "title", "message"})
	assertPinIDs(t, definition.Outputs, []string{"yes", "no", "result"})
}

func TestYesChoiceRoutesYesPort(t *testing.T) {
	opener := &openerStub{choice: nodes.DialogYes}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"title": "Deploy", "message": "Continue?",
	}), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "yes" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["choice"] != "yes" || output["title"] != "Deploy" || output["message"] != "Continue?" {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if opener.title != "Deploy" || opener.message != "Continue?" {
		t.Fatalf("dialog got %q / %q", opener.title, opener.message)
	}
}

func TestNoChoiceRoutesNoPort(t *testing.T) {
	opener := &openerStub{choice: nodes.DialogNo}
	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, nil), runtimeStub{opener: opener})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "no" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["choice"] != "no" {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
}

// Cancellation or an unknown selection routes to No so the graph still
// terminates a downstream branch; the reported choice is coerced to "no".
func TestCancelAndUnknownChoicesFallBackToNoPort(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	for _, choice := range []nodes.DialogChoice{nodes.DialogCancel, "something-else"} {
		opener := &openerStub{choice: choice}
		result, err := module.Execute(context.Background(), invocation(definition, map[string]any{}, nil), runtimeStub{opener: opener})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Ports) != 1 || result.Ports[0] != "no" {
			t.Fatalf("choice %q: ports = %#v", choice, result.Ports)
		}
		output, ok := result.Outputs["result"].(map[string]any)
		if !ok || output["choice"] != "no" {
			t.Fatalf("choice %q: result output = %#v", choice, result.Outputs["result"])
		}
	}
}

func TestShowQuestionErrorPropagates(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "display question cancelled") {
		t.Fatalf("err = %v", err)
	}
}
