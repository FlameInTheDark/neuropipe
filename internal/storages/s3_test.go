package storages

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// fakeS3 is a minimal in-process S3-compatible server: enough of the REST
// surface (bucket HEAD, object PUT/GET/HEAD/DELETE, ListObjectsV2, batch
// delete, server-side copy) to exercise the real minio client end to end.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	server  *httptest.Server
	copies  int
	deletes []string
	log     chan string
}

func newFakeS3(t *testing.T) *fakeS3 {
	f := &fakeS3{objects: map[string][]byte{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeS3) endpoint() string {
	parsed, err := url.Parse(f.server.URL)
	if err != nil {
		panic(err)
	}
	return parsed.Host
}

func (f *fakeS3) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeS3) content(key string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.objects[key]
}

func (f *fakeS3) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.objects)
}

func (f *fakeS3) handle(w http.ResponseWriter, r *http.Request) {
	if f.log != nil {
		f.log <- r.Method + " " + r.URL.String()
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key := path, ""
	if index := strings.Index(path, "/"); index >= 0 {
		bucket, key = path[:index], path[index+1:]
	}
	if bucket != "test-bucket" {
		writeS3Error(w, http.StatusNotFound, "NoSuchBucket", bucket)
		return
	}
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodHead && key == "":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && key == "" && query.Get("location") != "":
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint>us-east-1</LocationConstraint>`))
	case r.Method == http.MethodGet && key == "" && query.Get("list-type") == "2":
		f.handleList(w, query)
	case r.Method == http.MethodHead && key != "":
		f.mu.Lock()
		defer f.mu.Unlock()
		data, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		w.Header().Set("ETag", "\"etag\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && key != "":
		f.mu.Lock()
		defer f.mu.Unlock()
		data, ok := f.objects[key]
		if !ok {
			writeS3Error(w, http.StatusNotFound, "NoSuchKey", key)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		w.Header().Set("ETag", "\"etag\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	case r.Method == http.MethodPut && key != "" && r.Header.Get("x-amz-copy-source") != "":
		source := r.Header.Get("x-amz-copy-source")
		if parsed, err := url.Parse(source); err == nil && parsed.Path != "" {
			source = parsed.Path
		}
		source = strings.TrimPrefix(strings.TrimPrefix(source, "/"), "test-bucket/")
		f.mu.Lock()
		defer f.mu.Unlock()
		data, ok := f.objects[source]
		if !ok {
			writeS3Error(w, http.StatusNotFound, "NoSuchKey", source)
			return
		}
		f.objects[key] = data
		f.copies++
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<CopyObjectResult><LastModified>2026-01-01T00:00:00.000Z</LastModified><ETag>"abc"</ETag></CopyObjectResult>`))
	case r.Method == http.MethodPut && key != "":
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[key] = body
		f.mu.Unlock()
		w.Header().Set("ETag", "\"etag\"")
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodDelete && key != "":
		f.mu.Lock()
		delete(f.objects, key)
		f.deletes = append(f.deletes, key)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && query.Has("delete"):
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		for _, name := range parseMultiDeleteKeys(body) {
			delete(f.objects, name)
			f.deletes = append(f.deletes, name)
		}
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<DeleteResult></DeleteResult>`))
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// handleList implements ListObjectsV2 with delimiter grouping.
func (f *fakeS3) handleList(w http.ResponseWriter, query url.Values) {
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	f.mu.Lock()
	defer f.mu.Unlock()
	var files []s3Object
	prefixes := map[string]struct{}{}
	for key, data := range f.objects {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):]
		if delimiter != "" {
			if index := strings.Index(rest, delimiter); index >= 0 {
				prefixes[prefix+rest[:index+len(delimiter)]] = struct{}{}
				continue
			}
		}
		files = append(files, s3Object{Key: key, Size: int64(len(data)), LastModified: "2026-08-29T10:00:00.000Z"})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Key < files[j].Key })
	result := s3ListResult{
		Name:           "test-bucket",
		Prefix:         prefix,
		Delimiter:      delimiter,
		KeyCount:       len(files) + len(prefixes),
		IsTruncated:    false,
		Contents:       files,
		CommonPrefixes: s3Prefixes(prefixes),
	}
	w.Header().Set("Content-Type", "application/xml")
	encoded, _ := xml.Marshal(result)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(encoded)
}

type s3Object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

type s3CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type s3ListResult struct {
	XMLName        xml.Name `xml:"ListBucketResult"`
	Name           string   `xml:"Name"`
	Prefix         string   `xml:"Prefix"`
	KeyCount       int      `xml:"KeyCount"`
	MaxKeys        int      `xml:"MaxKeys"`
	Delimiter      string   `xml:"Delimiter"`
	IsTruncated    bool     `xml:"IsTruncated"`
	Contents       []s3Object
	CommonPrefixes []s3CommonPrefix
}

func s3Prefixes(set map[string]struct{}) []s3CommonPrefix {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]s3CommonPrefix, 0, len(names))
	for _, name := range names {
		result = append(result, s3CommonPrefix{Prefix: name})
	}
	return result
}

func parseMultiDeleteKeys(body []byte) []string {
	pattern := regexp.MustCompile(`<Key>(.*?)</Key>`)
	matches := pattern.FindAllStringSubmatch(string(body), -1)
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		if decoded, err := url.QueryUnescape(match[1]); err == nil {
			keys = append(keys, decoded)
		} else {
			keys = append(keys, match[1])
		}
	}
	return keys
}

func writeS3Error(w http.ResponseWriter, status int, code, resource string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>` + code + `</Code><Resource>` + resource + `</Resource></Error>`))
}

