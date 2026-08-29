package storages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Presign expiry bounds mirror the SigV4 presigned-URL window: one second to
// one week (604800 seconds).
const (
	presignMinSeconds int64 = 1
	presignMaxSeconds int64 = 7 * 24 * 60 * 60
)

// dialS3 builds an S3-compatible connection. A blank endpoint targets AWS S3
// itself; a custom endpoint works with MinIO, Cloudflare R2, Wasabi, Backblaze
// B2 and every other S3-compatible server (path-style addressing).
func dialS3(_ context.Context, item domain.Storage, secret string) (storageConn, error) {
	endpoint := item.Endpoint
	secure := item.Secure == nil || *item.Secure
	lookup := minio.BucketLookupAuto
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
		secure = true
	} else {
		// Custom endpoints (R2, MinIO) need path-style so bucket names in the
		// host position cannot collide with wildcard certificates.
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(item.AccessKey, secret, ""),
		Secure:       secure,
		Region:       s3Region(item),
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}
	return &s3Conn{client: client, item: item, canPresign: item.AccessKey != "" && secret != ""}, nil
}

// s3Conn adapts the minio client to the storageConn surface. The client is
// safe for concurrent use, so no extra locking is needed.
type s3Conn struct {
	client     *minio.Client
	item       domain.Storage
	canPresign bool // access key + secret are both present
}

func (c *s3Conn) Close() error { return nil }

// Probe verifies the bucket is reachable and authorized.
func (c *s3Conn) Probe(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.item.Bucket)
	if err != nil {
		return fmt.Errorf("reach S3 bucket %q: %w", c.item.Bucket, err)
	}
	if !exists {
		return fmt.Errorf("S3 bucket %q does not exist or is not accessible", c.item.Bucket)
	}
	return nil
}

func (c *s3Conn) List(ctx context.Context, dir string) ([]domain.StorageEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list storage cancelled: %w", err)
	}
	prefix := remotePrefix(dir)
	entries := make([]domain.StorageEntry, 0)
	objectCh := c.client.ListObjects(ctx, c.item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: false})
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("list %q: %w", dir, object.Err)
		}
		key := strings.TrimSuffix(object.Key, "/")
		if key == dir { // the directory's own marker object
			continue
		}
		if strings.HasSuffix(object.Key, "/") {
			entries = append(entries, domain.StorageEntry{Name: baseName(key), Path: key, IsDir: true})
			continue
		}
		modTime := time.Time{}
		if !object.LastModified.IsZero() {
			modTime = object.LastModified
		}
		entries = append(entries, domain.StorageEntry{Name: baseName(key), Path: key, Size: object.Size, ModTime: modTime})
	}
	sortEntries(entries)
	return entries, nil
}

