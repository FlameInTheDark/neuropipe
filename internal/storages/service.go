// Package storages manages registered remote storage connections (S3 and
// FTP) and executes file operations against them. Metadata lives in its own
// persistence table, secrets in the vault, and every operation resolves the
// connection by ID so credentials never leave the backend. Driver-specific
// behaviour lives in s3.go and ftp.go behind the storageConn interface.
package storages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/security"
	"github.com/google/uuid"
)

const (
	pingTimeout = 10 * time.Second
	// maxUploadDataBytes caps in-memory uploads (Upload Data nodes) so a
	// runaway graph cannot exhaust the process. File-to-storage uploads
	// stream and are not capped here.
	maxUploadDataBytes = 512 << 20
)

// storageConn is the driver-agnostic file-operation surface shared by the
// S3 and FTP implementations. Paths are normalized storage paths (see
// paths.go) — drivers translate them to object keys or FTP paths.
type storageConn interface {
	// Probe verifies the connection works (cheap liveness check).
	Probe(ctx context.Context) error
	List(ctx context.Context, path string) ([]domain.StorageEntry, error)
	UploadFile(ctx context.Context, localPath, remotePath, contentType string) (domain.StorageUploadResult, error)
	UploadData(ctx context.Context, data []byte, remotePath, contentType string) (domain.StorageUploadResult, error)
	Download(ctx context.Context, remotePath, localPath string) (domain.StorageDownloadResult, error)
	Delete(ctx context.Context, path string, recursive bool) (domain.StorageDeleteResult, error)
	MakeDir(ctx context.Context, path string) (domain.StorageMakeDirResult, error)
	Move(ctx context.Context, from, to string) (domain.StorageMoveResult, error)
	Close() error
}

// Service manages registered storages, their cached driver connections, and
// the file operations exposed to the browser view and pipeline nodes.
type Service struct {
	store  *persistence.Store
	vault  *security.Vault
	mu     sync.Mutex
	conns  map[string]storageConn
	closed bool
}

// New creates a storage service. vault may be nil for anonymous public
// buckets, but authenticated S3 and password-protected FTP connections
// require it to resolve secret references.
func New(store *persistence.Store, vault *security.Vault) *Service {
	return &Service{store: store, vault: vault, conns: make(map[string]storageConn)}
}

// Close releases every cached driver connection. Subsequent calls fail.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var first error
	for id, conn := range s.conns {
		if err := conn.Close(); err != nil && first == nil {
			first = fmt.Errorf("close storage connection %q: %w", id, err)
		}
	}
	clear(s.conns)
	return first
}

