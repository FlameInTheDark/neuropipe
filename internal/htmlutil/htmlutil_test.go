package htmlutil

import (
	"strings"
	"testing"
)

const sample = `<!DOCTYPE html><html><head><title>Page</title><style>body{color:red}</style><link rel="stylesheet" href="a.css"><link rel="icon" href="favicon.png"></head><body><h1>Hello</h1><script>alert(1)</script><noscript>no js</noscript><p>World</p></body></html>`

func TestCleanRemovesScriptsAndStyles(t *testing.T) {
	cleaned, err := Clean(sample, true, true)
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	for _, unwanted := range []string{"alert(1)", "noscript", "color:red", "a.css"} {
		if strings.Contains(cleaned, unwanted) {
			t.Fatalf("cleaned HTML still contains %q: %s", unwanted, cleaned)
		}
	}
	for _, wanted := range []string{"Hello", "World", "favicon.png"} {
		if !strings.Contains(cleaned, wanted) {
			t.Fatalf("cleaned HTML lost %q: %s", wanted, cleaned)
		}
	}
}

func TestCleanRemovesOnlyScripts(t *testing.T) {
	cleaned, err := Clean(sample, true, false)
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if strings.Contains(cleaned, "alert(1)") || !strings.Contains(cleaned, "color:red") {
		t.Fatalf("script-only clean wrong: %s", cleaned)
	}
}

func TestCleanReturnsSourceUnchanged(t *testing.T) {
	cleaned, err := Clean(sample, false, false)
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if cleaned != sample {
		t.Fatal("Clean() without flags must return the source unchanged")
	}
}
