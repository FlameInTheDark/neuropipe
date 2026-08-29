package storages

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// newS3AuthConn dials an S3 connection with real credentials so presigning
// can sign with them. Signing itself is offline — the fake server only gives
// the URL a realistic endpoint.
func newS3AuthConn(t *testing.T, fake *fakeS3) *s3Conn {
	t.Helper()
	item := domain.Storage{
		Name: "Cloud", Driver: domain.StorageDriverS3,
		Endpoint: fake.endpoint(), Bucket: "test-bucket", Secure: boolPtr(false), Region: "us-east-1",
		AccessKey: "AKIAEXAMPLE",
	}
	conn, err := dialS3(t.Context(), item, "topsecret")
	if err != nil {
		t.Fatalf("dialS3() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn.(*s3Conn)
}

func TestS3PresignGetURLShape(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3AuthConn(t, fake)

	result, err := conn.Presign(t.Context(), domain.StoragePresignRequest{
		Path: "reports/2026/chart.png", Method: "GET", ExpiresSeconds: 900,
	})
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatalf("presigned URL does not parse: %v", err)
	}
	if parsed.Scheme != "http" || parsed.Host != fake.endpoint() {
		t.Fatalf("presigned URL base = %s://%s, want the fake endpoint", parsed.Scheme, parsed.Host)
	}
	if !strings.HasPrefix(parsed.Path, "/test-bucket/reports/2026/chart.png") {
		t.Fatalf("presigned URL path = %q", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		t.Fatalf("X-Amz-Algorithm = %q", query.Get("X-Amz-Algorithm"))
	}
	if query.Get("X-Amz-Expires") != "900" {
		t.Fatalf("X-Amz-Expires = %q", query.Get("X-Amz-Expires"))
	}
	if !strings.HasPrefix(query.Get("X-Amz-Credential"), "AKIAEXAMPLE/") {
		t.Fatalf("X-Amz-Credential = %q", query.Get("X-Amz-Credential"))
	}
	if query.Get("X-Amz-Signature") == "" {
		t.Fatal("X-Amz-Signature is empty")
	}
	if result.Method != "GET" || result.ExpiresInSeconds != 900 {
		t.Fatalf("Presign() = %#v", result)
	}
	expiresAt, err := time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		t.Fatalf("ExpiresAt does not parse: %v", err)
	}
	window := time.Until(expiresAt)
	if window < 850*time.Second || window > 950*time.Second {
		t.Fatalf("ExpiresAt window = %v, want ~900s", window)
	}
}

func TestS3PresignSignsRequiredHeaders(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3AuthConn(t, fake)

	result, err := conn.Presign(t.Context(), domain.StoragePresignRequest{
		Path:   "img.png",
		Method: "put", // lower-case normalizes
		Headers: map[string]string{
			"content-type":   "image/png",
			"x-amz-meta-who": "neuropipe",
		},
	})
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}
	if result.Method != "PUT" {
		t.Fatalf("Method = %q, want PUT", result.Method)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatalf("presigned URL does not parse: %v", err)
	}
	signed := parsed.Query().Get("X-Amz-SignedHeaders")
	for _, header := range []string{"host", "content-type", "x-amz-meta-who"} {
		if !strings.Contains(signed, header) {
			t.Fatalf("X-Amz-SignedHeaders = %q, want %q signed in", signed, header)
		}
	}
	// The echoed headers are canonicalized so consumers copy them verbatim.
	if result.Headers["Content-Type"] != "image/png" || result.Headers["X-Amz-Meta-Who"] != "neuropipe" {
		t.Fatalf("echoed headers = %#v", result.Headers)
	}
}

func TestS3PresignSignsQueryParams(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3AuthConn(t, fake)

	disposition := `attachment; filename="chart.png"`
	result, err := conn.Presign(t.Context(), domain.StoragePresignRequest{
		Path:   "reports/chart.png",
		Method: "GET",
		Params: map[string]string{"response-content-disposition": disposition},
	})
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatalf("presigned URL does not parse: %v", err)
	}
	if got := parsed.Query().Get("response-content-disposition"); got != disposition {
		t.Fatalf("response-content-disposition = %q", got)
	}
	if result.Params["response-content-disposition"] != disposition {
		t.Fatalf("echoed params = %#v", result.Params)
	}
}

func TestS3PresignMethodsProduceDifferentSignatures(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3AuthConn(t, fake)

	signatures := map[string]string{}
	for _, method := range []string{"GET", "PUT", "HEAD", "DELETE"} {
		result, err := conn.Presign(t.Context(), domain.StoragePresignRequest{
			Path: "same/object.png", Method: method, ExpiresSeconds: 300,
		})
		if err != nil {
			t.Fatalf("Presign(%s) error = %v", method, err)
		}
		parsed, err := url.Parse(result.URL)
		if err != nil {
			t.Fatalf("Presign(%s) URL does not parse: %v", method, err)
		}
		signature := parsed.Query().Get("X-Amz-Signature")
		if signature == "" {
			t.Fatalf("Presign(%s) produced an empty signature", method)
		}
		if _, clash := signatures[signature]; clash {
			t.Fatalf("methods %s produced identical signatures", method)
		}
		signatures[signature] = method
	}
	if len(signatures) != 4 {
		t.Fatalf("expected 4 distinct signatures, got %d", len(signatures))
	}
}

func TestS3PresignDefaultsAndValidation(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3AuthConn(t, fake)

	// Blank expiry falls back to one hour.
	result, err := conn.Presign(t.Context(), domain.StoragePresignRequest{Path: "a.txt", Method: "GET"})
	if err != nil {
		t.Fatalf("Presign(default) error = %v", err)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatalf("presigned URL does not parse: %v", err)
	}
	if parsed.Query().Get("X-Amz-Expires") != "3600" {
		t.Fatalf("default X-Amz-Expires = %q", parsed.Query().Get("X-Amz-Expires"))
	}

	for name, request := range map[string]domain.StoragePresignRequest{
		"bad method":    {Path: "a.txt", Method: "POST"},
		"negative":      {Path: "a.txt", Method: "GET", ExpiresSeconds: -5},
		"beyond 7 days": {Path: "a.txt", Method: "GET", ExpiresSeconds: 604801},
		"blank header":  {Path: "a.txt", Method: "GET", Headers: map[string]string{" ": "x"}},
		"blank param":   {Path: "a.txt", Method: "GET", Params: map[string]string{"": "x"}},
	} {
		if _, err := conn.Presign(t.Context(), request); err == nil {
			t.Fatalf("Presign(%s) expected an error", name)
		}
	}
}

func TestS3PresignAnonymousConnectionRejected(t *testing.T) {
	fake := newFakeS3(t)
	conn := newS3Conn(t, fake) // no access key / secret
	if _, err := conn.Presign(t.Context(), domain.StoragePresignRequest{Path: "a.txt", Method: "GET"}); err == nil || !strings.Contains(err.Error(), "anonymous") {
		t.Fatalf("Presign(anonymous) error = %v", err)
	}
}

func TestValidStoragePresignMethod(t *testing.T) {
	for _, method := range []string{"GET", "get", " PUT ", "HEAD", "DELETE"} {
		if !ValidStoragePresignMethod(method) {
			t.Fatalf("ValidStoragePresignMethod(%q) = false", method)
		}
	}
	for _, method := range []string{"", "POST", "PATCH", "OPTIONS", "CONNECT"} {
		if ValidStoragePresignMethod(method) {
			t.Fatalf("ValidStoragePresignMethod(%q) = true", method)
		}
	}
}
