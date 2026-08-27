// This file hosts the embedded SugarDB engine: a key/value store that runs
// inside the Neuropipe process and speaks RESP on a loopback-only TCP
// listener. Reusing the loopback listener (instead of the in-process API)
// keeps every existing go-redis code path - command validation, browsing,
// console, and pub/sub - working against SugarDB unchanged.
package kv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sugardblib "github.com/echovault/sugardb/sugardb"
	"github.com/redis/go-redis/v9"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// embeddedManager owns the lifecycle of every running SugarDB instance, one
// per registered sugardb connection. Instances start lazily on first use and
// keep running for the remainder of the session so their AOF/snapshot state
// stays warm; Close stops them all and flushes persistence to disk.
type embeddedManager struct {
	// dataRoot is the app data directory; connections without an explicit
	// Path persist under <dataRoot>/sugardb/<connection id>.
	dataRoot string

	mu      sync.Mutex
	servers map[string]*sugarInstance
}

func newEmbeddedManager(dataRoot string) *embeddedManager {
	return &embeddedManager{dataRoot: dataRoot, servers: make(map[string]*sugarInstance)}
}

// sugarInstance is one running embedded engine plus its loopback address.
type sugarInstance struct {
	server *sugardblib.SugarDB
	addr   string
}

// start brings the embedded engine for item online, returning its loopback
// address. An already-running instance is returned as-is unless restart is
// set (used after a configuration change).
func (m *embeddedManager) start(item domain.Database, secret string, restart bool) (string, error) {
	if item.Driver != domain.DatabaseDriverSugarDB {
		return "", fmt.Errorf("driver %q is not the embedded SugarDB store", item.Driver)
	}
	if item.ID == "" {
		return "", errors.New("database ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if running := m.servers[item.ID]; running != nil {
		if !restart {
			return running.addr, nil
		}
		running.server.ShutDown()
		delete(m.servers, item.ID)
	}
	dir, err := m.dataDir(item)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("prepare sugardb data directory: %w", err)
	}
	port, err := freeLoopbackPort()
	if err != nil {
		return "", fmt.Errorf("reserve sugardb port: %w", err)
	}
	config := sugardblib.DefaultConfig()
	config.BindAddr = "127.0.0.1"
	config.Port = port
	config.DataDir = dir
	// Recover from both persistence layers so restarts restore the keyspace.
	// "always" fsyncs each write: desktop automation write volumes are modest,
	// and losing the tail of the log on exit would surprise users far more
	// than the extra fsync costs.
	config.RestoreAOF = true
	config.RestoreSnapshot = true
	config.AOFSyncStrategy = "always"
	if secret != "" {
		config.RequirePass = true
		config.Password = secret
	}
	server, err := sugardblib.NewSugarDB(sugardblib.WithConfig(config))
	if err != nil {
		return "", fmt.Errorf("start embedded sugardb: %w", err)
	}
	// Start runs the accept loop synchronously, so it must run on its own
	// goroutine; ShutDown closes the listener, which unblocks the loop.
	go server.Start()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	probe := redis.NewClient(embeddedOptions(addr, item, secret))
	defer func() { _ = probe.Close() }()
	probeCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := probe.Ping(probeCtx).Err(); err != nil {
		server.ShutDown()
		return "", fmt.Errorf("start embedded sugardb: %w", err)
	}
	m.servers[item.ID] = &sugarInstance{server: server, addr: addr}
	return addr, nil
}

// stop terminates one connection's engine. Its persisted files are kept so
// unregistering a database never destroys user data.
func (m *embeddedManager) stop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if running := m.servers[id]; running != nil {
		running.server.ShutDown()
		delete(m.servers, id)
	}
}

// close stops every engine; used on service shutdown.
func (m *embeddedManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, running := range m.servers {
		running.server.ShutDown()
		delete(m.servers, id)
	}
}

// server returns the live engine for id, or nil when none is running. Used
// by the Info path to read engine details the RESP protocol does not expose.
func (m *embeddedManager) server(id string) *sugardblib.SugarDB {
	m.mu.Lock()
	defer m.mu.Unlock()
	if running := m.servers[id]; running != nil {
		return running.server
	}
	return nil
}

// dataDir resolves the persistence directory: an explicit Path wins; the
// default nests under the app data root so stores survive upgrades without
// any configuration.
func (m *embeddedManager) dataDir(item domain.Database) (string, error) {
	if dir := strings.TrimSpace(item.Path); dir != "" {
		return dir, nil
	}
	root := strings.TrimSpace(m.dataRoot)
	if root == "" {
		return "", errors.New("sugardb data directory requires an explicit path or an app data root")
	}
	return filepath.Join(root, "sugardb", item.ID), nil
}

// embeddedOptions builds the client options for a running embedded engine.
// DisableIdentity skips the CLIENT SETINFO handshake pipeline, which the
// engine does not implement.
func embeddedOptions(addr string, item domain.Database, secret string) *redis.Options {
	opts := &redis.Options{Addr: addr, MaxRetries: -1, DisableIdentity: true}
	if item.DBIndex > 0 {
		opts.DB = item.DBIndex
	}
	if secret != "" {
		opts.Password = secret
	}
	return opts
}

// freeLoopbackPort reserves an unused TCP port and releases it again. The
// tiny window between release and SugarDB binding is acceptable for a local
// desktop app where no other process races for that port.
func freeLoopbackPort() (uint16, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = listener.Close() }()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("loopback listener has no TCP address")
	}
	return uint16(addr.Port), nil
}
