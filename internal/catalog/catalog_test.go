package catalog

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

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

func TestJavaScriptExposesCodeInputPinAndEditorField(t *testing.T) {
	definition, ok := New().Get("action:javascript")
	if !ok {
		t.Fatal("JavaScript definition is missing")
	}
	if _, ok := New().Node("action:javascript"); !ok {
		t.Fatal("JavaScript module is missing")
	}
	var codePin *domain.NodePort
	for index := range definition.Inputs {
		if definition.Inputs[index].ID == "code" {
			codePin = &definition.Inputs[index]
			break
		}
	}
	if codePin == nil {
		t.Fatalf("JavaScript is missing the Code input pin: %#v", definition.Inputs)
	}
	if codePin.Kind != domain.PinData || codePin.Direction != domain.PinInput {
		t.Fatalf("JavaScript Code pin is not a data input: %#v", codePin)
	}
	if codePin.Type == nil || codePin.Type.Kind != domain.TypeString {
		t.Fatalf("JavaScript Code pin must be a string: %#v", codePin)
	}
	if !codePin.IgnoreConfigFallback {
		t.Fatalf("JavaScript Code pin must opt out of inspector config fallback so the editor value is read explicitly")
	}
	if len(definition.Fields) != 1 || definition.Fields[0].Kind != "javascript-editor" {
		t.Fatalf("JavaScript fields = %#v", definition.Fields)
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
	// The explicit Headers pin (map<string,string>) backs the
	// take-headers-from-pin toggle; the remaining config fields must stay
	// configuration-only.
	for _, pin := range definition.Inputs {
		switch pin.ID {
		case "headers":
			if pin.Kind != domain.PinData || pin.DataType != domain.DataObject || pin.MaxConnections != 1 {
				t.Fatalf("Headers pin = %#v, want a single-wire object pin", pin)
			}
		case "useCustomUserAgent", "userAgent":
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

func TestButtonTriggerHasOneStructuredPayload(t *testing.T) {
	definition, ok := New().Get("trigger:button")
	if !ok {
		t.Fatal("Button Trigger definition is missing")
	}
	payloads := make([]domain.NodePort, 0, 1)
	for _, output := range definition.Outputs {
		if output.ID == "payload" && output.Kind == domain.PinData {
			payloads = append(payloads, output)
		}
	}
	if len(payloads) != 1 {
		t.Fatalf("Button Trigger payload outputs = %#v, want exactly one", payloads)
	}
	payload := payloads[0]
	if payload.Type == nil || payload.Type.Kind != domain.TypeRecord || len(payload.Type.Fields) != 1 {
		t.Fatalf("Button Trigger payload type = %#v, want one-field record", payload.Type)
	}
	field := payload.Type.Fields[0]
	if field.ID != "trigger" || field.Name != "trigger" || field.Type.Kind != domain.TypeString {
		t.Fatalf("Button Trigger payload field = %#v, want trigger string", field)
	}
	if len(payload.Fields) != 1 || payload.Fields[0].Path != "trigger" || payload.Fields[0].DataType != domain.DataText {
		t.Fatalf("Button Trigger payload display fields = %#v, want trigger text", payload.Fields)
	}
}

func TestAgentToolPinsAcceptMultipleToolFunctions(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"llm:agent", "llm:coding_agent"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		found := false
		for _, input := range definition.Inputs {
			if input.ID != "tools" {
				continue
			}
			found = true
			if input.Kind != domain.PinTool || input.Direction != domain.PinInput || input.MaxConnections != 0 {
				t.Fatalf("%s Tools input = %#v, want unlimited tool input", nodeType, input)
			}
			break
		}
		if !found {
			t.Fatalf("%s has no Tools input", nodeType)
		}
	}
}

func TestLocalFileModulesPublishStrictContracts(t *testing.T) {
	registry := New()
	list, ok := registry.Get("action:list_directory")
	if !ok {
		t.Fatal("List Directory definition is missing")
	}
	if list.Mode != domain.NodeImpure || len(list.Capabilities) != 1 || list.Capabilities[0] != domain.CapabilityFileRead {
		t.Fatalf("List Directory definition = %#v", list)
	}
	var files domain.NodePort
	for _, output := range list.Outputs {
		if output.ID == "result" {
			files = output
		}
	}
	if files.Label != "Files" || files.Type == nil || files.Type.Kind != domain.TypeList || files.Type.Element == nil || files.Type.Element.Kind != domain.TypeRecord {
		t.Fatalf("List Directory Files output = %#v", files)
	}

	read, ok := registry.Get("action:file_read")
	if !ok {
		t.Fatal("Read File definition is missing")
	}
	outputs := make(map[string]domain.NodePort, len(read.Outputs))
	for _, output := range read.Outputs {
		outputs[output.ID] = output
	}
	if outputs["result"].Type == nil || outputs["result"].Type.Kind != domain.TypeBytes {
		t.Fatalf("Read File outputs = %#v", outputs)
	}
	write, ok := registry.Get("action:file_write")
	if !ok {
		t.Fatal("Write File definition is missing")
	}
	if len(write.Inputs) != 3 || write.Inputs[2].Type == nil || write.Inputs[2].Type.Kind != domain.TypeString {
		t.Fatalf("Write File inputs = %#v", write.Inputs)
	}

	for _, nodeType := range []string{"data:base64_encode", "data:base64_decode"} {
		if _, ok := registry.Node(nodeType); !ok {
			t.Fatalf("%s module is missing", nodeType)
		}
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

func TestAgentChatModeAndTurnControlsStayConfigurationScoped(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"llm:agent", "llm:coding_agent"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		modeFound := false
		unlimitedFound := false
		for _, configField := range definition.Fields {
			if configField.Name == "chatMode" {
				modeFound = true
				if len(configField.Options) != 2 {
					t.Fatalf("%s chatMode options = %#v, want message and history", nodeType, configField.Options)
				}
			}
			if configField.Name == "unlimitedTurns" {
				unlimitedFound = true
			}
		}
		if !modeFound || !unlimitedFound {
			t.Fatalf("%s fields miss chat mode or unlimited turns", nodeType)
		}
		chatID := false
		for _, input := range definition.Inputs {
			if input.ID == "chatMode" || input.ID == "unlimitedTurns" {
				t.Fatalf("%s leaked a mode control as a data pin", nodeType)
			}
			if input.ID == "chatId" {
				chatID = true
				if input.Required || input.DataType != domain.DataText {
					t.Fatalf("%s chatId pin = %#v, want optional Text input", nodeType, input)
				}
			}
		}
		if !chatID {
			t.Fatalf("%s has no chatId input pin", nodeType)
		}
		if mode, _ := definition.DefaultConfig["chatMode"].(string); mode != "message" {
			t.Fatalf("%s must default to one-message mode", nodeType)
		}
		if unlimited, _ := definition.DefaultConfig["unlimitedTurns"].(bool); unlimited {
			t.Fatalf("%s must default to a capped turn budget", nodeType)
		}
	}
}

func TestLLMChatStatusToggleStaysConfigurationScoped(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"llm:prompt", "llm:extract", "llm:boolean", "llm:choice", "llm:summarize", "llm:agent", "llm:coding_agent"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		toggleFound := false
		for _, configField := range definition.Fields {
			if configField.Name == "updateChatStatus" {
				toggleFound = true
			}
		}
		if !toggleFound {
			t.Fatalf("%s has no chat status toggle", nodeType)
		}
		chatRunID := false
		for _, input := range definition.Inputs {
			if input.ID == "updateChatStatus" {
				t.Fatalf("%s leaked the toggle as a data pin", nodeType)
			}
			if input.ID == "chatRunId" {
				chatRunID = true
				if input.Required || input.DataType != domain.DataText {
					t.Fatalf("%s chatRunId pin = %#v, want optional Text input", nodeType, input)
				}
			}
		}
		if !chatRunID {
			t.Fatalf("%s has no chatRunId input pin", nodeType)
		}
		if enabled, _ := definition.DefaultConfig["updateChatStatus"].(bool); enabled {
			t.Fatalf("%s must default to silent status updates", nodeType)
		}
	}
}

func TestLLMNodesExposeProviderAndModelSelection(t *testing.T) {
	registry := New()
	for _, nodeType := range []string{"llm:prompt", "llm:extract", "llm:boolean", "llm:choice", "llm:summarize", "llm:agent", "llm:coding_agent"} {
		definition, ok := registry.Get(nodeType)
		if !ok {
			t.Fatalf("%s definition is missing", nodeType)
		}
		providerField := false
		modelField := false
		for _, configField := range definition.Fields {
			if configField.Name == "providerId" {
				providerField = true
				if configField.Kind != "llm-provider" {
					t.Fatalf("%s providerId kind = %q, want llm-provider", nodeType, configField.Kind)
				}
			}
			if configField.Name == "model" {
				modelField = true
				if configField.Kind != "llm-model" {
					t.Fatalf("%s model kind = %q, want llm-model", nodeType, configField.Kind)
				}
			}
		}
		if !providerField || !modelField {
			t.Fatalf("%s is missing the provider or model selection field", nodeType)
		}
		modelPin := false
		for _, input := range definition.Inputs {
			if input.ID == "providerId" {
				t.Fatalf("%s leaked provider selection as a data pin", nodeType)
			}
			if input.ID == "model" {
				modelPin = true
				if input.Required || input.DataType != domain.DataText {
					t.Fatalf("%s model pin = %#v, want optional Text input", nodeType, input)
				}
			}
		}
		// The model pin stays available so legacy graphs with a wired model
		// override keep validating, and dynamic model routing keeps working.
		if !modelPin {
			t.Fatalf("%s lost its model input pin", nodeType)
		}
		if value, exists := definition.DefaultConfig["providerId"]; !exists || value != "" {
			t.Fatalf("%s default providerId = %#v, want an empty default that follows the app default provider", nodeType, value)
		}
		if value, exists := definition.DefaultConfig["model"]; !exists || value != "" {
			t.Fatalf("%s default model = %#v, want an empty default that follows the provider default model", nodeType, value)
		}
	}
}
