// Package runtime owns processes launched by Neuropipe for local inference.
package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// LlamaManager owns only the llama-server process it starts itself.
type LlamaManager struct {
	mu       sync.RWMutex
	process  *managedProcess
	endpoint string
	mode     domain.RuntimeMode
	model    string
}

// NewLlamaManager creates an inactive local inference manager.
func NewLlamaManager() *LlamaManager { return &LlamaManager{} }

// Start launches llama-server on a free loopback port and waits for its OpenAI API.
func (m *LlamaManager) Start(ctx context.Context, settings domain.LlamaRuntimeSettings) (domain.LlamaRuntimeStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil && m.process.Running() {
		return m.statusLocked(), nil
	}
	if err := validateSettings(settings); err != nil {
		return domain.LlamaRuntimeStatus{}, err
	}
	port, err := freePort()
	if err != nil {
		return domain.LlamaRuntimeStatus{}, err
	}
	mode := resolveMode(settings.Mode)
	args := serverArguments(settings, port, mode)
	process, err := startProcess(settings.BinaryPath, args)
	if err != nil {
		return domain.LlamaRuntimeStatus{}, fmt.Errorf("start llama.cpp: %w", err)
	}
	endpoint := "http://127.0.0.1:" + port
	if err := waitReady(ctx, endpoint+"/v1/models", 45*time.Second, process.Done()); err != nil {
		process.Stop()
		return domain.LlamaRuntimeStatus{}, fmt.Errorf("llama.cpp did not become ready: %w; output: %s", err, process.Output())
	}
	m.process, m.endpoint, m.mode, m.model = process, endpoint, mode, filepath.Base(settings.ModelPath)
	return m.statusLocked(), nil
}

// Stop terminates the managed process, if running.
func (m *LlamaManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil {
		m.process.Stop()
	}
	m.process, m.endpoint, m.model = nil, "", ""
}

// Status reports the process owned by Neuropipe.
func (m *LlamaManager) Status() domain.LlamaRuntimeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statusLocked()
}

// PID returns the process ID of the running child Neuropipe launched itself.
// It is intentionally an internal metric source, not a renderer-facing value.
func (m *LlamaManager) PID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.process == nil || !m.process.Running() || m.process.command.Process == nil {
		return 0
	}
	return m.process.command.Process.Pid
}

func (m *LlamaManager) statusLocked() domain.LlamaRuntimeStatus {
	running := m.process != nil && m.process.Running()
	status := domain.LlamaRuntimeStatus{Running: running, Endpoint: m.endpoint, Mode: m.mode, Model: m.model}
	if running {
		status.Message = "Managed llama.cpp server is ready on loopback."
	} else {
		status.Message = "Not running"
	}
	return status
}

func validateSettings(settings domain.LlamaRuntimeSettings) error {
	if info, err := os.Stat(settings.BinaryPath); err != nil || info.IsDir() {
		return errors.New("choose a valid llama-server executable")
	}
	if info, err := os.Stat(settings.ModelPath); err != nil || info.IsDir() {
		return errors.New("choose a valid GGUF model file")
	}
	if settings.ContextSize < 1024 {
		return errors.New("context size must be at least 1024")
	}
	return nil
}

func resolveMode(mode domain.RuntimeMode) domain.RuntimeMode {
	if mode == "" || mode == domain.RuntimeAuto {
		return domain.RuntimeCPU
	}
	return mode
}

func serverArguments(settings domain.LlamaRuntimeSettings, port string, mode domain.RuntimeMode) []string {
	args := []string{"-m", settings.ModelPath, "--host", "127.0.0.1", "--port", port, "--alias", filepath.Base(settings.ModelPath), "-c", fmt.Sprint(settings.ContextSize)}
	if mode == domain.RuntimeCUDA || mode == domain.RuntimeVulkan || mode == domain.RuntimeHIP {
		args = append(args, "-ngl", "999")
	}
	return args
}

func freePort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find free loopback port: %w", err)
	}
	defer func() { _ = listener.Close() }()
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port), nil
}

func waitReady(ctx context.Context, endpoint string, timeout time.Duration, done <-chan struct{}) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				_ = response.Body.Close()
				if response.StatusCode/100 == 2 {
					return nil
				}
			}
		}
		select {
		case <-done:
			return errors.New("the managed process exited before opening its API")
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("timed out waiting for the local API")
		case <-ticker.C:
		}
	}
}

type managedProcess struct {
	command *exec.Cmd
	done    chan struct{}
	output  *processOutput
}

func startProcess(binary string, args []string) (*managedProcess, error) {
	output := &processOutput{}
	command := exec.Command(binary, args...)
	command.Dir = filepath.Dir(binary)
	configureHiddenChildProcess(command)
	command.Stdout = io.MultiWriter(os.Stderr, output)
	command.Stderr = io.MultiWriter(os.Stderr, output)
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &managedProcess{command: command, done: make(chan struct{}), output: output}
	go func() { _ = command.Wait(); close(process.done) }()
	return process, nil
}

func (p *managedProcess) Done() <-chan struct{} { return p.done }
func (p *managedProcess) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *managedProcess) Stop() {
	if p.Running() && p.command.Process != nil {
		_ = p.command.Process.Kill()
	}
	<-p.done
}
func (p *managedProcess) Output() string { return p.output.String() }

type processOutput struct {
	mu   sync.Mutex
	data []byte
}

func (o *processOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data = append(o.data, data...)
	if len(o.data) > 6000 {
		o.data = append([]byte(nil), o.data[len(o.data)-6000:]...)
	}
	return len(data), nil
}
func (o *processOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return string(bytes.TrimSpace(o.data))
}
