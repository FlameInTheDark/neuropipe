package attachments

import "testing"

func TestSourceModeNormalises(t *testing.T) {
	cases := map[any]string{
		"url": "url", "file": "file", "base64": "base64", "bytes": "bytes",
		"": "", nil: "", "URL": "", " bogus ": "", 42: "",
	}
	for value, want := range cases {
		if got := SourceMode(value); got != want {
			t.Fatalf("SourceMode(%#v) = %q, want %q", value, got, want)
		}
	}
}

func TestSourceIncludes(t *testing.T) {
	if !SourceIncludes("", "url") || !SourceIncludes("", "bytes") {
		t.Fatal("auto mode must include every source")
	}
	if !SourceIncludes("url", "url") || SourceIncludes("url", "file") || SourceIncludes("bytes", "url") {
		t.Fatal("explicit mode must include only its own source")
	}
}

func TestNameIncludes(t *testing.T) {
	for _, mode := range []string{"", "base64", "bytes"} {
		if !NameIncludes(mode) {
			t.Fatalf("name must participate in %q mode", mode)
		}
	}
	for _, mode := range []string{"url", "file"} {
		if NameIncludes(mode) {
			t.Fatalf("name must not participate in %q mode", mode)
		}
	}
}

func TestDataValueFiltersEmptyPins(t *testing.T) {
	if DataValue(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if DataValue("") != nil || DataValue("   ") != nil {
		t.Fatal("blank text must collapse to nil")
	}
	if got := DataValue("aGVsbG8="); got != "aGVsbG8=" {
		t.Fatalf("payload text must pass through, got %#v", got)
	}
	if got := DataValue([]byte("x")); string(got.([]byte)) != "x" {
		t.Fatal("byte payloads must pass through")
	}
}
