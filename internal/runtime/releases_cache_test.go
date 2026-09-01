package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// releasesJSON is one valid Windows release with a CPU asset plus CUDA bits,
// enough for the pattern matcher to produce a manifest.
func releasesJSON(host string) string {
	return `[{"tag_name":"b600","published_at":"2026-02-01T00:00:00Z","assets":[` +
		`{"name":"llama-b600-bin-win-cpu-x64.zip","browser_download_url":"http://` + host + `/cpu.zip","size":10},` +
		`{"name":"llama-b600-bin-win-cuda-12.4-x64.zip","browser_download_url":"http://` + host + `/cuda.zip","size":20},` +
		`{"name":"cudart-llama-bin-win-cuda-12.4-x64.zip","browser_download_url":"http://` + host + `/cudart.zip","size":30}` +
		`]}]`
}

func TestLlamaCatalogListCachesSuccessfulReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(releasesJSON(request.Host)))
	}))
	defer server.Close()

	root := t.TempDir()
	catalog := NewLlamaCatalog(root)
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "b600" {
		t.Fatalf("List() = %#v, want b600", releases)
	}
	if _, err := os.Stat(filepath.Join(root, "releases-cache.json")); err != nil {
		t.Fatalf("release cache was not written: %v", err)
	}
	// The cached CUDA manifest must keep the cudart artifact so installs keep working offline.
	cached, record, ok := catalog.loadReleaseCache()
	if !ok || len(cached) != 1 {
		t.Fatalf("loadReleaseCache() = %#v, %v, want one manifest", cached, ok)
	}
	if record.Source != releaseSourceAPI {
		t.Fatalf("cache source = %q, want %q", record.Source, releaseSourceAPI)
	}
	if cached[0].Cudart.URL == "" || cached[0].Release.CUDA.URL == "" {
		t.Fatalf("cached manifest = %#v, want CUDA server and cudart artifacts preserved", cached[0])
	}
}

func TestLlamaCatalogListFallsBackToCacheWhenGitHubFails(t *testing.T) {
	failing := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if failing {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		_, _ = writer.Write([]byte(releasesJSON(request.Host)))
	}))
	defer server.Close()

	root := t.TempDir()
	catalog := NewLlamaCatalog(root)
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	// Prime the cache with one successful lookup, then break the network.
	if _, err := catalog.List(context.Background()); err != nil {
		t.Fatalf("priming List() error = %v", err)
	}
	failing = true

	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatalf("List() after rate limiting error = %v, want the cached listing", err)
	}
	if len(releases) != 1 || releases[0].Version != "b600" {
		t.Fatalf("List() = %#v, want the cached b600 listing", releases)
	}
}

func TestLlamaCatalogListFallsBackToCacheOnEmptyListing(t *testing.T) {
	empty := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if empty {
			_, _ = writer.Write([]byte(`[]`))
			return
		}
		_, _ = writer.Write([]byte(releasesJSON(request.Host)))
	}))
	defer server.Close()

	root := t.TempDir()
	catalog := NewLlamaCatalog(root)
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	if _, err := catalog.List(context.Background()); err != nil {
		t.Fatalf("priming List() error = %v", err)
	}
	empty = true

	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatalf("List() after an empty response error = %v, want the cached listing", err)
	}
	if len(releases) != 1 {
		t.Fatalf("List() = %#v, want the cached listing to serve the runtime page", releases)
	}
}

func TestLlamaCatalogListErrorsWithoutCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	releases, err := catalog.List(context.Background())
	if err == nil {
		t.Fatalf("List() = %#v, want an error when no cache can serve the page", releases)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("List() error = %v, want the underlying HTTP failure to surface", err)
	}
}

func TestLlamaCatalogListErrorsOnEmptyResponseWithoutCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	releases, err := catalog.List(context.Background())
	if err == nil {
		t.Fatalf("List() = %#v, want an error when the listing is empty and no cache exists", releases)
	}
	if !strings.Contains(err.Error(), "no compatible Windows x64 llama.cpp releases") {
		t.Fatalf("List() error = %v, want the actionable zero-listing message", err)
	}
}

func TestLlamaCatalogIgnoresCorruptReleaseCache(t *testing.T) {
	root := t.TempDir()
	catalog := NewLlamaCatalog(root)
	if err := os.WriteFile(filepath.Join(root, "releases-cache.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}
	if _, _, ok := catalog.loadReleaseCache(); ok {
		t.Fatal("loadReleaseCache() = true for a corrupt file, want false")
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL
	if _, err := catalog.List(context.Background()); err == nil {
		t.Fatal("List() = nil error, want the fetch failure to surface when the cache is unusable")
	}
}
