package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// atomFeedXML renders a plain Atom feed page for the given build tags.
func atomFeedXML(tags ...string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?><feed>`)
	for _, tag := range tags {
		fmt.Fprintf(&builder, `<entry><title>%s</title><updated>2026-02-01T00:00:00Z</updated></entry>`, tag)
	}
	builder.WriteString(`</feed>`)
	return builder.String()
}

// assetsHTML renders an expanded-assets-like fragment whose download anchors
// mirror the paths github.com serves.
func assetsHTML(tag string, names ...string) string {
	var builder strings.Builder
	for _, name := range names {
		fmt.Fprintf(&builder, `<li><a href="/ggml-org/llama.cpp/releases/download/%s/%s">%s</a></li>`, tag, name, name)
	}
	return builder.String()
}

func windowsAssetNames(tag string) []string {
	return []string{
		"llama-" + tag + "-bin-win-cpu-arm64.zip",
		"llama-" + tag + "-bin-win-cpu-x64.zip",
		"llama-" + tag + "-bin-win-cuda-12.4-x64.zip",
		"llama-" + tag + "-bin-win-cuda-13.4-arm64.zip",
		"cudart-llama-bin-win-cuda-12.4-x64.zip",
		"llama-" + tag + "-bin-win-rocm-7.14-x64.zip",
		"llama-" + tag + "-bin-win-vulkan-x64.zip",
	}
}

// TestLlamaCatalogWebFallbackServesLiveListWhenAPIFails reproduces the
// reported setup: the GitHub API is unreachable but github.com pages load,
// exactly like a browser. The listing must come from the web pages, carry a
// notice, and persist to the cache with the web source.
func TestLlamaCatalogWebFallbackServesLiveListWhenAPIFails(t *testing.T) {
	var feedFetches, assetFetches int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/releases"):
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`{"message":"API rate limit exceeded"}`))
		case strings.HasSuffix(request.URL.Path, "/releases.atom"):
			atomic.AddInt32(&feedFetches, 1)
			switch request.URL.Query().Get("after") {
			case "":
				_, _ = writer.Write([]byte(atomFeedXML("b600", "b599")))
			case "b599":
				_, _ = writer.Write([]byte(atomFeedXML("b598")))
			default:
				_, _ = writer.Write([]byte(atomFeedXML()))
			}
		case strings.Contains(request.URL.Path, "/releases/expanded_assets/"):
			atomic.AddInt32(&assetFetches, 1)
			tag := path.Base(request.URL.Path)
			_, _ = writer.Write([]byte(assetsHTML(tag, windowsAssetNames(tag)...)))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	listing, err := catalog.ListWithInfo(context.Background())
	if err != nil {
		t.Fatalf("ListWithInfo() error = %v, want the web fallback to serve a live list", err)
	}
	if listing.Source != releaseSourceWeb {
		t.Fatalf("source = %q, want %q", listing.Source, releaseSourceWeb)
	}
	if listing.Notice == "" {
		t.Fatal("notice is empty, want an explanation of the API fallback")
	}
	if len(listing.Releases) != 3 {
		t.Fatalf("releases = %d, want 3 (b600, b599, b598)", len(listing.Releases))
	}
	if listing.Releases[0].Version != "b600" || listing.Releases[2].Version != "b598" {
		t.Fatalf("releases are not newest-first: %v", listing.Releases)
	}
	// arm64 assets must be ignored, rocm builds map to HIP, CUDA pairs with cudart.
	if listing.Releases[0].CPU.URL == "" || listing.Releases[0].CUDA.URL == "" || listing.Releases[0].Vulkan.URL == "" || listing.Releases[0].HIP.URL == "" {
		t.Fatalf("release artifacts incomplete: %#v", listing.Releases[0])
	}
	if !strings.HasPrefix(listing.Releases[0].CPU.URL, server.URL+"/") {
		t.Fatalf("CPU URL = %q, want an absolute URL on the test server", listing.Releases[0].CPU.URL)
	}
	if feedFetches < 2 {
		t.Fatalf("feed fetches = %d, want pagination to follow ?after=", feedFetches)
	}
	if assetFetches != 3 {
		t.Fatalf("asset fetches = %d, want one per release", assetFetches)
	}
	cached, record, ok := catalog.loadReleaseCache()
	if !ok || record.Source != releaseSourceWeb || len(cached) != 3 {
		t.Fatalf("cache = %d manifests, source %q, ok %v, want 3 web-sourced manifests", len(cached), record.Source, ok)
	}
	if cached[0].Cudart.URL == "" {
		t.Fatal("cached web manifest lost the cudart artifact")
	}
}

// TestLlamaCatalogWebFallbackToleratesFailingAssetPages keeps a partial live
// listing when one release's asset page fails.
func TestLlamaCatalogWebFallbackToleratesFailingAssetPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/releases"):
			writer.WriteHeader(http.StatusForbidden)
		case strings.HasSuffix(request.URL.Path, "/releases.atom"):
			_, _ = writer.Write([]byte(atomFeedXML("b600", "b599")))
		case strings.HasSuffix(request.URL.Path, "/releases/expanded_assets/b600"):
			writer.WriteHeader(http.StatusInternalServerError)
		default:
			tag := path.Base(request.URL.Path)
			_, _ = writer.Write([]byte(assetsHTML(tag, "llama-"+tag+"-bin-win-cpu-x64.zip")))
		}
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	listing, err := catalog.ListWithInfo(context.Background())
	if err != nil {
		t.Fatalf("ListWithInfo() error = %v, want the surviving release", err)
	}
	if listing.Source != releaseSourceWeb || len(listing.Releases) != 1 || listing.Releases[0].Version != "b599" {
		t.Fatalf("listing = %#v (source %q), want only b599 from the web", listing.Releases, listing.Source)
	}
}

// TestLlamaCatalogPrefersFreshCacheOverWebScrape verifies the fresh-cache
// shortcut: while the API is down, a listing younger than
// releaseCacheFreshFor is served from disk without touching github.com pages.
func TestLlamaCatalogPrefersFreshCacheOverWebScrape(t *testing.T) {
	var assetFetches int32
	apiDown := true
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/releases"):
			if apiDown {
				writer.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = writer.Write([]byte(releasesJSON(request.Host)))
		case strings.HasSuffix(request.URL.Path, "/releases.atom"):
			_, _ = writer.Write([]byte(atomFeedXML("b600")))
		default:
			atomic.AddInt32(&assetFetches, 1)
			tag := path.Base(request.URL.Path)
			_, _ = writer.Write([]byte(assetsHTML(tag, "llama-"+tag+"-bin-win-cpu-x64.zip")))
		}
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	first, err := catalog.ListWithInfo(context.Background())
	if err != nil || first.Source != releaseSourceWeb {
		t.Fatalf("first listing = %q, %v, want the web fallback", first.Source, err)
	}
	if got := atomic.LoadInt32(&assetFetches); got != 1 {
		t.Fatalf("asset fetches after priming = %d, want 1", got)
	}

	second, err := catalog.ListWithInfo(context.Background())
	if err != nil {
		t.Fatalf("second listing error = %v, want the fresh cache", err)
	}
	if second.Source != releaseSourceCache {
		t.Fatalf("second listing source = %q, want %q", second.Source, releaseSourceCache)
	}
	if second.Notice == "" {
		t.Fatal("cache notice is empty, want an explanation")
	}
	if got := atomic.LoadInt32(&assetFetches); got != 1 {
		t.Fatalf("asset fetches after cached listing = %d, want the web pages untouched", got)
	}
}

// TestLlamaCatalogRefreshBypassesFreshCache makes sure the forced refresh
// used by the Runtime page's refresh button re-scrapes even with a fresh cache.
func TestLlamaCatalogRefreshBypassesFreshCache(t *testing.T) {
	var assetFetches int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/releases"):
			writer.WriteHeader(http.StatusForbidden)
		case strings.HasSuffix(request.URL.Path, "/releases.atom"):
			_, _ = writer.Write([]byte(atomFeedXML("b600")))
		default:
			atomic.AddInt32(&assetFetches, 1)
			tag := path.Base(request.URL.Path)
			_, _ = writer.Write([]byte(assetsHTML(tag, "llama-"+tag+"-bin-win-cpu-x64.zip")))
		}
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	if _, err := catalog.ListWithInfo(context.Background()); err != nil {
		t.Fatalf("priming listing error = %v", err)
	}
	forced, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v, want a fresh web lookup", err)
	}
	if forced.Source != releaseSourceWeb {
		t.Fatalf("Refresh() source = %q, want %q", forced.Source, releaseSourceWeb)
	}
	if got := atomic.LoadInt32(&assetFetches); got != 2 {
		t.Fatalf("asset fetches after forced refresh = %d, want a re-scrape", got)
	}
}

// TestLlamaCatalogServesStaleCacheWithNotice covers the last resort: the API
// is down, the web pages are down, and only an aged cache remains.
func TestLlamaCatalogServesStaleCacheWithNotice(t *testing.T) {
	everythingDown := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if everythingDown {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if strings.HasSuffix(request.URL.Path, "/releases") {
			_, _ = writer.Write([]byte(releasesJSON(request.Host)))
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()

	root := t.TempDir()
	catalog := NewLlamaCatalog(root)
	catalog.releasesURL, catalog.http, catalog.webBase = server.URL+"/releases", server.Client(), server.URL

	if _, err := catalog.ListWithInfo(context.Background()); err != nil {
		t.Fatalf("priming listing error = %v", err)
	}
	// Age the cache beyond releaseCacheFreshFor so both live sources are retried.
	payload, err := json.Marshal(releaseCacheRecord{
		FetchedAt: time.Now().UTC().Add(-2 * time.Hour),
		Source:    releaseSourceAPI,
		Manifests: []releaseManifest{{Release: domain.LlamaRuntimeRelease{Version: "b600", CPU: domain.RuntimeArtifact{URL: "http://example.invalid/cpu.zip"}}}},
	})
	if err != nil {
		t.Fatalf("marshal aged cache: %v", err)
	}
	if err := os.WriteFile(path.Join(root, "releases-cache.json"), payload, 0o600); err != nil {
		t.Fatalf("write aged cache: %v", err)
	}
	everythingDown = true

	listing, err := catalog.ListWithInfo(context.Background())
	if err != nil {
		t.Fatalf("ListWithInfo() error = %v, want the stale cache to serve the page", err)
	}
	if listing.Source != releaseSourceCache || len(listing.Releases) != 1 || listing.Releases[0].Version != "b600" {
		t.Fatalf("listing = %#v (source %q), want the cached b600 listing", listing.Releases, listing.Source)
	}
	if !strings.Contains(listing.Notice, "cached") {
		t.Fatalf("notice = %q, want it to mention the cache", listing.Notice)
	}
}