func (c *s3Conn) UploadFile(ctx context.Context, localPath, remotePath, contentType string) (domain.StorageUploadResult, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return domain.StorageUploadResult{}, fmt.Errorf("read local file: %w", err)
	}
	if info.IsDir() {
		return domain.StorageUploadResult{}, fmt.Errorf("local path is a directory, not a file")
	}
	if remotePath == "" || strings.HasSuffix(remotePath, "/") {
		remotePath = strings.TrimSuffix(remotePath, "/") + "/" + filepath.Base(localPath)
	}
	if contentType == "" {
		contentType, err = detectFileContentType(localPath)
		if err != nil {
			return domain.StorageUploadResult{}, err
		}
	}
	file, err := os.Open(localPath)
	if err != nil {
		return domain.StorageUploadResult{}, fmt.Errorf("open local file: %w", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := c.client.PutObject(ctx, c.item.Bucket, remotePath, file, info.Size(), minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return domain.StorageUploadResult{}, fmt.Errorf("upload to S3: %w", err)
	}
	return domain.StorageUploadResult{Key: remotePath, Size: info.Size(), Driver: string(domain.StorageDriverS3)}, nil
}

func (c *s3Conn) UploadData(ctx context.Context, data []byte, remotePath, contentType string) (domain.StorageUploadResult, error) {
	if remotePath == "" {
		return domain.StorageUploadResult{}, fmt.Errorf("remote path is required")
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if _, err := c.client.PutObject(ctx, c.item.Bucket, remotePath, bytesReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return domain.StorageUploadResult{}, fmt.Errorf("upload to S3: %w", err)
	}
	return domain.StorageUploadResult{Key: remotePath, Size: int64(len(data)), Driver: string(domain.StorageDriverS3)}, nil
}

func (c *s3Conn) Download(ctx context.Context, remotePath, localPath string) (domain.StorageDownloadResult, error) {
	object, err := c.client.GetObject(ctx, c.item.Bucket, remotePath, minio.GetObjectOptions{})
	if err != nil {
		return domain.StorageDownloadResult{}, fmt.Errorf("open S3 object: %w", err)
	}
	defer func() { _ = object.Close() }()
	if _, err := object.Stat(); err != nil {
		// Stat surfaces 404s that GetObject defers until read.
		var resp minio.ErrorResponse
		if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
			return domain.StorageDownloadResult{}, fmt.Errorf("remote file %q does not exist", remotePath)
		}
		return domain.StorageDownloadResult{}, fmt.Errorf("open S3 object: %w", err)
	}
	if localPath == "" || strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, string(os.PathSeparator)) {
		localPath = strings.TrimRight(localPath, "/\\") + string(os.PathSeparator) + baseName(remotePath)
	}
	if err := ensureParent(localPath); err != nil {
		return domain.StorageDownloadResult{}, err
	}
	file, err := os.Create(localPath)
	if err != nil {
		return domain.StorageDownloadResult{}, fmt.Errorf("create local file: %w", err)
	}
	defer func() { _ = file.Close() }()
	written, err := io.Copy(file, object)
	if err != nil {
		return domain.StorageDownloadResult{}, fmt.Errorf("download from S3: %w", err)
	}
	return domain.StorageDownloadResult{Path: localPath, Name: baseName(remotePath), Bytes: written}, nil
}

func (c *s3Conn) Delete(ctx context.Context, target string, recursive bool) (domain.StorageDeleteResult, error) {
	// 1. A plain object with this exact key.
	if _, err := c.client.StatObject(ctx, c.item.Bucket, target, minio.StatObjectOptions{}); err == nil {
		if err := c.client.RemoveObject(ctx, c.item.Bucket, target, minio.RemoveObjectOptions{}); err != nil {
			return domain.StorageDeleteResult{}, fmt.Errorf("delete S3 object: %w", err)
		}
		// When the same name also prefixes other keys, recursive removes them.
		if recursive {
			count, err := c.deletePrefix(ctx, target)
			if err != nil {
				return domain.StorageDeleteResult{Deleted: true, Count: 1 + count}, err
			}
			return domain.StorageDeleteResult{Deleted: true, Count: 1 + count}, nil
		}
		return domain.StorageDeleteResult{Deleted: true, Count: 1}, nil
	}
	// 2. A folder (prefix) with contents.
	count, listed, err := c.countPrefix(ctx, target)
	if err != nil {
		return domain.StorageDeleteResult{}, err
	}
	if listed {
		if !recursive {
			return domain.StorageDeleteResult{}, fmt.Errorf("%q is a folder; enable recursive delete to remove its %d objects", target, count)
		}
		if err := c.deletePrefixOnly(ctx, target); err != nil {
			return domain.StorageDeleteResult{}, err
		}
		return domain.StorageDeleteResult{Deleted: true, Count: count}, nil
	}
	// 3. A lone folder marker object ("path/").
	marker := target + "/"
	if _, err := c.client.StatObject(ctx, c.item.Bucket, marker, minio.StatObjectOptions{}); err == nil {
		if err := c.client.RemoveObject(ctx, c.item.Bucket, marker, minio.RemoveObjectOptions{}); err != nil {
			return domain.StorageDeleteResult{}, fmt.Errorf("delete S3 folder marker: %w", err)
		}
		return domain.StorageDeleteResult{Deleted: true, Count: 1}, nil
	}
	return domain.StorageDeleteResult{}, fmt.Errorf("remote file %q does not exist", target)
}

