package uuid

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestGenerateV4ReturnsValidUUID(t *testing.T) {
	module := NewGenerate()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{"version": "v4"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	value, ok := result.Outputs["value"].(string)
	if !ok || value == "" {
		t.Fatalf("expected non-empty value UUID, got %#v", result.Outputs["value"])
	}
	if !strings.Contains(value, "-") || len(value) != 36 {
		t.Fatalf("v4 UUID %q has unexpected shape", value)
	}
}

func TestGenerateV5RequiresNamespaceAndName(t *testing.T) {
	module := NewGenerate()
	if _, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{"version": "v5"},
	}, nil); err == nil {
		t.Fatal("v5 without namespace/name should error")
	}
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{"version": "v5"},
		Inputs: map[string]any{"namespace": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "name": "example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	value, ok := result.Outputs["value"].(string)
	if !ok || len(value) != 36 {
		t.Fatalf("v5 example.com returned an unexpected UUID: %#v", result.Outputs["value"])
	}
	// v5 is deterministic: same (namespace, name) must always produce the same UUID.
	again, _ := module.Execute(context.Background(), nodes.Invocation{
		Config: map[string]any{"version": "v5"},
		Inputs: map[string]any{"namespace": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "name": "example.com"},
	}, nil)
	if value != again.Outputs["value"] {
		t.Fatalf("v5 should be deterministic: %q vs %q", value, again.Outputs["value"])
	}
}

func TestValidateAcceptsValidUUID(t *testing.T) {
	module := NewValidate()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": "550e8400-e29b-41d4-a716-446655440000"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Outputs["result"].(bool) {
		t.Fatalf("expected valid UUID to be reported as valid")
	}
}

func TestValidateRejectsGarbage(t *testing.T) {
	module := NewValidate()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": "not-a-uuid"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs["result"].(bool) {
		t.Fatalf("expected garbage input to be reported as invalid")
	}
}

func TestParseReturnsVersion(t *testing.T) {
	module := NewParse()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": "550e8400-e29b-41d4-a716-446655440000"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not a record: %#v", result.Outputs["result"])
	}
	if !out["isValid"].(bool) {
		t.Fatalf("expected isValid=true for valid UUID")
	}
	if out["version"] != int64(4) {
		t.Fatalf("expected version 4, got %v", out["version"])
	}
}

func TestExtractFindsUUIDsInText(t *testing.T) {
	module := NewExtract()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"value": "first 550e8400-e29b-41d4-a716-446655440000 then 6ba7b810-9dad-11d1-80b4-00c04fd430c8 end"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	matches, ok := result.Outputs["result"].([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", result.Outputs["result"])
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 UUIDs, got %d", len(matches))
	}
}
