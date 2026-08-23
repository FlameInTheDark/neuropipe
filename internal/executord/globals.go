package executord

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// executorGlobals implements pipeline.GlobalVariablesStore with an
// executor-local, untyped JSON file. Global variables on a remote executor
// are intentionally isolated from the desktop workspace: they are created
// implicitly on first write and persist across restarts.
type executorGlobals struct {
	path string

	mu     sync.Mutex
	values map[string]any
}

const globalsFileName = "globals.json"

// NewExecutorGlobals loads the executor-local global variable store.
func NewExecutorGlobals(dataDir string) (*executorGlobals, error) {
	store := &executorGlobals{path: filepath.Join(dataDir, globalsFileName), values: map[string]any{}}
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read executor global variables: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &store.values); err != nil {
			return nil, fmt.Errorf("parse executor global variables: %w", err)
		}
	}
	return store, nil
}

func (s *executorGlobals) Read(name string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.values[name]
	if !exists {
		return nil, fmt.Errorf("global variable %q does not exist on this executor", name)
	}
	return value, nil
}

func (s *executorGlobals) Set(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = value
	return s.flushLocked()
}

func (s *executorGlobals) Increment(name string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	number, err := toFloat(s.values[name])
	if err != nil {
		return 0, fmt.Errorf("global variable %q holds a non-numeric value: %w", name, err)
	}
	number += delta
	s.values[name] = number
	if err := s.flushLocked(); err != nil {
		return 0, err
	}
	return number, nil
}

func (s *executorGlobals) Append(name string, item any) ([]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := toList(s.values[name])
	if err != nil {
		return nil, fmt.Errorf("global variable %q holds a non-list value: %w", name, err)
	}
	list = append(list, item)
	s.values[name] = list
	if err := s.flushLocked(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *executorGlobals) flushLocked() error {
	data, err := json.Marshal(s.values)
	if err != nil {
		return fmt.Errorf("encode executor global variables: %w", err)
	}
	if err := atomicWrite(s.path, data); err != nil {
		return fmt.Errorf("persist executor global variables: %w", err)
	}
	return nil
}

func toFloat(value any) (float64, error) {
	switch value := value.(type) {
	case nil:
		return 0, nil
	case float64:
		return value, nil
	case int:
		return float64(value), nil
	case int64:
		return float64(value), nil
	default:
		return 0, fmt.Errorf("value is not numeric")
	}
}

func toList(value any) ([]any, error) {
	if value == nil {
		return make([]any, 0), nil
	}
	if list, ok := value.([]any); ok {
		return list, nil
	}
	return nil, fmt.Errorf("value is not a list")
}