/* ---------------- driver tests ---------------- */

func newS3Conn(t *testing.T, fake *fakeS3) *s3Conn {
	t.Helper()
	item := domain.Storage{
		Name: "Cloud", Driver: domain.StorageDriverS3,
		Endpoint: fake.endpoint(), Bucket: "test-bucket", Secure: boolPtr(false), Region: "us-east-1",
	}
	conn, err := dialS3(t.Context(), item, "secret")
	if err != nil {
		t.Fatalf("dialS3() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn.(*s3Conn)
}

func seedS3(t *testing.T, fake *fakeS3, objects map[string]string) {
	t.Helper()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for key, content := range objects {
		fake.objects[key] = []byte(content)
	}
}

func TestS3Probe(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3Conn(t, fake)
	if err := conn.Probe(t.Context()); err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
}

func TestS3ListFoldersFirst(t *testing.T) {
	fake := newFakeS3(t)
	seedS3(t, fake, map[string]string{
		"zeta.txt":           "z",
		"reports/2025/a.csv": "aaa",
		"reports/2025/b.csv": "bbb",
		"reports/2026/c.csv": "ccc",
		"reports/2026/":      "",
		"media/logo.png":     "png",
	})
	conn := newS3Conn(t, fake)

	entries, err := conn.List(t.Context(), "")
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List(\"\") returned %d entries, want 3: %#v", len(entries), entries)
	}
	if !entries[0].IsDir || entries[0].Name != "media" || entries[0].Path != "media" {
		t.Errorf("entries[0] = %#v, want media dir", entries[0])
	}
	if !entries[1].IsDir || entries[1].Name != "reports" {
		t.Errorf("entries[1] = %#v, want reports dir", entries[1])
	}
	if entries[2].IsDir || entries[2].Name != "zeta.txt" || entries[2].Size != 1 {
		t.Errorf("entries[2] = %#v, want zeta.txt file", entries[2])
	}

	entries, err = conn.List(t.Context(), "reports/2026")
	if err != nil {
		t.Fatalf("List(\"reports/2026\") error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "c.csv" || entries[0].Size != 3 {
		t.Fatalf("List(\"reports/2026\") = %#v, want single c.csv", entries)
	}
}

func TestS3UploadAndDownload(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3Conn(t, fake)

	local := t.TempDir() + "/chart.png"
	if err := os.WriteFile(local, []byte("PNGDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploaded, err := conn.UploadFile(t.Context(), local, "reports/", "")
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if uploaded.Key != "reports/chart.png" || uploaded.Size != 7 || uploaded.Driver != "s3" {
		t.Fatalf("UploadFile() = %#v", uploaded)
	}
	if string(fake.content("reports/chart.png")) != "PNGDATA" {
		t.Fatalf("stored content = %q", fake.content("reports/chart.png"))
	}

	data, err := conn.UploadData(t.Context(), []byte("hello"), "notes/greeting.txt", "text/plain")
	if err != nil {
		t.Fatalf("UploadData() error = %v", err)
	}
	if data.Key != "notes/greeting.txt" || data.Size != 5 {
		t.Fatalf("UploadData() = %#v", data)
	}

	target := t.TempDir() + "/downloaded/"
	downloaded, err := conn.Download(t.Context(), "notes/greeting.txt", target)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if downloaded.Name != "greeting.txt" || downloaded.Bytes != 5 {
		t.Fatalf("Download() = %#v", downloaded)
	}
	content, err := os.ReadFile(downloaded.Path)
	if err != nil || string(content) != "hello" {
		t.Fatalf("downloaded file content = %q, %v", content, err)
	}
}

func TestS3DownloadMissing(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3Conn(t, fake)
	if _, err := conn.Download(t.Context(), "nope.txt", t.TempDir()+"/nope.txt"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Download(missing) error = %v", err)
	}
}

func TestS3DeleteFileFolderAndMarker(t *testing.T) {
	fake := newFakeS3(t)
	seedS3(t, fake, map[string]string{
		"logs/app.log":   "line",
		"logs/old/1.log": "1",
		"logs/old/2.log": "2",
		"logs/old/":      "",
		"readme.md":      "hi",
		"solo/":          "",
	})
	conn := newS3Conn(t, fake)

	// plain file
	result, err := conn.Delete(t.Context(), "readme.md", false)
	if err != nil || !result.Deleted || result.Count != 1 {
		t.Fatalf("Delete(file) = %#v, %v", result, err)
	}
	// folder without recursive
	if _, err := conn.Delete(t.Context(), "logs", false); err == nil || !strings.Contains(err.Error(), "recursive") {
		t.Fatalf("Delete(folder, false) error = %v", err)
	}
	// folder with recursive
	result, err = conn.Delete(t.Context(), "logs", true)
	if err != nil || !result.Deleted || result.Count != 4 {
		t.Fatalf("Delete(folder, true) = %#v, %v", result, err)
	}
	if fake.count() != 1 {
		t.Fatalf("remaining objects = %d (%v)", fake.count(), fake.remaining())
	}
	// lone marker
	result, err = conn.Delete(t.Context(), "solo", false)
	if err != nil || !result.Deleted || result.Count != 1 {
		t.Fatalf("Delete(marker) = %#v, %v", result, err)
	}
	if fake.count() != 0 {
		t.Fatalf("objects left after marker delete: %v", fake.remaining())
	}
	// missing
	if _, err := conn.Delete(t.Context(), "ghost", false); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Delete(missing) error = %v", err)
	}
}

func TestS3MakeDirAndMove(t *testing.T) {
	fake := newFakeS3(t)
	seedS3(t, fake, map[string]string{"draft.csv": "a,b,c"})
	conn := newS3Conn(t, fake)

	made, err := conn.MakeDir(t.Context(), "reports")
	if err != nil || !made.Created {
		t.Fatalf("MakeDir() = %#v, %v", made, err)
	}
	if !fake.has("reports/") {
		t.Fatal("folder marker reports/ missing")
	}

	moved, err := conn.Move(t.Context(), "draft.csv", "reports/final.csv")
	if err != nil || !moved.Moved {
		t.Fatalf("Move() = %#v, %v", moved, err)
	}
	if fake.has("draft.csv") || !fake.has("reports/final.csv") {
		t.Fatalf("move left draft=%v final=%v", fake.has("draft.csv"), fake.has("reports/final.csv"))
	}

	// moving a populated folder prefix is rejected
	seedS3(t, fake, map[string]string{"folder/inner.txt": "x"})
	if _, err := conn.Move(t.Context(), "folder", "folder2"); err == nil || !strings.Contains(err.Error(), "moving folders") {
		t.Fatalf("Move(folder) error = %v", err)
	}
	// moving a missing object
	if _, err := conn.Move(t.Context(), "ghost.csv", "anywhere"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Move(missing) error = %v", err)
	}
}

func (f *fakeS3) remaining() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.objects))
	for key := range f.objects {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}
