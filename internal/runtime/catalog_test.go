package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestLlamaCatalogListsAndInstallsVerifiedRuntime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("managed runtime installation is Windows-only")
	}
	archive := zipBytes(t, map[string]string{"bundle/llama-server.exe": "server"})
	digest := checksum(archive)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/releases":
			_, _ = fmt.Fprintf(writer, `[{"tag_name":"b500","published_at":"2026-01-01T00:00:00Z","assets":[{"name":"llama-b500-bin-win-cpu-x64.zip","browser_download_url":%q,"size":%d,"digest":"sha256:%s"}]}]`, "http://"+request.Host+"/cpu.zip", len(archive), digest)
		case "/cpu.zip":
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	catalog := NewLlamaCatalog(t.TempDir())
	catalog.releasesURL, catalog.http = server.URL+"/releases", server.Client()
	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "b500" || releases[0].CPU.SHA256 != digest {
		t.Fatalf("List() = %#v, want b500 CPU artifact", releases)
	}

	if err := catalog.Install(context.Background(), domain.LlamaRuntimeInstallRequest{Version: "b500", Mode: domain.RuntimeCPU}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !runtimeFileExists(catalog.RuntimeBinary("b500", domain.RuntimeCPU)) {
		t.Fatalf("installed binary %q is missing", catalog.RuntimeBinary("b500", domain.RuntimeCPU))
	}
	status, err := catalog.Status("b500")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.Installed) != 1 || !status.Installed[0].CPUInstalled {
		t.Fatalf("Status() = %#v, want installed CPU runtime", status)
	}
}

