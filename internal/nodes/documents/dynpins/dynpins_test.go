package dynpins

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func TestConfiguredAcceptsValidRows(t *testing.T) {
	config := map[string]any{"valuePins": []any{
		map[string]any{"id": "field_1", "name": "customer", "label": "Customer", "value": "Contoso"},
		map[string]any{"name": "amount"}, // no id, no label: both derived
	}}
	rows, err := Configured(config, "valuePins")
	if err != nil {
		t.Fatalf("Configured() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].ID != "field_1" || rows[0].Name != "customer" || rows[0].Label != "Customer" || rows[0].Value != "Contoso" {
		t.Fatalf("row 0 = %#v", rows[0])
	}
	if rows[1].ID != "row_2" || rows[1].Label != "amount" {
		t.Fatalf("row 1 = %#v", rows[1])
	}
}

func TestConfiguredDropsBlankAndRejectsBrokenRows(t *testing.T) {
	rows, err := Configured(map[string]any{"cellPins": []any{map[string]any{"name": "  "}}}, "cellPins")
	if err != nil {
		t.Fatalf("blank rows should be dropped, got %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %#v", rows)
	}
	if _, err := Configured(map[string]any{"cellPins": map[string]any{"B4": 1}}, "cellPins"); err == nil {
		t.Fatal("non-list config must fail")
	}
	if _, err := Configured(map[string]any{"cellPins": []any{
		map[string]any{"id": "a", "name": "B4"}, map[string]any{"id": "a", "name": "C4"},
	}}, "cellPins"); err == nil || !strings.Contains(err.Error(), "duplicate pin ID") {
		t.Fatalf("duplicate ids must fail, got %v", err)
	}
	if _, err := Configured(map[string]any{"cellPins": []any{
		map[string]any{"id": "a", "name": "B4"}, map[string]any{"id": "b", "name": "B4"},
	}}, "cellPins"); err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("duplicate names must fail, got %v", err)
	}
	if _, err := Configured(map[string]any{"cellPins": []any{map[string]any{"id": "pin_a", "name": "B4"}}}, "cellPins"); err == nil {
		t.Fatal("pin_ prefix must be rejected")
	}
	tooMany := make([]any, MaxRows+1)
	for index := range tooMany {
		tooMany[index] = map[string]any{"name": string(rune('A'+index%26)) + string(rune('0'+index/26))}
	}
	if _, err := Configured(map[string]any{"cellPins": tooMany}, "cellPins"); err == nil {
		t.Fatal("row cap must be enforced")
	}
}

func TestInputAndOutputPins(t *testing.T) {
	rows := []Row{
		{ID: "field_1", Name: "customer", Label: "Customer", Value: "Contoso"},
		{ID: "field_2", Name: "amount"},
	}
	inputs := InputPins(rows, "#a1a1aa")
	if len(inputs) != 2 {
		t.Fatalf("inputs = %#v", inputs)
	}
	first := inputs[0]
	if first.ID != "pin_field_1" || first.Label != "Customer" || first.Kind != domain.PinData || first.Direction != domain.PinInput {
		t.Fatalf("first pin = %#v", first)
	}
	if first.Type == nil || first.Type.Kind != domain.TypeAny {
		t.Fatalf("first pin type = %#v", first.Type)
	}
	if first.MaxConnections != 1 || first.Default != "Contoso" {
		t.Fatalf("first pin contract = %#v", first)
	}
	second := inputs[1]
	if second.ID != "pin_field_2" || second.Default != nil {
		t.Fatalf("second pin = %#v", second)
	}
	outputs := OutputPins(rows, "#a1a1aa")
	if len(outputs) != 2 || outputs[0].Direction != domain.PinOutput || outputs[0].Default != nil {
		t.Fatalf("outputs = %#v", outputs)
	}
}

func TestValuesPrefersWiredOverLiteral(t *testing.T) {
	rows := []Row{
		{ID: "field_1", Name: "customer", Value: "Contoso"},
		{ID: "field_2", Name: "amount", Value: float64(7)},
		{ID: "field_3", Name: "skipped"}, // no wire, no literal
	}
	invocation := nodes.Invocation{Inputs: map[string]any{
		"pin_field_1": "Litware",
	}}
	values := Values(invocation, rows)
	if len(values) != 2 {
		t.Fatalf("values = %#v", values)
	}
	if values["customer"] != "Litware" {
		t.Fatalf("wired value must win: %#v", values["customer"])
	}
	if values["amount"] != float64(7) {
		t.Fatalf("literal fallback missing: %#v", values["amount"])
	}
	if _, exists := values["skipped"]; exists {
		t.Fatalf("empty row leaked into values: %#v", values)
	}
}
