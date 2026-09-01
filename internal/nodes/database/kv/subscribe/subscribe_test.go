package subscribe

import (
	"context"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestRegisterExposesTrigger(t *testing.T) {
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	module, ok := registry.Get("kv:subscribe")
	if !ok {
		t.Fatal("kv:subscribe was not registered")
	}
	definition := module.Definition()
	if definition.Type != "kv:subscribe" || definition.Category != "KV Store" {
		t.Fatalf("definition = %#v", definition)
	}
	if definition.Mode != domain.NodeEvent {
		t.Fatalf("mode = %v, want %v", definition.Mode, domain.NodeEvent)
	}
	if definition.TriggerKind != domain.TriggerKV {
		t.Fatalf("trigger kind = %v, want %v", definition.TriggerKind, domain.TriggerKV)
	}
	if !definition.PortContractOwned {
		t.Fatal("trigger must own its port contract")
	}
	if len(definition.Capabilities) != 1 || definition.Capabilities[0] != domain.CapabilityNetwork {
		t.Fatalf("capabilities = %#v", definition.Capabilities)
	}
}

func TestDefinitionContract(t *testing.T) {
	definition := New().Definition()
	if len(definition.Inputs) != 0 {
		t.Fatalf("inputs = %#v", definition.Inputs)
	}
	outputIDs := make([]string, 0, len(definition.Outputs))
	for _, output := range definition.Outputs {
		outputIDs = append(outputIDs, output.ID)
		if output.DataType != domain.DataText && output.ID != "out" {
			t.Fatalf("output %q = %#v", output.ID, output)
		}
	}
	want := []string{"out", "channel", "message", "pattern", "receivedAt"}
	if len(outputIDs) != len(want) {
		t.Fatalf("outputs = %#v, want %v", outputIDs, want)
	}
	for index := range want {
		if outputIDs[index] != want[index] {
			t.Fatalf("outputs = %#v, want %v", outputIDs, want)
		}
	}
	fields := map[string]string{}
	for _, field := range definition.Fields {
		fields[field.Name] = field.Kind
	}
	if fields["databaseId"] != "kv-database-select" || fields["channels"] != "string" || fields["patterns"] != "string" {
		t.Fatalf("fields = %#v", definition.Fields)
	}
	if definition.DefaultConfig["channels"] != "" || definition.DefaultConfig["patterns"] != "" || definition.DefaultConfig["databaseId"] != "" {
		t.Fatalf("defaults = %#v", definition.DefaultConfig)
	}
}

func TestResolveReturnsStaticContract(t *testing.T) {
	module := New()
	node := domain.FlowNode{ID: "kv-sub-1", Type: "kv:subscribe", Data: map[string]any{"config": map[string]any{"channels": "events:*"}}}
	definition, err := module.Resolve(node)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(definition.Outputs) != len(module.Definition().Outputs) {
		t.Fatalf("Resolve() outputs = %#v", definition.Outputs)
	}
}

// The trigger owns only metadata: message delivery belongs to the kvsub
// service, so the module intentionally ships no executor.
func TestExecuteHasNoExecutor(t *testing.T) {
	module := New()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Node:            domain.FlowNode{ID: "kv-sub-1", Type: "kv:subscribe"},
		SchemaVersion:   domain.GraphSchemaV3,
		Config:          map[string]any{"databaseId": "db-1", "channels": "events:*"},
		Inputs:          map[string]any{},
		ConnectedInputs: map[string]bool{},
	}, nil); err == nil {
		t.Fatal("Execute() must fail because delivery is service-owned")
	}
}
