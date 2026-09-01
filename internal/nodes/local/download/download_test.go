package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func registeredModule(t *testing.T) nodes.Node {
	t.Helper()
	registry := nodes.New()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	module, ok := registry.Get("action:download")
	if !ok {
		t.Fatal("action:download was not registered")
	}
	return module
}

func invocation(definition domain.NodeDefinition, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: "action:download", Data: map[string]any{"config": config}},
		Definition:      definition,
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func assertPinIDs(t *testing.T, ports []domain.NodePort, want []string) {
	t.Helper()
	got := make([]string, 0, len(ports))
	for _, port := range ports {
		got = append(got, port.ID)
	}
	if len(got) != len(want) {
		t.Fatalf("pin ids = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pin ids = %v, want %v", got, want)
		}
	}
}

// newServer serves one fixed body under any path and records the
// User-Agent of the last request it saw.
func newServer(t *testing.T, body string, status int, userAgent *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userAgent != nil {
			*userAgent = r.Header.Get("User-Agent")
		}
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRegistrationMetadata(t *testing.T) {
	definition := registeredModule(t).Definition()
	if definition.Type != "action:download" || definition.Mode != domain.NodeImpure || definition.Category != "Local" {
		t.Fatalf("definition = %#v", definition)
	}
	assertPinIDs(t, definition.Inputs, []string{"in", "url", "location"})
	assertPinIDs(t, definition.Outputs, []string{"out", "result"})
}

func TestDownloadsFileFromServer(t *testing.T) {
	body := "payload bytes"
	var userAgent string
	server := newServer(t, body, http.StatusOK, &userAgent)
	target := t.TempDir()

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"url": server.URL + "/file.txt", "location": target,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ports) != 1 || result.Ports[0] != "out" {
		t.Fatalf("ports = %#v", result.Ports)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if output["path"] != filepath.Join(target, "file.txt") || output["bytes"] != int64(len(body)) || output["status"] != http.StatusOK {
		t.Fatalf("result output = %#v", output)
	}
	if saved, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(saved) != body {
		t.Fatalf("saved file = %q, err = %v", saved, err)
	}
	if userAgent != "Neuropipe/0.1" {
		t.Fatalf("user agent = %q", userAgent)
	}
}

// Query strings must not leak into the derived file name.
func TestQuerySuffixIsIgnoredForFileName(t *testing.T) {
	server := newServer(t, "body", http.StatusOK, nil)
	target := t.TempDir()

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"url": server.URL + "/report.pdf?version=2&token=x", "location": target,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["path"] != filepath.Join(target, "report.pdf") {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
}

func TestConfigFallbackForURLAndLocation(t *testing.T) {
	server := newServer(t, "from config", http.StatusOK, nil)
	target := t.TempDir()

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"url": server.URL + "/config.txt", "location": target,
	}, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["path"] != filepath.Join(target, "config.txt") {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
	if saved, err := os.ReadFile(filepath.Join(target, "config.txt")); err != nil || string(saved) != "from config" {
		t.Fatalf("saved file = %q, err = %v", saved, err)
	}
}

// A wired URL pin wins over a stale inspector URL.
func TestURLPinOverridesConfig(t *testing.T) {
	server := newServer(t, "from pin", http.StatusOK, nil)
	target := t.TempDir()

	module := registeredModule(t)
	result, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{
		"url": server.URL + "/stale.txt", "location": target,
	}, map[string]any{
		"url": server.URL + "/pinned.txt",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := result.Outputs["result"].(map[string]any)
	if !ok || output["path"] != filepath.Join(target, "pinned.txt") {
		t.Fatalf("result output = %#v", result.Outputs["result"])
	}
}

func TestHTTPErrorStatusFails(t *testing.T) {
	server := newServer(t, "not here", http.StatusNotFound, nil)
	module := registeredModule(t)
	_, err := module.Execute(context.Background(), invocation(module.Definition(), map[string]any{}, map[string]any{
		"url": server.URL + "/missing.txt", "location": t.TempDir(),
	}), nil)
	if err == nil || !strings.Contains(err.Error(), "download request returned 404") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	module := registeredModule(t)
	definition := module.Definition()
	unreachable := "http://127.0.0.1:1/file.txt"
	cases := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		{"missing url", map[string]any{}, map[string]any{"location": t.TempDir()}, "download URL is required"},
		{"missing location", map[string]any{}, map[string]any{"url": unreachable}, "download location is required"},
		{"scheme-less url", map[string]any{}, map[string]any{"url": "example.com/file.txt", "location": t.TempDir()}, "download URL is invalid"},
		{"empty host", map[string]any{}, map[string]any{"url": "http:///file.txt", "location": t.TempDir()}, "download URL is invalid"},
		{"url without file name", map[string]any{}, map[string]any{"url": "http://127.0.0.1:1/", "location": t.TempDir()}, "does not expose a file name"},
	}
	for _, testCase := range cases {
		_, err := module.Execute(context.Background(), invocation(definition, testCase.config, testCase.inputs), nil)
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := registeredModule(t)
	_, err := module.Execute(ctx, invocation(module.Definition(), map[string]any{}, map[string]any{
		"url": "http://127.0.0.1:1/file.txt", "location": t.TempDir(),
	}), nil)
	if err == nil || !strings.Contains(err.Error(), "download cancelled") {
		t.Fatalf("err = %v", err)
	}
}
