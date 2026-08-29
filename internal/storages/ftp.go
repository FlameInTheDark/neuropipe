package storages

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jlaffaye/ftp"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

const (
	ftpDialTimeout  = 15 * time.Second
	ftpIdleRedial   = 60 * time.Second
	ftpCommandWait  = 60 * time.Second
	ftpTransferWait = 5 * time.Minute
)

// dialFTP connects and logs in to an FTP server. FTP control channels are
// stateful, so the returned conn serializes every command behind a mutex and
// re-dials transparently when the server has dropped an idle connection.
func dialFTP(ctx context.Context, item domain.Storage, password string) (storageConn, error) {
	conn := &ftpConn{mu: newChanMutex(), item: item, password: password}
	if err := conn.redial(ctx); err != nil {
		return nil, err
	}
	return conn, nil
}

type ftpConn struct {
	mu       chanMutex
	item     domain.Storage
	password string
	conn     *ftp.ServerConn
	lastUsed time.Time
}

// chanMutex is a mutex that also exposes TryLock-style semantics used to
// serialise commands on the single FTP control channel.
type chanMutex struct {
	lock chan struct{}
}

func (m *chanMutex) Lock()   { <-m.lock }
func (m *chanMutex) Unlock() { m.lock <- struct{}{} }

func newChanMutex() chanMutex {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return chanMutex{lock: lock}
}

// redial establishes a fresh control connection, logs in, and switches into
// the configured base directory. The caller must hold the mutex.
func (c *ftpConn) redial(ctx context.Context) error {
	address := net.JoinHostPort(c.item.Host, fmt.Sprintf("%d", ftpPort(c.item)))
	dialCtx, cancel := context.WithTimeout(ctx, ftpDialTimeout)
	defer cancel()
	options := []ftp.DialOption{ftp.DialWithContext(dialCtx), ftp.DialWithTimeout(ftpDialTimeout)}
	switch c.item.TLSMode {
	case domain.StorageTLSExplicit:
		options = append(options, ftp.DialWithExplicitTLS(tlsConfigFor(c.item.Host)))
	case domain.StorageTLSImplicit:
		options = append(options, ftp.DialWithTLS(tlsConfigFor(c.item.Host)))
	}
	conn, err := ftp.Dial(address, options...)
	if err != nil {
		return fmt.Errorf("connect to FTP server %s: %w", address, err)
	}
	if c.item.Username != "" || c.password != "" {
		if err := conn.Login(c.item.Username, c.password); err != nil {
			_ = conn.Quit()
			return fmt.Errorf("FTP login failed: %w", err)
		}
	}
	if c.item.BaseDir != "" {
		if err := conn.ChangeDir(c.item.BaseDir); err != nil {
			_ = conn.Quit()
			return fmt.Errorf("enter FTP base directory %q: %w", c.item.BaseDir, err)
		}
	}
	c.conn = conn
	c.lastUsed = time.Now()
	return nil
}

// withConn runs fn against the live control connection, re-dialing once when
// the cached channel turned out to be stale (idle timeout, server restart).
func (c *ftpConn) withConn(ctx context.Context, fn func(conn *ftp.ServerConn) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		if err := c.redial(ctx); err != nil {
			return err
		}
	} else if time.Since(c.lastUsed) > ftpIdleRedial {
		// Proactively refresh long-idle channels instead of risking a dead
		// socket half-way through a transfer.
		_ = c.conn.Quit()
		c.conn = nil
		if err := c.redial(ctx); err != nil {
			return err
		}
	}
	err := fn(c.conn)
	c.lastUsed = time.Now()
	if err != nil && !ctxErr(err) {
		// One transparent retry on a fresh connection.
		_ = c.conn.Quit()
		c.conn = nil
		if dialErr := c.redial(ctx); dialErr != nil {
			return err
		}
		err = fn(c.conn)
		c.lastUsed = time.Now()
	}
	return err
}

func (c *ftpConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Quit()
	c.conn = nil
	return err
}

// Probe issues NOOP to confirm the session is alive.
func (c *ftpConn) Probe(ctx context.Context) error {
	return c.withConn(ctx, func(conn *ftp.ServerConn) error {
		if err := conn.NoOp(); err != nil {
			return fmt.Errorf("FTP server unreachable: %w", err)
		}
		return nil
	})
}