func TestModelCatalogResumesAndVerifiesModelDownload(t *testing.T) {
	model := []byte("verified GGUF model bytes")
	digest := checksum(model)
	var sawRange bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/models/acme/model":
			_, _ = fmt.Fprintf(writer, `{"author":"acme","downloads":42,"likes":7,"lastModified":"2026-08-02T00:00:00Z","tags":["gguf","text-generation"],"siblings":[{"rfilename":"model.Q4_K_M.gguf","size":%d,"lfs":{"oid":"%s","size":%d}}]}`, len(model), digest, len(model))
		case "/acme/model/resolve/main/model.Q4_K_M.gguf":
			if request.Header.Get("Range") == "bytes=8-" {
				sawRange = true
				writer.Header().Set("Content-Range", fmt.Sprintf("bytes 8-%d/%d", len(model)-1, len(model)))
				writer.WriteHeader(http.StatusPartialContent)
				_, _ = writer.Write(model[8:])
				return
			}
			_, _ = writer.Write(model)
		case "/acme":
			_, _ = writer.Write([]byte(`<img src="https://cdn-avatars.huggingface.co/v1/production/acme.png">`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	catalog := NewModelCatalog(root)
	catalog.hubURL, catalog.http = server.URL, server.Client()
	partial := filepath.Join(root, "acme__model", "model.Q4_K_M.gguf.part")
	if err := os.MkdirAll(filepath.Dir(partial), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partial, model[:8], 0o600); err != nil {
		t.Fatal(err)
	}

	progress := make([]domain.InstallProgress, 0)
	installed, err := catalog.InstallWithProgress(context.Background(), domain.ModelInstallRequest{Repository: "acme/model", File: "model.Q4_K_M.gguf"}, func(update domain.InstallProgress) {
		progress = append(progress, update)
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !sawRange {
		t.Fatal("model download did not resume from the partial file")
	}
	contents, err := os.ReadFile(installed.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, model) {
		t.Fatalf("installed model = %q, want %q", contents, model)
	}
	metadataData, err := os.ReadFile(installed.Path + ".neuropipe.json")
	if err != nil {
		t.Fatalf("read installed model metadata: %v", err)
	}
	var metadata domain.InstalledModelMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatalf("decode installed model metadata: %v", err)
	}
	if metadata.Repository != "acme/model" || metadata.File != "model.Q4_K_M.gguf" || metadata.Downloads != 42 || metadata.Quantization != "Q4_K_M" || metadata.SHA256 != digest || metadata.AvatarURL != "https://cdn-avatars.huggingface.co/v1/production/acme.png" {
		t.Fatalf("installed metadata = %#v, want Hugging Face source metadata", metadata)
	}
	listed, err := catalog.Installed(context.Background())
	if err != nil {
		t.Fatalf("Installed() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Repository != "acme/model" || listed[0].Author != "acme" || listed[0].AvatarURL != "https://cdn-avatars.huggingface.co/v1/production/acme.png" || listed[0].Downloads != 42 || listed[0].Quantization != "Q4_K_M" {
		t.Fatalf("Installed() = %#v, want sidecar source metadata", listed)
	}
	if len(progress) == 0 || progress[len(progress)-1].Stage != "installed" || progress[len(progress)-1].Percentage != 100 {
		t.Fatalf("install progress = %#v, want completed install progress", progress)
	}
	resumedProgress := false
	for _, update := range progress {
		if update.Stage == "downloading" && update.DownloadedBytes >= 8 {
			resumedProgress = true
			break
		}
	}
	if !resumedProgress {
		t.Fatalf("install progress = %#v, want resumed byte count", progress)
	}
}

func TestModelCatalogListsInstalledModels(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"acme__alpha/alpha.Q4_K_M.gguf":  "alpha",
		"org__beta/beta.gguf":            "beta",
		"org__beta/incomplete.gguf.part": "partial",
		"notes.txt":                      "ignore",
	}
	for relative, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	models, err := NewModelCatalog(root).Installed(context.Background())
	if err != nil {
		t.Fatalf("Installed() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("Installed() returned %d models, want 2: %#v", len(models), models)
	}
	if models[0].ID != "acme__alpha/alpha.Q4_K_M" || models[0].Name != "alpha.Q4_K_M.gguf" || models[0].Size != int64(len("alpha")) {
		t.Fatalf("first model = %#v, want alpha metadata", models[0])
	}
	if models[1].ID != "org__beta/beta" {
		t.Fatalf("second model = %#v, want beta", models[1])
	}
}

func TestModelCatalogSearchIncludesCreatorAvatar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/models":
			if request.URL.Query().Get("full") != "true" {
				http.Error(writer, "full model metadata is required", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`[{"id":"acme/model","author":"acme","downloads":42,"likes":7,"tags":["gguf"]}]`))
		case "/acme":
			_, _ = writer.Write([]byte(`<img src="https://cdn-avatars.huggingface.co/v1/production/acme.png">`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	catalog := NewModelCatalog(t.TempDir())
	catalog.hubURL, catalog.http = server.URL, server.Client()
	models, err := catalog.Search(context.Background(), domain.ModelSearchRequest{Sort: "recommended"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(models) != 1 || models[0].AvatarURL != "https://cdn-avatars.huggingface.co/v1/production/acme.png" {
		t.Fatalf("Search() = %#v, want creator avatar URL", models)
	}
}

func TestModelCatalogIgnoresMalformedInstalledMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "acme__alpha", "alpha.Q4_K_M.gguf")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".neuropipe.json", []byte(`{"repository":"not a repo"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	models, err := NewModelCatalog(root).Installed(context.Background())
	if err != nil {
		t.Fatalf("Installed() error = %v", err)
	}
	if len(models) != 1 || models[0].Repository != "" || models[0].Name != "alpha.Q4_K_M.gguf" {
		t.Fatalf("Installed() = %#v, want generic local model when metadata is malformed", models)
	}
}

func TestReportInstallProgressPercentage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		downloaded int64
		total      int64
		want       int
	}{
		{name: "unknown total", downloaded: 10, total: 0, want: 0},
		{name: "half complete", downloaded: 50, total: 100, want: 50},
		{name: "clamped", downloaded: 120, total: 100, want: 100},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var got domain.InstallProgress
			reportInstallProgress(func(progress domain.InstallProgress) { got = progress }, "model", "downloading", "model.gguf", test.downloaded, test.total, 42)
			if got.Percentage != test.want {
				t.Fatalf("percentage = %d, want %d", got.Percentage, test.want)
			}
			if got.BytesPerSecond != 42 || got.Kind != "model" {
				t.Fatalf("progress = %#v, want model progress with transfer rate", got)
			}
		})
	}
}

func TestProgressWriterReportsRecentTransferRate(t *testing.T) {
	t.Parallel()
	var progress downloadProgress
	writer := &progressWriter{
		writer:   &bytes.Buffer{},
		total:    4,
		lastEmit: time.Now().Add(-time.Second),
		report: func(update downloadProgress) {
			progress = update
		},
	}
	if _, err := writer.Write([]byte("rate")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if progress.downloadedBytes != 4 || progress.bytesPerSecond <= 0 {
		t.Fatalf("progress = %#v, want completed download with a positive transfer rate", progress)
	}
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, contents := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func checksum(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
