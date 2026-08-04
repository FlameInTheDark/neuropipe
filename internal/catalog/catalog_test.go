package catalog

import "testing"

func TestActionFieldsBecomeTypedPinsWithoutGenericInput(t *testing.T) {
	definition, ok := New().Get("action:notification")
	if !ok {
		t.Fatal("Desktop Notification definition is missing")
	}
	inputs := make(map[string]bool, len(definition.Inputs))
	for _, pin := range definition.Inputs {
		inputs[pin.ID] = true
	}
	if inputs["input"] {
		t.Fatal("Desktop Notification has an unexpected generic input pin")
	}
	for _, id := range []string{"in", "title", "message"} {
		if !inputs[id] {
			t.Fatalf("Desktop Notification is missing %q pin", id)
		}
	}
}

func TestStructuredConfigurationDoesNotBecomeDataPins(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"flow:switch", "llm:choice", "llm:extract"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		for _, pin := range definition.Inputs {
			if pin.ID == "options" || pin.ID == "schema" {
				t.Fatalf("%s exposes structured configuration %q as a data pin", nodeType, pin.ID)
			}
		}
	}
}

func TestKnownTerminalResultShapeIsPublished(t *testing.T) {
	definition, ok := New().Get("action:terminal")
	if !ok {
		t.Fatal("Run Terminal Command definition is missing")
	}
	for _, pin := range definition.Outputs {
		if pin.ID != "result" {
			continue
		}
		if len(pin.Fields) != 2 || pin.Fields[0].Path != "terminal.command" || pin.Fields[1].Path != "terminal.output" {
			t.Fatalf("terminal result fields = %#v", pin.Fields)
		}
		return
	}
	t.Fatal("Run Terminal Command result output is missing")
}

func TestHTTPRequestHeaderConfigurationDoesNotCreateDataPins(t *testing.T) {
	definition, ok := New().Get("action:http")
	if !ok {
		t.Fatal("HTTP Request definition is missing")
	}
	fields := make(map[string]bool, len(definition.Fields))
	for _, field := range definition.Fields {
		fields[field.Name] = true
	}
	for _, name := range []string{"headers", "useCustomUserAgent", "userAgent"} {
		if !fields[name] {
			t.Fatalf("HTTP Request is missing %q configuration", name)
		}
	}
	for _, pin := range definition.Inputs {
		if pin.ID == "headers" || pin.ID == "useCustomUserAgent" || pin.ID == "userAgent" {
			t.Fatalf("HTTP Request exposes configuration-only %q as a data pin", pin.ID)
		}
	}
}

func TestChatTriggerHasOnlyItsExplicitBlueprintPins(t *testing.T) {
	definition, ok := New().Get("trigger:chat")
	if !ok {
		t.Fatal("Chat Trigger definition is missing")
	}
	pins := make(map[string]bool, len(definition.Outputs))
	for _, pin := range definition.Outputs {
		pins[pin.ID] = true
	}
	for _, id := range []string{"out", "text", "chatId", "chatRunId"} {
		if !pins[id] {
			t.Fatalf("Chat Trigger is missing %q", id)
		}
	}
	if pins["payload"] || len(pins) != 4 {
		t.Fatalf("Chat Trigger outputs = %#v, want only its explicit contract", definition.Outputs)
	}
}

func TestObjectNodesExposeOnlyTheirBlueprintContracts(t *testing.T) {
	registry := New()
	build, ok := registry.Get("data:build_object")
	if !ok {
		t.Fatal("Build Object definition is missing")
	}
	if len(build.Fields) != 1 || build.Fields[0].Kind != "object-fields" {
		t.Fatalf("Build Object configuration = %#v, want configurable object fields", build.Fields)
	}
	breakObject, ok := registry.Get("data:break_object")
	if !ok {
		t.Fatal("Break Object definition is missing")
	}
	if len(breakObject.Inputs) != 1 || breakObject.Inputs[0].ID != "source" || breakObject.Inputs[0].DataType != "object" {
		t.Fatalf("Break Object inputs = %#v", breakObject.Inputs)
	}
	if len(breakObject.Fields) != 1 || breakObject.Fields[0].Kind != "field-outputs" {
		t.Fatalf("Break Object configuration = %#v, want configurable outputs", breakObject.Fields)
	}
}