func (s *Service) List(ctx context.Context) ([]domain.Storage, error) {
	return s.store.ListStorages(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Storage, error) {
	return s.store.GetStorage(ctx, strings.TrimSpace(id))
}

// Register records a new storage connection, storing its secret in the vault
// and probing reachability so the browser shows a truthful status pill.
func (s *Service) Register(ctx context.Context, request domain.SaveStorageRequest) (domain.Storage, error) {
	item, err := BuildStorage(request)
	if err != nil {
		return domain.Storage{}, err
	}
	if err := s.applySecrets(&item, request); err != nil {
		return domain.Storage{}, err
	}
	secret, password, err := s.resolveSecrets(item)
	if err != nil {
		return domain.Storage{}, err
	}
	status := domain.DatabaseStatusUnverified
	probeCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	if err := s.probe(probeCtx, item, secret, password); err == nil {
		status = domain.DatabaseStatusConnected
	}
	cancel()
	created, err := s.store.CreateStorage(ctx, item)
	if err != nil {
		_ = s.deleteSecret(item.SecretRef)
		_ = s.deleteSecret(item.PasswordRef)
		return domain.Storage{}, err
	}
	if status == domain.DatabaseStatusConnected {
		if err := s.store.UpdateStorageStatus(ctx, created.ID, domain.DatabaseStatusConnected); err != nil {
			return domain.Storage{}, err
		}
	}
	return s.store.GetStorage(ctx, created.ID)
}

// Update replaces a storage's metadata, rotating vault secrets when new
// values are supplied, and drops any cached connection so the next operation
// dials with the new settings.
func (s *Service) Update(ctx context.Context, request domain.SaveStorageRequest) (domain.Storage, error) {
	item, err := BuildStorage(request)
	if err != nil {
		return domain.Storage{}, err
	}
	item.ID = strings.TrimSpace(request.ID)
	if item.ID == "" {
		return domain.Storage{}, fmt.Errorf("storage ID is required")
	}
	stored, err := s.store.GetStorage(ctx, item.ID)
	if err != nil {
		return domain.Storage{}, err
	}
	if err := s.applySecrets(&item, request); err != nil {
		return domain.Storage{}, err
	}
	updated, err := s.store.UpdateStorage(ctx, item)
	if err != nil {
		return domain.Storage{}, err
	}
	if stored.SecretRef != "" && stored.SecretRef != updated.SecretRef {
		_ = s.deleteSecret(stored.SecretRef)
	}
	if stored.PasswordRef != "" && stored.PasswordRef != updated.PasswordRef {
		_ = s.deleteSecret(stored.PasswordRef)
	}
	s.mu.Lock()
	if conn, exists := s.conns[item.ID]; exists {
		_ = conn.Close()
		delete(s.conns, item.ID)
	}
	s.mu.Unlock()
	return updated, nil
}

// Delete removes the storage registration, closes its cached connection, and
// purges the vault secrets it owns. Remote files are never touched.
func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	item, err := s.store.GetStorage(ctx, id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if conn, exists := s.conns[id]; exists {
		_ = conn.Close()
		delete(s.conns, id)
	}
	s.mu.Unlock()
	if err := s.store.DeleteStorage(ctx, id); err != nil {
		return err
	}
	_ = s.deleteSecret(item.SecretRef)
	_ = s.deleteSecret(item.PasswordRef)
	return nil
}

// Ping re-probes the connection and persists the resulting status.
func (s *Service) Ping(ctx context.Context, id string) (domain.DatabaseStatus, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return domain.DatabaseStatusError, err
	}
	secret, password, err := s.resolveSecrets(item)
	if err != nil {
		return domain.DatabaseStatusError, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	status := domain.DatabaseStatusConnected
	if err := s.probe(probeCtx, item, secret, password); err != nil {
		status = domain.DatabaseStatusError
	}
	if err := s.store.UpdateStorageStatus(ctx, item.ID, status); err != nil {
		return status, err
	}
	return status, nil
}

// TestConnection probes the supplied (unsaved) settings without persisting
// anything, so the connection dialog can verify before saving.
func (s *Service) TestConnection(ctx context.Context, item domain.Storage, secret, password string) (domain.DatabaseStatus, error) {
	if err := validateStorageItem(item); err != nil {
		return domain.DatabaseStatusError, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := s.probe(probeCtx, item, secret, password); err != nil {
		return domain.DatabaseStatusError, err
	}
	return domain.DatabaseStatusConnected, nil
}

// probe opens a transient driver connection and runs its cheap liveness
// check (S3: bucket exists; FTP: login + NOOP).
func (s *Service) probe(ctx context.Context, item domain.Storage, secret, password string) error {
	conn, err := s.dial(ctx, item, secret, password)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	return conn.Probe(ctx)
}

// connection returns the cached driver connection for a storage, dialing on
// first use or after settings changed.
func (s *Service) connection(ctx context.Context, id string) (storageConn, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("select a storage first")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("storage service is closed")
	}
	if conn, exists := s.conns[id]; exists {
		s.mu.Unlock()
		return conn, nil
	}
	s.mu.Unlock()
	item, err := s.store.GetStorage(ctx, id)
	if err != nil {
		return nil, err
	}
	secret, password, err := s.resolveSecrets(item)
	if err != nil {
		return nil, err
	}
	conn, err := s.dial(ctx, item, secret, password)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = conn.Close()
		return nil, fmt.Errorf("storage service is closed")
	}
	if existing, exists := s.conns[id]; exists {
		// Another goroutine won the race; reuse its connection.
		s.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	s.conns[id] = conn
	s.mu.Unlock()
	return conn, nil
}

// dropConnection closes and forgets the cached connection for a storage —
// used when a cached connection turns out to be stale.
func (s *Service) dropConnection(id string) {
	s.mu.Lock()
	if conn, exists := s.conns[id]; exists {
		_ = conn.Close()
		delete(s.conns, id)
	}
	s.mu.Unlock()
}

// dial builds a driver connection from persisted settings and secrets.
func (s *Service) dial(ctx context.Context, item domain.Storage, secret, password string) (storageConn, error) {
	switch item.Driver {
	case domain.StorageDriverS3:
		return dialS3(ctx, item, secret)
	case domain.StorageDriverFTP:
		return dialFTP(ctx, item, password)
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", item.Driver)
	}
}

/* ---------------- vault helpers ---------------- */

// resolveSecrets loads the plaintext S3 secret key and FTP password for a
// storage. Missing refs resolve to empty strings (anonymous access).
func (s *Service) resolveSecrets(item domain.Storage) (secret string, password string, err error) {
	secret, err = s.resolveSecret(item.SecretRef, "storage secret")
	if err != nil {
		return "", "", err
	}
	password, err = s.resolveSecret(item.PasswordRef, "storage password")
	if err != nil {
		return "", "", err
	}
	return secret, password, nil
}

func (s *Service) resolveSecret(ref, what string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return "", nil
	}
	secret, err := s.vault.Get(ref)
	if err != nil {
		return "", fmt.Errorf("load %s: %w", what, err)
	}
	return secret, nil
}

