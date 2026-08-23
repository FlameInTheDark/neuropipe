package remoteexec

import (
	"context"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	executorv1 "github.com/FlameInTheDark/neuropipe/internal/proto/executor/v1"
	"google.golang.org/grpc"
)

// Target identifies one registered executor connection.
type Target struct {
	ID       string
	Address  string
	TokenRef string
	UseTLS   bool
}

// TokenResolver reads shared secrets from the desktop vault.
type TokenResolver interface {
	Get(name string) (string, error)
}

// RunEventHandler receives completed-or-updated runs streamed by executors.
// Implementations update local execution records and emit Wails events.
type RunEventHandler func(executorID string, execution domain.Execution)

// StatusHandler receives connectivity transitions for the Settings display.
type StatusHandler func(executorID string, status domain.RemoteExecutorStatus)

// OnlineHandler fires when an executor transitions offline -> online so the
// app layer can reconcile deployments.
type OnlineHandler func(target Target)

const (
	pingInterval    = 30 * time.Second
	pingTimeout     = 8 * time.Second
	dialRetryDelay  = 10 * time.Second
	hostCallTimeout = 5 * time.Minute
)

// Manager owns every executor connection: supervised dials, the reverse
// tunnel that serves host-service calls, and the event streams feeding run
// results back into the desktop execution records.
type Manager struct {
	vault      TokenResolver
	bridge     HostBridge
	onRunEvent RunEventHandler
	onStatus   StatusHandler
	onOnline   OnlineHandler

	mu    sync.Mutex
	conns map[string]*conn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates the connection manager. Handlers may be nil.
func NewManager(vault TokenResolver, bridge HostBridge, onRunEvent RunEventHandler, onStatus StatusHandler, onOnline OnlineHandler) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		vault:      vault,
		bridge:     bridge,
		onRunEvent: onRunEvent,
		onStatus:   onStatus,
		onOnline:   onOnline,
		conns:      make(map[string]*conn),
		ctx:        ctx,
		cancel:     cancel,
		wg:         sync.WaitGroup{},
	}
}

// Ensure creates or refreshes the supervised connection for a target.
// Changing address, TLS mode, or token reference reopens the connection.
func (m *Manager) Ensure(target Target) error {
	if target.ID == "" || target.Address == "" {
		return ErrUnknownExecutor
	}
	token, err := m.vault.Get(target.TokenRef)
	if err != nil || token == "" {
		return ErrTokenUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.conns[target.ID]; ok {
		if existing.matches(target, token) {
			return nil
		}
		existing.close()
		delete(m.conns, target.ID)
	}
	connection := newConn(m, target, token)
	m.conns[target.ID] = connection
	m.wg.Add(1)
	go connection.supervise(m.ctx, &m.wg, m)
	return nil
}

// Remove stops and forgets one executor connection.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.conns[id]; ok {
		existing.close()
		delete(m.conns, id)
	}
}

// Stop terminates every supervised connection and waits for their loops.
func (m *Manager) Stop() {
	m.cancel()
	m.mu.Lock()
	for _, connection := range m.conns {
		connection.close()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

// CachedStatus returns the last observed connectivity state without I/O.
func (m *Manager) CachedStatus(id string) domain.RemoteExecutorStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if connection, ok := m.conns[id]; ok {
		return connection.snapshotStatus()
	}
	return domain.RemoteExecutorStatus{Online: false, Message: "unknown"}
}

func (m *Manager) connFor(id string) (*conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	connection, ok := m.conns[id]
	if !ok {
		return nil, ErrUnknownExecutor
	}
	return connection, nil
}

// conn is one supervised executor endpoint.
type conn struct {
	manager *Manager
	target  Target
	token   string

	client executorv1.ExecutorClient
	cc     *grpc.ClientConn

	mu     sync.Mutex
	status domain.RemoteExecutorStatus

	sessionMu     sync.Mutex
	sessionCancel context.CancelFunc
}

func newConn(manager *Manager, target Target, token string) *conn {
	return &conn{manager: manager, target: target, token: token}
}

func (c *conn) matches(target Target, token string) bool {
	return c.target.Address == target.Address && c.target.TokenRef == target.TokenRef && c.target.UseTLS == target.UseTLS && c.token == token
}

func (c *conn) close() {
	c.sessionMu.Lock()
	if c.sessionCancel != nil {
		c.sessionCancel()
	}
	c.sessionMu.Unlock()
	c.mu.Lock()
	cc := c.cc
	c.cc = nil
	c.client = nil
	c.status = domain.RemoteExecutorStatus{Online: false, Message: "removed"}
	c.mu.Unlock()
	if cc != nil {
		_ = cc.Close()
	}
}

func (c *conn) snapshotStatus() domain.RemoteExecutorStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *conn) setStatus(status domain.RemoteExecutorStatus) {
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
}