func (c *s3Conn) MakeDir(ctx context.Context, dir string) (domain.StorageMakeDirResult, error) {
	marker := dir + "/"
	if _, err := c.client.PutObject(ctx, c.item.Bucket, marker, bytesReader(nil), 0, minio.PutObjectOptions{ContentType: "application/x-directory"}); err != nil {
		return domain.StorageMakeDirResult{}, fmt.Errorf("create S3 folder: %w", err)
	}
	return domain.StorageMakeDirResult{Path: dir, Created: true}, nil
}

// Move renames one object via a server-side copy followed by a delete.
// S3 has no atomic rename, and folder (prefix) moves would require copying
// every object, so they are rejected explicitly.
func (c *s3Conn) Move(ctx context.Context, from, to string) (domain.StorageMoveResult, error) {
	if count, found, err := c.countPrefix(ctx, from); err != nil {
		return domain.StorageMoveResult{}, err
	} else if found {
		return domain.StorageMoveResult{}, fmt.Errorf("%q is a folder with %d objects; moving folders is not supported on S3", from, count)
	}
	if _, err := c.client.StatObject(ctx, c.item.Bucket, from, minio.StatObjectOptions{}); err != nil {
		return domain.StorageMoveResult{}, fmt.Errorf("remote file %q does not exist", from)
	}
	if _, err := c.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: c.item.Bucket, Object: to},
		minio.CopySrcOptions{Bucket: c.item.Bucket, Object: from},
	); err != nil {
		return domain.StorageMoveResult{}, fmt.Errorf("copy S3 object: %w", err)
	}
	if err := c.client.RemoveObject(ctx, c.item.Bucket, from, minio.RemoveObjectOptions{}); err != nil {
		return domain.StorageMoveResult{From: from, To: to, Moved: true}, fmt.Errorf("delete S3 source after move: %w", err)
	}
	return domain.StorageMoveResult{From: from, To: to, Moved: true}, nil
}

