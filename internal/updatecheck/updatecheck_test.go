package updatecheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSource struct {
	release Release
	err     error
}

func (s fakeSource) Latest(context.Context) (Release, error) { return s.release, s.err }

func TestCheckerCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		current    string
		latest     string
		wantUpdate bool
		wantError  bool
	}{
		{name: "newer patch", current: "v1.1.0", latest: "v1.1.1", wantUpdate: true},
		{name: "same version", current: "v1.1.0", latest: "v1.1.0"},
		{name: "older release", current: "v1.2.0", latest: "v1.1.9"},
		{name: "release beats prerelease", current: "v1.2.0-rc.1", latest: "v1.2.0", wantUpdate: true},
		{name: "development build is skipped", current: "dev", latest: "v9.9.9"},
		{name: "malformed release tag", current: "v1.1.0", latest: "latest", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			checker := NewChecker(fakeSource{release: Release{Version: test.latest, URL: "https://github.com/FlameInTheDark/neuropipe/releases"}}, test.current)
			release, available, err := checker.Check(context.Background())
			if test.wantError {
				if err == nil {
					t.Fatal("Check() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if available != test.wantUpdate {
				t.Errorf("available = %t, want %t", available, test.wantUpdate)
			}
			if test.current != "dev" && release.Version != test.latest {
				t.Errorf("release version = %q, want %q", release.Version, test.latest)
			}
		})
	}
}

func TestCheckerHandlesSourceFailure(t *testing.T) {
	t.Parallel()
	checker := NewChecker(fakeSource{err: errors.New("offline")}, "v1.0.0")
	if _, _, err := checker.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("Check() error = %v, want wrapped source error", err)
	}
}

func TestGitHubSourceLatest(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept = %q", request.Header.Get("Accept"))
		}
		if request.Header.Get("User-Agent") != "Neuropipe-update-checker" {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/FlameInTheDark/neuropipe/releases/tag/v1.2.3"}`))
	}))
	defer server.Close()
	source := &GitHubSource{client: server.Client(), endpoint: server.URL}
	release, err := source.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if release.Version != "v1.2.3" || !strings.HasSuffix(release.URL, "/v1.2.3") {
		t.Errorf("release = %#v", release)
	}
}

func TestGitHubSourceRejectsUnexpectedResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte("rate limited"))
	}))
	defer server.Close()
	source := &GitHubSource{client: server.Client(), endpoint: server.URL}
	if _, err := source.Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("Latest() error = %v, want status error", err)
	}
}

func TestParseSemanticVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		wantErr bool
	}{
		{input: "v1.2.3"},
		{input: "1.2.3-rc.1+build.7"},
		{input: "1.2", wantErr: true},
		{input: "1.2.3-01", wantErr: true},
		{input: "1.2.3+", wantErr: true},
		{input: "1.2.3+build_1", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			_, err := parseSemanticVersion(test.input)
			if (err != nil) != test.wantErr {
				t.Errorf("parseSemanticVersion(%q) error = %v, want error: %t", test.input, err, test.wantErr)
			}
		})
	}
}
