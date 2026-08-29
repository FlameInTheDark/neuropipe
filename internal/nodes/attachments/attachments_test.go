package attachments

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/report.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4 fake"))
		case "/raw":
			_, _ = w.Write([]byte{0x00, 0x01, 0x02, 0x03})
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	loaded, err := Load(context.Background(), Sources{URLs: server.URL + "/report.pdf"}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if loaded[0].Name != "report.pdf" || loaded[0].ContentType != "application/pdf" || string(loaded[0].Data) != "%PDF-1.4 fake" {
		t.Fatalf("attachment = %#v", loaded[0])
	}

	loaded, err = Load(context.Background(), Sources{URLs: server.URL + "/raw"}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].ContentType != "application/octet-stream" {
		t.Fatalf("fallback content type = %q", loaded[0].ContentType)
	}

	if _, err := Load(context.Background(), Sources{URLs: server.URL + "/missing"}, Limits{}); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("404 error = %v", err)
	}
	if _, err := Load(context.Background(), Sources{URLs: "not a url"}, Limits{}); err == nil || !strings.Contains(err.Error(), "not a valid absolute URL") {
		t.Fatalf("invalid url error = %v", err)
	}
}

func TestLoadFromMultipleURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data for " + r.URL.Path))
	}))
	defer server.Close()

	sources := Sources{URLs: server.URL + "/a.txt\n" + server.URL + "/b.txt\n\n"}
	loaded, err := Load(context.Background(), sources, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d attachments, want 2", len(loaded))
	}
	if loaded[0].Name != "a.txt" || loaded[1].Name != "b.txt" {
		t.Fatalf("names = %q, %q", loaded[0].Name, loaded[1].Name)
	}
}

func TestLoadFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello file"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(context.Background(), Sources{Paths: path}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].Name != "notes.txt" || loaded[0].ContentType != "text/plain; charset=utf-8" && loaded[0].ContentType != "text/plain" {
		t.Fatalf("attachment = %#v", loaded[0])
	}
	if string(loaded[0].Data) != "hello file" {
		t.Fatalf("data = %q", loaded[0].Data)
	}
	if _, err := Load(context.Background(), Sources{Paths: filepath.Join(dir, "missing.bin")}, Limits{}); err == nil {
		t.Fatal("missing path accepted")
	}
	if _, err := Load(context.Background(), Sources{Paths: dir}, Limits{}); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("directory error = %v", err)
	}
}

func TestLoadDataBytesAndBase64(t *testing.T) {
	payload := []byte("binary payload")
	loaded, err := Load(context.Background(), Sources{Data: payload, DataName: "out.png"}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].Name != "out.png" || string(loaded[0].Data) != "binary payload" || loaded[0].ContentType != "image/png" {
		t.Fatalf("attachment = %#v", loaded[0])
	}

	encoded := base64.StdEncoding.EncodeToString(payload)
	loaded, err = Load(context.Background(), Sources{Data: encoded, DataName: "out.png"}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded[0].Data) != "binary payload" {
		t.Fatalf("base64 data = %q", loaded[0].Data)
	}

	loaded, err = Load(context.Background(), Sources{Data: "data:image/png;base64," + encoded, DataName: ""}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].Name != "file.bin" || string(loaded[0].Data) != "binary payload" {
		t.Fatalf("data url attachment = %#v", loaded[0])
	}

	if _, err := Load(context.Background(), Sources{Data: "!!!not base64!!!"}, Limits{}); err == nil {
		t.Fatal("invalid base64 accepted")
	}
	if _, err := Load(context.Background(), Sources{Data: 42}, Limits{}); err == nil {
		t.Fatal("numeric data accepted")
	}
	if _, err := Load(context.Background(), Sources{Data: ""}, Limits{}); err == nil {
		t.Fatal("empty string data accepted")
	}
}

func TestSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 2048))
	}))
	defer server.Close()

	limits := Limits{MaxBytes: 1024}
	if _, err := Load(context.Background(), Sources{URLs: server.URL + "/big.bin"}, limits); err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("url size error = %v", err)
	}
	if _, err := Load(context.Background(), Sources{Data: make([]byte, 2048), DataName: "big.bin"}, limits); err == nil {
		t.Fatal("data size accepted")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(path, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(context.Background(), Sources{Paths: path}, limits); err == nil {
		t.Fatal("path size accepted")
	}
}

func TestCountLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "x")
	}))
	defer server.Close()
	sources := Sources{URLs: server.URL + "/1.txt\n" + server.URL + "/2.txt"}
	if _, err := Load(context.Background(), sources, Limits{MaxCount: 1}); err == nil || !strings.Contains(err.Error(), "too many attachments") {
		t.Fatalf("count error = %v", err)
	}
	if loaded, err := Load(context.Background(), sources, Limits{MaxCount: 2}); err != nil || len(loaded) != 2 {
		t.Fatalf("within count: %v %v", loaded, err)
	}
}

func TestEmptySources(t *testing.T) {
	loaded, err := Load(context.Background(), Sources{}, Limits{MaxBytes: 10, MaxCount: 2})
	if err != nil || loaded != nil {
		t.Fatalf("empty sources = %#v, %v", loaded, err)
	}
	loaded, err = Load(context.Background(), Sources{URLs: "  \n ", Paths: ""}, Limits{})
	if err != nil || loaded != nil {
		t.Fatalf("whitespace sources = %#v, %v", loaded, err)
	}
}
