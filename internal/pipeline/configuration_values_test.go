package pipeline

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestCanonicalConfigurationIntegerIsTypeDirected(t *testing.T) {
	value, err := canonicalConfigurationValue(float64(3), domain.TypeSpec{Kind: domain.TypeInt})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := value.(int); !ok || got != 3 {
		t.Fatalf("canonical integer = %#v (%T)", value, value)
	}
	if _, err := canonicalConfigurationValue(float64(3.5), domain.TypeSpec{Kind: domain.TypeInt}); err == nil {
		t.Fatal("fractional JSON config was accepted as an Int")
	}
	value, err = canonicalConfigurationValue(float64(3), domain.TypeSpec{Kind: domain.TypeFloat})
	if err != nil || value != float64(3) {
		t.Fatalf("float configuration = %#v, %v", value, err)
	}
}
