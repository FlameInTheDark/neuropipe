package executord

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// RuntimeProvider is one provider configured for local LLM mode. The API key
// lives in the executor vault; only its reference is persisted here.
type RuntimeProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	Enabled   bool   `json:"enabled"`
	APIKeyRef string `json:"apiKeyRef,omitempty"`
}

// RuntimeConfig is the RPC-managed subset of executor settings. It is
// persisted locally and survives restarts.
type RuntimeConfig struct {
	LLMMode           domain.ExecutorLLMMode `json:"llmMode"`
	DefaultProviderID string                 `json:"defaultProviderId,omitempty"`
	MaxConcurrentRuns int                    `json:"maxConcurrentRuns"`
	Providers         []RuntimeProvider      `json:"providers,omitempty"`
}

// DefaultRuntimeConfig returns the shipped defaults: proxy LLM calls through
// the desktop session and allow four parallel runs.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		LLMMode:           domain.ExecutorLLMProxy,
		MaxConcurrentRuns: DefaultMaxConcurrentRuns,
	}
}

// runtimeStore persists the mutable configuration atomically.
type runtimeStore struct {
	path string

	mu     sync.Mutex
	config RuntimeConfig
}

const runtimeFileName = "runtime.json"

// NewRuntimeStore loads the mutable configuration, applying defaults when
// no file exists yet.
func NewRuntimeStore(dataDir string) (*runtimeStore, error) {
	store := &runtimeStore{path: filepath.Join(dataDir, runtimeFileName), config: DefaultRuntimeConfig()}
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read executor runtime config: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &store.config); err != nil {
			return nil, fmt.Errorf("parse executor runtime config: %w", err)
		}
	}
	store.normalize()
	return store, nil
}

// normalize repairs legacy or partial files so the daemon always starts with
// coherent values.
func (s *runtimeStore) normalize() {
	if s.config.LLMMode != domain.ExecutorLLMLocal && s.config.LLMMode != domain.ExecutorLLMProxy {
		s.config.LLMMode = domain.ExecutorLLMProxy
	}
	if s.config.MaxConcurrentRuns < 1 {
		s.config.MaxConcurrentRuns = DefaultMaxConcurrentRuns
	}
}

func (s *runtimeStore) Get() RuntimeConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *runtimeStore) Set(config RuntimeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	s.normalize()
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode executor runtime config: %w", err)
	}
	if err := atomicWrite(s.path, data); err != nil {
		return fmt.Errorf("persist executor runtime config: %w", err)
	}
	return nil
}