func (c *ftpConn) List(ctx context.Context, dir string) ([]domain.StorageEntry, error) {
	entries := make([]domain.StorageEntry, 0)
	err := c.withConn(ctx, func(conn *ftp.ServerConn) error {
		target := "."
		if dir != "" {
			target = dir
		}
		listed, err := conn.List(target)
		if err != nil {
			return fmt.Errorf("list FTP directory %q: %w", dir, err)
		}
		for _, entry := range listed {
			name := strings.TrimSpace(entry.Name)
			if name == "" || name == "." || name == ".." {
				continue
			}
			switch entry.Type {
			case ftp.EntryTypeFolder:
				entries = append(entries, domain.StorageEntry{Name: name, Path: entryPath(dir, name), IsDir: true, ModTime: entry.Time})
			case ftp.EntryTypeLink:
				// Symlinks are listed as folders when they point at
				// directories; servers report the type already resolved.
				entries = append(entries, domain.StorageEntry{Name: name, Path: entryPath(dir, name), IsDir: true, ModTime: entry.Time})
			default:
				entries = append(entries, domain.StorageEntry{Name: name, Path: entryPath(dir, name), Size: int64(entry.Size), ModTime: entry.Time})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortEntries(entries)
	return entries, nil
}

func (c *ftpConn) UploadFile(ctx context.Context, localPath, remotePath, _ string) (domain.StorageUploadResult, error) {
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
	file, err := os.Open(localPath)
	if err != nil {
		return domain.StorageUploadResult{}, fmt.Errorf("open local file: %w", err)
	}
	defer func() { _ = file.Close() }()
	err = c.withConn(ctx, func(conn *ftp.ServerConn) error {
		if err := conn.Stor(remotePath, file); err != nil {
			return fmt.Errorf("upload to FTP: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	return domain.StorageUploadResult{Key: remotePath, Size: info.Size(), Driver: string(domain.StorageDriverFTP)}, nil
}

func (c *ftpConn) UploadData(ctx context.Context, data []byte, remotePath, _ string) (domain.StorageUploadResult, error) {
	if remotePath == "" {
		return domain.StorageUploadResult{}, fmt.Errorf("remote path is required")
	}
	err := c.withConn(ctx, func(conn *ftp.ServerConn) error {
		if err := conn.Stor(remotePath, strings.NewReader(string(data))); err != nil {
			return fmt.Errorf("upload to FTP: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StorageUploadResult{}, err
	}
	return domain.StorageUploadResult{Key: remotePath, Size: int64(len(data)), Driver: string(domain.StorageDriverFTP)}, nil
}

func (c *ftpConn) Download(ctx context.Context, remotePath, localPath string) (domain.StorageDownloadResult, error) {
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
	var written int64
	err = c.withConn(ctx, func(conn *ftp.ServerConn) error {
		response, err := conn.Retr(remotePath)
		if err != nil {
			if isFTPNotFound(err) {
				return fmt.Errorf("remote file %q does not exist", remotePath)
			}
			return fmt.Errorf("open FTP file: %w", err)
		}
		defer func() { _ = response.Close() }()
		written, err = io.Copy(file, response)
		if err != nil {
			return fmt.Errorf("download from FTP: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StorageDownloadResult{}, err
	}
	return domain.StorageDownloadResult{Path: localPath, Name: baseName(remotePath), Bytes: written}, nil
}

func (c *ftpConn) Delete(ctx context.Context, target string, recursive bool) (domain.StorageDeleteResult, error) {
	var result domain.StorageDeleteResult
	err := c.withConn(ctx, func(conn *ftp.ServerConn) error {
		isDir, err := c.entryIsDir(conn, target)
		if err != nil {
			return err
		}
		if !isDir {
			if err := conn.Delete(target); err != nil {
				return fmt.Errorf("delete FTP file %q: %w", target, err)
			}
			result = domain.StorageDeleteResult{Deleted: true, Count: 1}
			return nil
		}
		if !recursive {
			return fmt.Errorf("%q is a folder; enable recursive delete to remove its contents", target)
		}
		count, err := c.removeDir(conn, target)
		if err != nil {
			return err
		}
		result = domain.StorageDeleteResult{Deleted: true, Count: count}
		return nil
	})
	return result, err
}

// removeDir recursively deletes one FTP directory and returns the number of
// removed entries (files plus directories).
func (c *ftpConn) removeDir(conn *ftp.ServerConn, dir string) (int64, error) {
	listed, err := conn.List(dir)
	if err != nil {
		return 0, fmt.Errorf("list FTP directory %q: %w", dir, err)
	}
	var count int64
	for _, entry := range listed {
		name := strings.TrimSpace(entry.Name)
		if name == "" || name == "." || name == ".." {
			continue
		}
		child := entryPath(dir, name)
		if entry.Type == ftp.EntryTypeFolder || entry.Type == ftp.EntryTypeLink {
			// The recursive call removes the child folder itself and counts it.
			childCount, err := c.removeDir(conn, child)
			if err != nil {
				return count, err
			}
			count += childCount
			continue
		}
		if err := conn.Delete(child); err != nil {
			return count, fmt.Errorf("delete FTP file %q: %w", child, err)
		}
		count++
	}
	if err := conn.RemoveDir(dir); err != nil {
		return count, fmt.Errorf("delete FTP folder %q: %w", dir, err)
	}
	return count + 1, nil
}

func (c *ftpConn) MakeDir(ctx context.Context, dir string) (domain.StorageMakeDirResult, error) {
	var result domain.StorageMakeDirResult
	err := c.withConn(ctx, func(conn *ftp.ServerConn) error {
		if err := conn.MakeDir(dir); err != nil {
			return fmt.Errorf("create FTP folder %q: %w", dir, err)
		}
		result = domain.StorageMakeDirResult{Path: dir, Created: true}
		return nil
	})
	return result, err
}

// Move renames remotely via RNFR/RNTO. FTP servers rename folders just as
// atomically as files.
func (c *ftpConn) Move(ctx context.Context, from, to string) (domain.StorageMoveResult, error) {
	var result domain.StorageMoveResult
	err := c.withConn(ctx, func(conn *ftp.ServerConn) error {
		if err := conn.Rename(from, to); err != nil {
			if isFTPNotFound(err) {
				return fmt.Errorf("remote file %q does not exist", from)
			}
			return fmt.Errorf("move FTP entry %q: %w", from, err)
		}
		result = domain.StorageMoveResult{From: from, To: to, Moved: true}
		return nil
	})
	return result, err
}

// entryIsDir resolves whether a remote path is a folder by listing its
// parent; FTP has no per-path STAT that works across servers.
func (c *ftpConn) entryIsDir(conn *ftp.ServerConn, target string) (bool, error) {
	if target == "" {
		return true, nil
	}
	parent := ""
	name := target
	if idx := strings.LastIndex(target, "/"); idx >= 0 {
		parent, name = target[:idx], target[idx+1:]
	}
	listed, err := conn.List(parentOrRoot(parent))
	if err != nil {
		return false, fmt.Errorf("list FTP directory %q: %w", parent, err)
	}
	for _, entry := range listed {
		if strings.TrimSpace(entry.Name) == name {
			return entry.Type == ftp.EntryTypeFolder || entry.Type == ftp.EntryTypeLink, nil
		}
	}
	return false, fmt.Errorf("remote file %q does not exist", target)
}

func parentOrRoot(parent string) string {
	if parent == "" {
		return "."
	}
	return parent
}

// isConnFailure reports whether err is a transport-level failure (dropped
// socket, timeout, EOF) rather than an FTP protocol reply, so withConn only
// retries operations the server never actually executed.
func isConnFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

// isFTPNotFound maps common "no such file" replies onto one condition. The
// library surfaces server replies as textproto errors whose message starts
// with the numeric status ("550 File unavailable."), so the code is matched
// from the text.
func isFTPNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if strings.HasPrefix(message, "550") {
		return true
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "no such file") || strings.Contains(lower, "not found")
}

// tlsConfigFor builds the client TLS configuration for FTPS: the server name
// matches the host and modern cipher versions are enforced.
func tlsConfigFor(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}
