package typespec

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestAssignableUsesExplicitGoStyleConversions(t *testing.T) {
	if Assignable(Int(), String()) || Assignable(Int(), Float()) || Assignable(Any(), Int()) {
		t.Fatal("implicit conversion or any narrowing was accepted")
	}
	if !Assignable(Int(), Any()) || !Assignable(Int(), Int()) {
		t.Fatal("valid assignment was rejected")
	}
}

func TestStructuralRecordAssignability(t *testing.T) {
	person := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "name", Type: String()}, {Name: "age", Type: Int()}}}
	nameOnly := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "name", Type: String()}}}
	if !Assignable(person, nameOnly) {
		t.Fatal("a record with required fields should satisfy a smaller structural contract")
	}
	if Assignable(nameOnly, person) {
		t.Fatal("a record missing age should not satisfy person")
	}
}

func TestValidateValueChecksNestedRecords(t *testing.T) {
	type profile struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	contract := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "name", Type: String()}, {Name: "age", Type: Int()}}}
	if err := ValidateValue(profile{Name: "Ada", Age: 37}, contract); err != nil {
		t.Fatalf("ValidateValue() error = %v", err)
	}
	if err := ValidateValue("not an object", contract); err == nil {
		t.Fatal("ValidateValue() accepted text for a record")
	}
	if err := ValidateValue(map[string]any{"name": "Ada", "age": "37"}, contract); err == nil {
		t.Fatal("ValidateValue() accepted string for int")
	}
}

func TestValidateValueAllowsAnyNestedInsideStructuredContracts(t *testing.T) {
	key, value := String(), Any()
	contract := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "payload", Type: domain.TypeSpec{Kind: domain.TypeMap, Key: &key, Value: &value}}}}
	if err := ValidateValue(map[string]any{"payload": map[string]any{"count": 3, "items": []any{"text"}}}, contract); err != nil {
		t.Fatalf("nested any value was rejected: %v", err)
	}
}

func TestNamedRecordRequiresItsNamedGoValue(t *testing.T) {
	type User struct{ Name string }
	type Account struct{ Name string }
	contract := domain.TypeSpec{Kind: domain.TypeRecord, Name: "User", Fields: []domain.TypeFieldSpec{{Name: "Name", Type: String()}}}
	if err := ValidateValue(User{Name: "Ada"}, contract); err != nil {
		t.Fatalf("named User was rejected: %v", err)
	}
	if err := ValidateValue(Account{Name: "Ada"}, contract); err == nil {
		t.Fatal("different named record was accepted")
	}
}

func TestValidateValueAcceptsNumericKindsForFloatContracts(t *testing.T) {
	// JSON has one number domain. Producers may hand execution a Go int (e.g.
	// HTTP status codes) or a float64 (decoded JSON), and both satisfy a
	// declared number contract; the strict int→float pin rule is connection
	// assignability, not value validation.
	for _, value := range []any{200, int64(200), uint(200), float32(200), float64(200)} {
		if err := ValidateValue(value, Float()); err != nil {
			t.Fatalf("ValidateValue(%T) error = %v", value, err)
		}
	}
	if err := ValidateValue(1, Int()); err != nil {
		t.Fatalf("ValidateValue(1, int) error = %v", err)
	}
	if err := ValidateValue(1.0, Int()); err == nil {
		t.Fatal("ValidateValue() accepted a float for an int contract")
	}
	if err := ValidateValue("200", Float()); err == nil {
		t.Fatal("ValidateValue() accepted a string for a float contract")
	}
	if err := ValidateValue(map[string]any{"age": int(37)}, domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "age", Type: Float()}}}); err != nil {
		t.Fatalf("record with an int age should pass a float field contract: %v", err)
	}
}

func TestValidateSpec(t *testing.T) {
	text := String()
	if err := ValidateSpec(domain.TypeSpec{Kind: domain.TypeList, Element: &text}); err != nil {
		t.Fatalf("list contract: %v", err)
	}
	if err := ValidateSpec(domain.TypeSpec{Kind: domain.TypeMap, Key: &domain.TypeSpec{Kind: domain.TypeList}, Value: &text}); err == nil {
		t.Fatal("list map key should be rejected")
	}
	if err := ValidateSpec(domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{{Name: "value", Type: text}, {Name: "value", Type: text}}}); err == nil {
		t.Fatal("duplicate record field should be rejected")
	}
}
