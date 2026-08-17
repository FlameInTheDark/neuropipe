package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const dirtyPage = `<!DOCTYPE html><html><head><style>.x{color:red}</style></head><body><h1>Report</h1><script>track()</script></body></html>`

func TestHTTPNodeStripsScriptsAndStylesFromHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(dirtyPage))
	}))
	defer server.Close()

	engine := NewEngine(nil, nil, nil)
	result, err := engine.executeHTTP(context.Background(), map[string]any{"url": server.URL, "stripScripts": true, "stripStyles": true}, Packet{})
	if err != nil {
		t.Fatalf("executeHTTP() error = %v", err)
	}
	body, _ := result["out"][0]["body"].(string)
	if strings.Contains(body, "track()") || strings.Contains(body, "color:red") {
		t.Fatalf("body still contains stripped markup: %s", body)
	}
	if !strings.Contains(body, "Report") {
		t.Fatalf("body lost visible content: %s", body)
	}
}

func TestHTTPNodeLeavesHTMLAndOtherContentUntouchedWithoutToggles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(dirtyPage))
	}))
	defer server.Close()

	engine := NewEngine(nil, nil, nil)
	result, err := engine.executeHTTP(context.Background(), map[string]any{"url": server.URL}, Packet{})
	if err != nil {
		t.Fatalf("executeHTTP() error = %v", err)
	}
	body, _ := result["out"][0]["body"].(string)
	if body != dirtyPage {
		t.Fatal("body must stay byte-identical when no strip toggle is set")
	}
}

func TestIsHTMLResponseSniffsContentTypeAndMarkup(t *testing.T) {
	tests := []struct {
		contentType string
		body        string
		want        bool
	}{
		{contentType: "text/html; charset=utf-8", body: "anything", want: true},
		{contentType: "application/json", body: `{"a":1}`, want: false},
		{contentType: "", body: "<!DOCTYPE html><html></html>", want: true},
		{contentType: "", body: "<html><body></body></html>", want: true},
		{contentType: "", body: `{"looks":"json"}`, want: false},
	}
	for _, test := range tests {
		if got := isHTMLResponse(test.contentType, test.body); got != test.want {
			t.Fatalf("isHTMLResponse(%q, %q) = %v, want %v", test.contentType, test.body, got, test.want)
		}
	}
}
