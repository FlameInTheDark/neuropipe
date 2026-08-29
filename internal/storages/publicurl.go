// Public URL construction and presigned-URL service entry points. Public URLs
// are pure metadata lookups — the connection's settings (driver, endpoint,
// bucket, public base URL) map a storage-relative path to an address without
// any network round-trip, so URLs can be built for objects that are not even
// uploaded yet. Presigning needs the driver's credentials, so it resolves the
// cached connection and signs offline.
package storages

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// storagePresigner is implemented by drivers that can sign temporary URLs
// (S3 today). FTP has no presigning equivalent, so asking for one produces an
// explicit error instead of a silently broken link.
type storagePresigner interface {
	Presign(ctx context.Context, request domain.StoragePresignRequest) (domain.StoragePresignResult, error)
}

// StoragePresignURL generates a temporary signed URL for one object. The
// signature is computed locally against the stored credentials; no request
// reaches the server. FTP storages are rejected before any dialing because
// FTP has no presigned-URL equivalent.
func (s *Service) StoragePresignURL(ctx context.Context, request domain.StoragePresignRequest) (domain.StoragePresignResult, error) {
	item, err := s.Get(ctx, request.StorageID)
	if err != nil {
		return domain.StoragePresignResult{}, err
	}
	if item.Driver != domain.StorageDriverS3 {
		return domain.StoragePresignResult{}, fmt.Errorf("presigned URLs are only available for S3 storages")
	}
	path, err := CleanRemotePath(request.Path)
	if err != nil {
		return domain.StoragePresignResult{}, err
	}
	if path == "" {
		return domain.StoragePresignResult{}, fmt.Errorf("path is required")
	}
	request.Path = path
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StoragePresignResult{}, err
	}
	presigner, ok := conn.(storagePresigner)
	if !ok {
		return domain.StoragePresignResult{}, fmt.Errorf("presigned URLs are only available for S3 storages")
	}
	return presigner.Presign(ctx, request)
}

// StoragePublicURL builds the public address of one remote file or folder:
// the connection's public base URL when configured, the direct S3 object
// address otherwise, and a best-effort ftp(s):// URL for FTP storages
// without a base.
func (s *Service) StoragePublicURL(ctx context.Context, request domain.StoragePublicURLRequest) (domain.StoragePublicURLResult, error) {
	item, err := s.Get(ctx, request.StorageID)
	if err != nil {
		return domain.StoragePublicURLResult{}, err
	}
	path, err := CleanRemotePath(request.Path)
	if err != nil {
		return domain.StoragePublicURLResult{}, err
	}
	if item.PublicBaseURL != "" {
		return domain.StoragePublicURLResult{URL: joinPublicBase(item.PublicBaseURL, path), Kind: "public-base"}, nil
	}
	switch item.Driver {
	case domain.StorageDriverS3:
		return domain.StoragePublicURLResult{URL: s3PublicURL(item, path), Kind: "s3"}, nil
	case domain.StorageDriverFTP:
		return domain.StoragePublicURLResult{URL: ftpPublicURL(item, path), Kind: "ftp"}, nil
	default:
		return domain.StoragePublicURLResult{}, fmt.Errorf("unknown storage driver %q", item.Driver)
	}
}

// joinPublicBase maps a storage-relative path onto the configured public base
// URL. The base is interpreted as serving the storage root exactly as the
// browser presents it (already inside the FTP base folder).
func joinPublicBase(base, path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if path == "" {
		return trimmed + "/"
	}
	return trimmed + "/" + escapeRemotePath(path)
}

// escapeRemotePath percent-escapes every path segment while keeping the "/"
// separators, so keys with spaces or unicode round-trip as valid URLs.
func escapeRemotePath(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

// s3PublicURL returns the direct object address, mirroring the client's own
// addressing: virtual-host style for AWS (automatic lookup) and path style
// for custom endpoints (forced by dialS3 to avoid wildcard-certificate
// collisions on MinIO/R2-style servers).
func s3PublicURL(item domain.Storage, path string) string {
	escaped := escapeRemotePath(path)
	if item.Endpoint == "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", item.Bucket, s3Region(item), escaped)
	}
	scheme := "https"
	if item.Secure != nil && !*item.Secure {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, strings.TrimRight(item.Endpoint, "/"), item.Bucket, escaped)
}

// ftpPublicURL builds a best-effort protocol URL. FTP has no public HTTP
// addressing, so without a public base URL the result is a shareable ftp(s)://
// reference including the base folder (most clients and curl open these;
// browsers no longer do). Credentials are never embedded.
func ftpPublicURL(item domain.Storage, path string) string {
	scheme := "ftp"
	defaultPort := 21
	if item.TLSMode == domain.StorageTLSImplicit {
		scheme = "ftps"
		defaultPort = 990
	}
	host := item.Host
	if port := ftpPort(item); port != defaultPort {
		host = fmt.Sprintf("%s:%d", item.Host, port)
	}
	realPath := path
	if item.BaseDir != "" {
		if path == "" {
			realPath = item.BaseDir
		} else {
			realPath = item.BaseDir + "/" + path
		}
	}
	return fmt.Sprintf("%s://%s/%s", scheme, host, escapeRemotePath(realPath))
}
