package textbytes

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestResolveAndPinPreserveExactWireTypes(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		config    map[string]any
		want      Representation
		wantKind  domain.TypeKind
		wantError bool
	}{
		{name: "default", config: map[string]any{}, want: Text, wantKind: domain.TypeString},
		{name: "bytes", config: map[string]any{"representation": "bytes"}, want: Bytes, wantKind: domain.TypeBytes},
		{name: "non-string", config: map[string]any{"representation": true}, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			representation, err := Resolve(testCase.config, map[string]any{"representation": "text"}, "representation")
			if (err != nil) != testCase.wantError {
				t.Fatalf("Resolve() error = %v, want error %t", err, testCase.wantError)
			}
			if testCase.wantError {
				return
			}
			if representation != testCase.want {
				t.Fatalf("Resolve() = %q, want %q", representation, testCase.want)
			}
			pin := Pin("value", "Value", domain.PinInput, representation, true)
			if pin.Type == nil || pin.Type.Kind != testCase.wantKind {
				t.Fatalf("Pin() = %#v", pin)
			}
		})
	}
}

func TestTextRepresentationNeverTransportsInvalidUTF8(t *testing.T) {
	if _, err := InputBytes(string([]byte{0xff}), Text); err == nil {
		t.Fatal("InputBytes() accepted invalid UTF-8 text")
	}
	if _, err := OutputValue([]byte{0xff}, Text); err == nil {
		t.Fatal("OutputValue() accepted invalid UTF-8 text")
	}
}