// applySecrets writes new S3/FTP secrets (if any) to the vault and updates
// the item's refs. Blank values preserve the existing refs.
func (s *Service) applySecrets(item *domain.Storage, request domain.SaveStorageRequest) error {
	if err := s.applySecret(&item.SecretRef, strings.TrimSpace(request.Secret), "stgsec:"); err != nil {
		return err
	}
	return s.applySecret(&item.PasswordRef, strings.TrimSpace(request.Password), "stgpw:")
}

func (s *Service) applySecret(ref *string, value, prefix string) error {
	if value == "" {
		return nil
	}
	if s.vault == nil {
		return fmt.Errorf("storage secret storage is unavailable")
	}
	name := strings.TrimSpace(*ref)
	if name == "" {
		name = prefix + uuid.NewString()
		*ref = name
	}
	if err := s.vault.Put(name, value); err != nil {
		return fmt.Errorf("store storage secret: %w", err)
	}
	return nil
}

func (s *Service) deleteSecret(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || s.vault == nil {
		return nil
	}
	return s.vault.Delete(ref)
}

/* ---------------- file operations (nodes + browser) ---------------- */

func (s *Service) StorageListFiles(ctx context.Context, request domain.StorageListRequest) (domain.StorageListResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageListResult{}, err
	}
	path, err := CleanRemotePath(request.Path)
	if err != nil {
		return domain.StorageListResult{}, err
	}
	entries, err := conn.List(ctx, path)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageListResult{}, err
	}
	return domain.StorageListResult{Path: path, Entries: entries}, nil
}

func (s *Service) StorageUploadFile(ctx context.Context, request domain.StorageUploadFileRequest) (domain.StorageUploadResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	remotePath, err := CleanRemotePath(request.RemotePath)
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	result, err := conn.UploadFile(ctx, request.LocalPath, remotePath, request.ContentType)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageUploadResult{}, err
	}
	return result, nil
}

func (s *Service) StorageUploadData(ctx context.Context, request domain.StorageUploadDataRequest) (domain.StorageUploadResult, error) {
	if len(request.Data) > maxUploadDataBytes {
		return domain.StorageUploadResult{}, fmt.Errorf("upload data exceeds the %d MiB limit", maxUploadDataBytes>>20)
	}
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	remotePath, err := CleanRemotePath(request.RemotePath)
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	result, err := conn.UploadData(ctx, request.Data, remotePath, request.ContentType)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageUploadResult{}, err
	}
	return result, nil
}

func (s *Service) StorageDownloadFile(ctx context.Context, request domain.StorageDownloadRequest) (domain.StorageDownloadResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageDownloadResult{}, err
	}
	remotePath, err := CleanRemotePath(request.RemotePath)
	if err != nil {
		return domain.StorageDownloadResult{}, err
	}
	result, err := conn.Download(ctx, remotePath, request.LocalPath)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageDownloadResult{}, err
	}
	return result, nil
}

func (s *Service) StorageDelete(ctx context.Context, request domain.StorageDeleteRequest) (domain.StorageDeleteResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageDeleteResult{}, err
	}
	path, err := CleanRemotePath(request.Path)
	if err != nil {
		return domain.StorageDeleteResult{}, err
	}
	result, err := conn.Delete(ctx, path, request.Recursive)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageDeleteResult{}, err
	}
	return result, nil
}

func (s *Service) StorageMakeDir(ctx context.Context, request domain.StorageMakeDirRequest) (domain.StorageMakeDirResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageMakeDirResult{}, err
	}
	path, err := CleanRemotePath(request.Path)
	if err != nil {
		return domain.StorageMakeDirResult{}, err
	}
	result, err := conn.MakeDir(ctx, path)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageMakeDirResult{}, err
	}
	return result, nil
}

func (s *Service) StorageMove(ctx context.Context, request domain.StorageMoveRequest) (domain.StorageMoveResult, error) {
	conn, err := s.connection(ctx, request.StorageID)
	if err != nil {
		return domain.StorageMoveResult{}, err
	}
	from, err := CleanRemotePath(request.From)
	if err != nil {
		return domain.StorageMoveResult{}, err
	}
	to, err := CleanRemotePath(request.To)
	if err != nil {
		return domain.StorageMoveResult{}, err
	}
	result, err := conn.Move(ctx, from, to)
	if err != nil {
		s.maybeDropStale(request.StorageID, err)
		return domain.StorageMoveResult{}, err
	}
	return result, nil
}

// maybeDropStale forgets a cached connection after a failure so the next
// operation re-dials. Cheap for S3 (client has no state); correct for FTP
// (control channel may have timed out server-side).
func (s *Service) maybeDropStale(id string, err error) {
	if err == nil {
		return
	}
	if ctxErr(err) {
		return // cancellation/timeout of the caller, not a broken connection
	}
	s.dropConnection(id)
}

func ctxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