// countPrefix returns (objects, hasContents) under dir+"/".
func (c *s3Conn) countPrefix(ctx context.Context, dir string) (int64, bool, error) {
	prefix := remotePrefix(dir)
	found := false
	var count int64
	objectCh := c.client.ListObjects(ctx, c.item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	for object := range objectCh {
		if object.Err != nil {
			return 0, false, fmt.Errorf("list %q: %w", dir, object.Err)
		}
		if object.Key == prefix {
			continue // the folder's own zero-byte marker is not content
		}
		found = true
		count++
		if count > 100_000 {
			return count, found, fmt.Errorf("folder %q holds more than 100,000 objects; delete in smaller batches", dir)
		}
	}
	return count, found, nil
}

// deletePrefixOnly removes every object under dir+"/" (the dir key itself is
// left to the caller — either it does not exist or was already removed).
func (c *s3Conn) deletePrefixOnly(ctx context.Context, dir string) error {
	prefix := remotePrefix(dir)
	objectCh := make(chan minio.ObjectInfo)
	errorCh := c.client.RemoveObjects(ctx, c.item.Bucket, objectCh, minio.RemoveObjectsOptions{})
	go func() {
		defer close(objectCh)
		listCh := c.client.ListObjects(ctx, c.item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
		for object := range listCh {
			if object.Err != nil {
				return
			}
			objectCh <- object
		}
	}()
	var first error
	for result := range errorCh {
		if result.Err != nil && first == nil {
			first = fmt.Errorf("delete S3 object %q: %w", result.ObjectName, result.Err)
		}
	}
	return first
}

// deletePrefix removes the prefix contents and returns how many objects went away.
func (c *s3Conn) deletePrefix(ctx context.Context, dir string) (int64, error) {
	count, found, err := c.countPrefix(ctx, dir)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	if err := c.deletePrefixOnly(ctx, dir); err != nil {
		return count, err
	}
	// The folder marker, if any, needs its own removal.
	_ = c.client.RemoveObject(ctx, c.item.Bucket, dir+"/", minio.RemoveObjectOptions{})
	return count, nil
}

/* ---------------- presigned URLs ---------------- */

// ValidStoragePresignMethod reports whether the HTTP method can be presigned.
// GET, PUT, HEAD, and DELETE have well-defined SigV4 query signing across
// S3-compatible servers; other verbs are rejected rather than mis-signed.
func ValidStoragePresignMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodPut, http.MethodHead, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Presign produces a temporary signed URL for one object. Headers are signed
// into the URL — the consumer must send exactly these headers for the
// signature to validate — and params become signed query parameters
// (response-* overrides, versionId, …). Signing is computed offline; no
// request reaches the server.
func (c *s3Conn) Presign(ctx context.Context, request domain.StoragePresignRequest) (domain.StoragePresignResult, error) {
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if !ValidStoragePresignMethod(method) {
		return domain.StoragePresignResult{}, fmt.Errorf("method must be one of GET, PUT, HEAD, DELETE (got %q)", request.Method)
	}
	if !c.canPresign {
		return domain.StoragePresignResult{}, fmt.Errorf("presigned URLs need an S3 access key and secret key (this connection is anonymous)")
	}
	expires := request.ExpiresSeconds
	if expires == 0 {
		expires = 3600
	}
	if expires < presignMinSeconds || expires > presignMaxSeconds {
		return domain.StoragePresignResult{}, fmt.Errorf("expiry must be between %d seconds and 7 days (got %d seconds)", presignMinSeconds, expires)
	}
	headers := http.Header{}
	for name, value := range request.Headers {
		name = strings.TrimSpace(name)
		if name == "" {
			return domain.StoragePresignResult{}, fmt.Errorf("header names must not be blank")
		}
		headers.Set(name, value) // Set canonicalizes the name (content-type -> Content-Type)
	}
	params := url.Values{}
	for name, value := range request.Params {
		name = strings.TrimSpace(name)
		if name == "" {
			return domain.StoragePresignResult{}, fmt.Errorf("parameter names must not be blank")
		}
		params.Set(name, value)
	}
	signed, err := c.client.PresignHeader(ctx, method, c.item.Bucket, request.Path, time.Duration(expires)*time.Second, params, headers)
	if err != nil {
		return domain.StoragePresignResult{}, fmt.Errorf("presign S3 URL: %w", err)
	}
	result := domain.StoragePresignResult{
		URL:              signed.String(),
		Method:           method,
		ExpiresInSeconds: expires,
		ExpiresAt:        time.Now().UTC().Add(time.Duration(expires) * time.Second).Format(time.RFC3339),
	}
	if len(headers) > 0 {
		result.Headers = make(map[string]string, len(headers))
		for name, values := range headers {
			if len(values) > 0 {
				result.Headers[name] = values[0]
			}
		}
	}
	if len(params) > 0 {
		result.Params = make(map[string]string, len(params))
		for name := range params {
			result.Params[name] = params.Get(name)
		}
	}
	return result, nil
}

/* ---------------- shared helpers ---------------- */

// sortEntries orders folders first, then names case-insensitively.
func sortEntries(entries []domain.StorageEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// bytesReader adapts a byte slice to io.Reader without bytes import churn.
func bytesReader(data []byte) io.Reader { return &byteSliceReader{data: data} }

type byteSliceReader struct {
	data []byte
	pos  int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// detectFileContentType sniffs the first 512 bytes of a local file.
func detectFileContentType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open local file: %w", err)
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read local file: %w", err)
	}
	contentType := http.DetectContentType(buffer[:n])
	if contentType == "" {
		return "application/octet-stream", nil
	}
	return contentType, nil
}

// ensureParent creates the parent directory of a local target path.
func ensureParent(localPath string) error {
	dir := filepath.Dir(localPath)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return nil
}
