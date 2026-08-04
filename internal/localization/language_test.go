package localization

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":      English,
		"en":    English,
		"DE-de": German,
		"fr_FR": French,
		"ru":    Russian,
		"ja":    English,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got := Normalize(input); got != want {
				t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestIsSupported(t *testing.T) {
	t.Parallel()
	if !IsSupported(Russian) || IsSupported("ru-RU") {
		t.Fatal("supported-language matching is not exact")
	}
}
