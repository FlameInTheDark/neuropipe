package storages

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// fakeSave is a thin alias so the validation table stays readable.
type fakeSave = domain.SaveStorageRequest

func TestCleanRemotePath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: "/", want: ""},
		{in: ".", want: ""},
		{in: "reports", want: "reports"},
		{in: "reports/", want: "reports"},
		{in: "/reports/2026/", want: "reports/2026"},
		{in: "reports//2026", want: "reports/2026"},
		{in: "reports/./2026/./chart.png", want: "reports/2026/chart.png"},
		{in: "reports\\2026\\chart.png", want: "reports/2026/chart.png"},
		{in: "C:\\data\\chart.png", want: "C:/data/chart.png"},
		{in: "reports/../secrets", wantErr: true},
		{in: "../escape", wantErr: true},
		{in: "reports/..", wantErr: true},
		{in: "a/../../b", wantErr: true},
	}
	for _, tc := range cases {
		got, err := CleanRemotePath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("CleanRemotePath(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("CleanRemotePath(%q) error = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CleanRemotePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRemotePathHelpers(t *testing.T) {
	if got := remotePrefix(""); got != "" {
		t.Errorf("remotePrefix(\"\") = %q", got)
	}
	if got := remotePrefix("reports"); got != "reports/" {
		t.Errorf("remotePrefix(\"reports\") = %q", got)
	}
	if got := entryPath("", "a.txt"); got != "a.txt" {
		t.Errorf("entryPath(\"\", \"a.txt\") = %q", got)
	}
	if got := entryPath("reports", "a.txt"); got != "reports/a.txt" {
		t.Errorf("entryPath(\"reports\", \"a.txt\") = %q", got)
	}
	if got := baseName("reports/2026/a.txt"); got != "a.txt" {
		t.Errorf("baseName() = %q", got)
	}
	if !strings.Contains(ErrInvalidPath.Error(), "..") {
		t.Errorf("ErrInvalidPath message = %q", ErrInvalidPath.Error())
	}
}

func TestBuildStorageValidation(t *testing.T) {
	cases := []struct {
		name    string
		request fakeSave
		wantErr string
	}{
		{name: "empty name", request: fakeSave{Name: "  ", Driver: "s3"}, wantErr: "storage name is required"},
		{name: "s3 without bucket", request: fakeSave{Name: "Cloud", Driver: "s3"}, wantErr: "bucket is required"},
		{name: "s3 url endpoint", request: fakeSave{Name: "Cloud", Driver: "s3", Bucket: "b", Endpoint: "https://minio.local"}, wantErr: "endpoint must be a host"},
		{name: "ftp without host", request: fakeSave{Name: "Backup", Driver: "ftp"}, wantErr: "host is required"},
		{name: "ftp bad tls", request: fakeSave{Name: "Backup", Driver: "ftp", Host: "h", TLSMode: "sometimes"}, wantErr: "unknown TLS mode"},
		{name: "ftp bad port", request: fakeSave{Name: "Backup", Driver: "ftp", Host: "h", Port: 70000}, wantErr: "port must be"},
		{name: "unknown driver", request: fakeSave{Name: "X", Driver: "webdav"}, wantErr: "unknown storage driver"},
	}
	for _, tc := range cases {
		_, err := BuildStorage(tc.request)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error = %v, want containing %q", tc.name, err, tc.wantErr)
		}
	}
	item, err := BuildStorage(fakeSave{Name: "Cloud", Driver: "s3", Bucket: "data", Region: "eu-central-1", Secure: boolPtr(true)})
	if err != nil {
		t.Fatalf("valid s3 request rejected: %v", err)
	}
	if item.Driver != domain.StorageDriverS3 || item.Bucket != "data" || item.TLSMode != domain.StorageTLSNone {
		t.Fatalf("BuildStorage() = %#v", item)
	}
	if s3Region(item) != "eu-central-1" {
		t.Errorf("s3Region() = %q", s3Region(item))
	}
	item, err = BuildStorage(fakeSave{Name: "Backup", Driver: "ftp", Host: "files.local"})
	if err != nil {
		t.Fatalf("valid ftp request rejected: %v", err)
	}
	if ftpPort(item) != 21 {
		t.Errorf("ftpPort() = %d, want 21 default", ftpPort(item))
	}
}

func boolPtr(value bool) *bool { return &value }