// closeTransport releases a previous session's client connection before a
// fresh dial so broken connections never accumulate.
func (c *conn) closeTransport() {
	c.mu.Lock()
	cc := c.cc
	c.cc = nil
	c.client = nil
	c.mu.Unlock()
	if cc != nil {
		_ = cc.Close()
	}
}

// supervise owns the reconnect loop and the per-session pumps. It exits when
// the manager context ends or close() tears the connection down.
func (c *conn) supervise(lifetime context.Context, wg *sync.WaitGroup, m *Manager) {
	defer wg.Done()
	for {
		c.closeTransport()
		sessionCtx, cancel := context.WithCancel(lifetime)
		err := c.open(sessionCtx, m)
		if err == nil {
			m.notifyStatus(c.target.ID, c.onlineStatus())
			if m.onOnline != nil {
				m.onOnline(c.target)
			}
			err = c.awaitHealthy(sessionCtx, m)
		}
		cancel()
		message := "offline"
		if err != nil {
			message = friendlyError(err)
		}
		c.setStatus(domain.RemoteExecutorStatus{Online: false, Message: message})
		m.notifyStatus(c.target.ID, domain.RemoteExecutorStatus{Online: false, Message: message})
		wait := dialRetryDelay
		if lifetime.Err() != nil {
			return
		}
		select {
		case <-lifetime.Done():
			return
		case <-time.After(wait):
		}
	}
}

// open dials the endpoint and verifies reachability with one status call.
func (c *conn) open(ctx context.Context, m *Manager) error {
	cc, err := grpc.NewClient(c.target.Address, dialOptions(c.token, c.target.UseTLS, "")...)
	if err != nil {
		return err
	}
	client := executorv1.NewExecutorClient(cc)
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	response, pingErr := client.GetStatus(pingCtx, &executorv1.StatusRequest{})
	if pingErr != nil {
		_ = cc.Close()
		return pingErr
	}
	c.mu.Lock()
	c.cc = cc
	c.client = client
	c.status = domain.RemoteExecutorStatus{
		Online:        true,
		Version:       response.GetExecutorVersion(),
		Platform:      response.GetPlatform(),
		ActiveRuns:    int(response.GetActiveRuns()),
		MaxConcurrent: int(response.GetMaxConcurrentRuns()),
	}
	c.mu.Unlock()
	return nil
}

// awaitHealthy keeps the session alive until a health ping fails or the
// lifetime context ends.
func (c *conn) awaitHealthy(sessionCtx context.Context, m *Manager) error {
	sessionCtx, cancel := context.WithCancel(sessionCtx)
	defer cancel()
	c.sessionMu.Lock()
	c.sessionCancel = cancel
	c.sessionMu.Unlock()

	sessionDone := make(chan error, 2)
	go c.pumpTunnel(sessionCtx, sessionDone)
	go c.pumpEvents(sessionCtx, sessionDone)

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	var firstErr error
	for {
		select {
		case err := <-sessionDone:
			firstErr = err
		case <-ticker.C:
			if err := c.ping(); err != nil {
				firstErr = err
			} else {
				continue
			}
		case <-sessionCtx.Done():
			return sessionCtx.Err()
		}
		if firstErr != nil {
			return firstErr
		}
	}
}

func (c *conn) ping() error {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		return ErrUnknownExecutor
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	_, err := client.GetStatus(ctx, &executorv1.StatusRequest{})
	return err
}

func (c *conn) onlineStatus() domain.RemoteExecutorStatus {
	status := c.snapshotStatus()
	status.Online = true
	status.Message = ""
	return status
}

func (m *Manager) notifyStatus(id string, status domain.RemoteExecutorStatus) {
	if m.onStatus != nil {
		m.onStatus(id, status)
	}
}
