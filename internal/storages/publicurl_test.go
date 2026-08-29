package storages

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

/* ---------------- pure URL construction ---------------- */

func TestS3PublicURLShapes(t *testing.T) {
	tests := []struct {
		name string
		item domain.Storage
		path string
		want string
	}{
		{
			name: "aws virtual-host with region",
			item: domain.Storage{Driver: domain.StorageDriverS3, Bucket: "my-bucket", Region: "eu-central-1"},
			path: "reports/chart.png",
			want: "https://my-bucket.s3.eu-central-1.amazonaws.com/reports/chart.png",
		},
		{
			name: "aws default region",
			item: domain.Storage{Driver: domain.StorageDriverS3, Bucket: "b"},
			path: "a.txt",
			want: "https://b.s3.us-east-1.amazonaws.com/a.txt",
		},
		{
			name: "custom endpoint path style https",
			item: domain.Storage{Driver: domain.StorageDriverS3, Endpoint: "s3.example.com:9000", Bucket: "data"},
			path: "x/y.bin",
			want: "https://s3.example.com:9000/data/x/y.bin",
		},
		{
			name: "custom endpoint plain http",
			item: domain.Storage{Driver: domain.StorageDriverS3, Endpoint: "localhost:9000", Bucket: "data", Secure: boolPtr(false)},
			path: "x.bin",
			want: "http://localhost:9000/data/x.bin",
		},
		{
			name: "spaces and unicode are escaped per segment",
			item: domain.Storage{Driver: domain.StorageDriverS3, Bucket: "b"},
			path: "my file/ünïcode.png",
			want: "https://b.s3.us-east-1.amazonaws.com/my%20file/%C3%BCn%C3%AFcode.png",
		},
		{
			name: "root object",
			item: domain.Storage{Driver: domain.StorageDriverS3, Bucket: "b"},
			path: "",
			want: "https://b.s3.us-east-1.amazonaws.com/",
		},
	}
	for _, test := range tests {
		if got := s3PublicURL(test.item, test.path); got != test.want {
			t.Fatalf("%s: s3PublicURL() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestFTPPublicURLShapes(t *testing.T) {
	tests := []struct {
		name string
		item domain.Storage
		path string
		want string
	}{
		{
			name: "plain default port hidden",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com"},
			path: "pub/report.pdf",
			want: "ftp://files.example.com/pub/report.pdf",
		},
		{
			name: "custom port included",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com", Port: 2121},
			path: "a.txt",
			want: "ftp://files.example.com:2121/a.txt",
		},
		{
			name: "base dir prefixes the path",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com", BaseDir: "uploads"},
			path: "2026/a.csv",
			want: "ftp://files.example.com/uploads/2026/a.csv",
		},
		{
			name: "base dir root",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com", BaseDir: "uploads"},
			path: "",
			want: "ftp://files.example.com/uploads",
		},
		{
			name: "implicit tls keeps the dialed port",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "secure.example.com", TLSMode: domain.StorageTLSImplicit},
			path: "a.txt",
			want: "ftps://secure.example.com:21/a.txt",
		},
		{
			name: "implicit tls custom port",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "secure.example.com", Port: 990, TLSMode: domain.StorageTLSImplicit},
			path: "a.txt",
			want: "ftps://secure.example.com/a.txt",
		},
		{
			name: "explicit tls stays ftp scheme",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com", Port: 21, TLSMode: domain.StorageTLSExplicit},
			path: "a.txt",
			want: "ftp://files.example.com/a.txt",
		},
		{
			name: "spaces escaped",
			item: domain.Storage{Driver: domain.StorageDriverFTP, Host: "files.example.com"},
			path: "my file.txt",
			want: "ftp://files.example.com/my%20file.txt",
		},
	}
	for _, test := range tests {
		if got := ftpPublicURL(test.item, test.path); got != test.want {
			t.Fatalf("%s: ftpPublicURL() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestJoinPublicBase(t *testing.T) {
	tests := []struct{ base, path, want string }{
		{"https://cdn.example.com", "a/b.png", "https://cdn.example.com/a/b.png"},
		{"https://cdn.example.com/", "a/b.png", "https://cdn.example.com/a/b.png"},
		{"https://cdn.example.com/assets/", "a b.png", "https://cdn.example.com/assets/a%20b.png"},
		{"https://cdn.example.com", "", "https://cdn.example.com/"},
	}
	for _, test := range tests {
		if got := joinPublicBase(test.base, test.path); got != test.want {
			t.Fatalf("joinPublicBase(%q, %q) = %q, want %q", test.base, test.path, got, test.want)
		}
	}
}

/* ---------------- build validation ---------------- */

func TestBuildStoragePublicBaseURL(t *testing.T) {
	base := func(value string) domain.SaveStorageRequest {
		return domain.SaveStorageRequest{Name: "Cloud", Driver: domain.StorageDriverS3, Bucket: "b", PublicBaseURL: value}
	}
	valid, err := BuildStorage(base("https://cdn.example.com/assets/"))
	if err != nil {
		t.Fatalf("BuildStorage() error = %v", err)
	}
	if valid.PublicBaseURL != "https://cdn.example.com/assets" {
		t.Fatalf("PublicBaseURL = %q, want trailing slash trimmed", valid.PublicBaseURL)
	}
	for _, invalid := range []string{"cdn.example.com", "ftp://x", "https://user:pass@x", "https://example.com/../etc"} {
		if _, err := BuildStorage(base(invalid)); err == nil {
			t.Fatalf("BuildStorage(%q) expected an error", invalid)
		}
	}
}

/* ---------------- service level ---------------- */

func newURLTestService(t *testing.T) *Service {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := New(store, nil)
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestServicePublicURL(t *testing.T) {
	service := newURLTestService(t)
	ctx := t.Context()

	cloud, err := service.Register(ctx, domain.SaveStorageRequest{Name: "Cloud", Driver: domain.StorageDriverS3, Bucket: "my-bucket", Region: "eu-central-1"})
	if err != nil {
		t.Fatalf("Register(S3) error = %v", err)
	}
	result, err := service.StoragePublicURL(ctx, domain.StoragePublicURLRequest{StorageID: cloud.ID, Path: "reports/chart.png"})
	if err != nil {
		t.Fatalf("StoragePublicURL(S3) error = %v", err)
	}
	if result.URL != "https://my-bucket.s3.eu-central-1.amazonaws.com/reports/chart.png" || result.Kind != "s3" {
		t.Fatalf("StoragePublicURL(S3) = %#v", result)
	}

	backup, err := service.Register(ctx, domain.SaveStorageRequest{Name: "Backup", Driver: domain.StorageDriverFTP, Host: "files.invalid"})
	if err != nil {
		t.Fatalf("Register(FTP) error = %v", err)
	}
	result, err = service.StoragePublicURL(ctx, domain.StoragePublicURLRequest{StorageID: backup.ID, Path: "report.pdf"})
	if err != nil {
		t.Fatalf("StoragePublicURL(FTP) error = %v", err)
	}
	if result.URL != "ftp://files.invalid/report.pdf" || result.Kind != "ftp" {
		t.Fatalf("StoragePublicURL(FTP) = %#v", result)
	}

	mirror, err := service.Register(ctx, domain.SaveStorageRequest{Name: "Mirror", Driver: domain.StorageDriverS3, Bucket: "my-bucket", Endpoint: "minio.invalid:9000", PublicBaseURL: "https://cdn.example.com"})
	if err != nil {
		t.Fatalf("Register(mirror) error = %v", err)
	}
	result, err = service.StoragePublicURL(ctx, domain.StoragePublicURLRequest{StorageID: mirror.ID, Path: "reports/../secret"})
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("StoragePublicURL(traversal) error = %v", err)
	}
	result, err = service.StoragePublicURL(ctx, domain.StoragePublicURLRequest{StorageID: mirror.ID, Path: "reports/chart.png"})
	if err != nil {
		t.Fatalf("StoragePublicURL(mirror) error = %v", err)
	}
	if result.URL != "https://cdn.example.com/reports/chart.png" || result.Kind != "public-base" {
		t.Fatalf("StoragePublicURL(mirror) = %#v", result)
	}
}

func TestServicePresignRejectsFTPAndAnonymousS3(t *testing.T) {
	service := newURLTestService(t)
	ctx := t.Context()

	backup, err := service.Register(ctx, domain.SaveStorageRequest{Name: "Backup", Driver: domain.StorageDriverFTP, Host: "presign.invalid"})
	if err != nil {
		t.Fatalf("Register(FTP) error = %v", err)
	}
	if _, err := service.StoragePresignURL(ctx, domain.StoragePresignRequest{StorageID: backup.ID, Path: "a.txt", Method: "GET"}); err == nil || !strings.Contains(err.Error(), "only available for S3") {
		t.Fatalf("StoragePresignURL(FTP) error = %v", err)
	}

	// An S3 registration without secrets still succeeds (probe just fails);
	// presigning then names the missing credentials explicitly.
	anonymous, err := service.Register(ctx, domain.SaveStorageRequest{Name: "Anon", Driver: domain.StorageDriverS3, Bucket: "public-bucket", AccessKey: "AKIAEXAMPLE"})
	if err != nil {
		t.Fatalf("Register(anon S3) error = %v", err)
	}
	if _, err := service.StoragePresignURL(ctx, domain.StoragePresignRequest{StorageID: anonymous.ID, Path: "a.txt", Method: "GET"}); err == nil || !strings.Contains(err.Error(), "anonymous") {
		t.Fatalf("StoragePresignURL(anon) error = %v", err)
	}
}
