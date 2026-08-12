// Package variables owns the workspace-scoped, persisted Global Variables
// store. Memory is authoritative and guarded by a single mutex; a single
// background goroutine batches dirty values to SQLite at most once a second
// and once more on graceful shutdown. The graph engine and Wails façade reach
// it through focused interfaces, never through persistence internals.
package variables

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Operation identifies the atomic read-modify-write modes supported by the
// Set Global Variable node. Set overwrites unconditionally; Increment adds a
// delta to a number; Append mutates a list in place.
type Operation string

const (
	OperationSet       Operation = "set"
	OperationIncrement Operation = "increment"
	OperationAppend    Operation = "append"
)

// Service is the shared, process-wide variable store. Declaration mutations
// (create/update/delete) talk to the database synchronously and then update
// memory under the write lock; data mutations (write/increment/append) only
// touch memory and mark the store dirty for the next flush.
type Service struct {
	store *persistence.Store

	mu     sync.RWMutex
	defs   map[string]domain.GlobalVariable // by name
	byID   map[string]domain.GlobalVariable // by ID
	values map[string]any                   // live values; defaults materialise on read
	dirty  map[string]struct{}              // names written since last flush
	booted bool                             // declared once Start has run

	flushC  chan struct{}
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool

	// flushInterval is overridable by tests; the batching contract guarantees
	// at most one flush per interval, never per second *intentionally* less.
	flushInterval time.Duration
}

// New loads the declaration catalogue and any persisted live values. Startup
// failure is fatal to composition because every execution would otherwise see
// an inconsistent store.
func New(store *persistence.Store) (*Service, error) {
	defs, err := store.ListGlobalVariables(context.Background())
	if err != nil {
		return nil, err
	}
	persisted, err := store.LoadGlobalVariableValues(context.Background())
	if err != nil {
		return nil, err
	}
	service := &Service{
		store:         store,
		defs:          make(map[string]domain.GlobalVariable, len(defs)),
		byID:          make(map[string]domain.GlobalVariable, len(defs)),
		values:        make(map[string]any, len(persisted)),
		dirty:         make(map[string]struct{}),
		flushC:        make(chan struct{}, 1),
		flushInterval: time.Second,
	}
	for _, definition := range defs {
		service.defs[definition.Name] = definition
		service.byID[definition.ID] = definition
		if value, ok := persisted[definition.Name]; ok {
			service.values[definition.Name] = value
		} else {
			service.values[definition.Name] = definition.DefaultValue
		}
	}
	return service, nil
}

// Start launches the owned flusher goroutine. It is idempotent: a second call
// only notes the service as booted without spawning a duplicate goroutine.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()
	flushCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.flushLoop(flushCtx)
	// The outer context allows an embedding app to request a final flush via
	// context cancellation through Stop; nothing here reacts to ctx directly.
	_ = ctx
}

// Stop forces a final flush of any dirty values and waits for the flusher.
// It is idempotent and safe to call before Start.
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	s.wg.Wait()
	// The channel from the flusher signals completion of the final flush;
	// an extra pass is harmless because Stop itself already severed triggers.
	_ = s.flush(context.Background())
}

func (s *Service) flushLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Final flush happens on Stop after Wait so ordering is explicit.
			return
		case <-ticker.C:
			_ = s.flush(context.Background())
		case <-s.flushC:
			_ = s.flush(context.Background())
		}
	}
}

// flush copies dirty values under a short read lock, then upserts. A Set that
// arrives during the copy is simply picked up by the next flush.
func (s *Service) flush(ctx context.Context) error {
	s.mu.RLock()
	if len(s.dirty) == 0 {
		s.mu.RUnlock()
		return nil
	}
	dirtyNames := make([]string, 0, len(s.dirty))
	for name := range s.dirty {
		dirtyNames = append(dirtyNames, name)
	}
	sort.Strings(dirtyNames)
	snapshot := make(map[string]any, len(dirtyNames))
	for _, name := range dirtyNames {
		snapshot[name] = s.values[name]
	}
	s.mu.RUnlock()
	if err := s.store.FlushGlobalVariableValues(ctx, snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	for name := range snapshot {
		delete(s.dirty, name)
	}
	s.mu.Unlock()
	return nil
}

// acknowledgeName resolves a config-supplied name to the canonical declaration.
// Unknown names are a hard runtime failure: the config referenced something
// that has been deleted or was hand-edited, so no silent stub is created.
func (s *Service) acknowledgeName(name string) (domain.GlobalVariable, error) {
	definition, exists := s.defs[name]
	if !exists {
		return domain.GlobalVariable{}, fmt.Errorf("global variable %q is not declared", name)
	}
	return definition, nil
}

// Read returns the live value for a declared variable. The first read after
// startup yields the declaration's default when no run has written yet.
func (s *Service) Read(name string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, err := s.acknowledgeName(name)
	if err != nil {
		return nil, err
	}
	if value, exists := s.values[name]; exists {
		return value, nil
	}
	return definition.DefaultValue, nil
}

// ReadForNode exposes Read through the narrow node-facing interface. It does
// not distinguish "never written" from a persisted value - declaration
// defaults are served transparently.
func (s *Service) ReadForNode(name string) (any, bool) {
	value, err := s.Read(name)
	if err != nil {
		return nil, false
	}
	return value, true
}

// validateWrite guards type safety before a value reaches shared memory. `set`
// operation coerces only through JSON-shaped inputs (no implicit conversions),
// and numeric writes accept int/uint/float variants because the validator
// widens ints to float for the "number" data type.
func validateWrite(declared domain.DataType, value any) error {
	spec := typespec.FromDataType(declared)
	if err := typespec.ValidateValue(value, spec); err != nil {
		return fmt.Errorf("global variable value does not match declared type %q: %w", declared, err)
	}
	return nil
}

// Set stores a value for the declaration, validating against the declared type.
func (s *Service) Set(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, err := s.acknowledgeName(name)
	if err != nil {
		return err
	}
	if definition.DataType != domain.DataAny {
		if err := validateWrite(definition.DataType, value); err != nil {
			return err
		}
	}
	s.values[name] = value
	s.dirty[name] = struct{}{}
	s.notifyFlush()
	return nil
}

// Increment atomically adds delta to a number variable. The whole
// read-modify-write is guarded by the write lock, so two concurrent pipeline
// increments both land.
func (s *Service) Increment(name string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, err := s.acknowledgeName(name)
	if err != nil {
		return 0, err
	}
	if definition.DataType != domain.DataNumber {
		return 0, fmt.Errorf("global variable %q is declared as %q, cannot increment", name, definition.DataType)
	}
	current, exists := s.values[name]
	if !exists {
		current = definition.DefaultValue
	}
	number, err := toFloat(current)
	if err != nil {
		return 0, fmt.Errorf("global variable %q holds a non-numeric value: %w", name, err)
	}
	number += delta
	s.values[name] = number
	s.dirty[name] = struct{}{}
	s.notifyFlush()
	return number, nil
}

// Append atomically appends an item to a list variable and returns the
// resulting list.
func (s *Service) Append(name string, item any) ([]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, err := s.acknowledgeName(name)
	if err != nil {
		return nil, err
	}
	if definition.DataType != domain.DataList {
		return nil, fmt.Errorf("global variable %q is declared as %q, cannot append", name, definition.DataType)
	}
	current, exists := s.values[name]
	if !exists {
		current = definition.DefaultValue
	}
	list, err := toList(current)
	if err != nil {
		return nil, fmt.Errorf("global variable %q holds a non-list value: %w", name, err)
	}
	list = append(list, item)
	s.values[name] = list
	s.dirty[name] = struct{}{}
	s.notifyFlush()
	return list, nil
}

func (s *Service) notifyFlush() {
	select {
	case s.flushC <- struct{}{}:
	default:
		// Channel buffer is 1; a pending trigger is already enough.
	}
}

// List reports declarations with their current in-memory value for the
// Variables view and the catalogue picklist.
func (s *Service) List() ([]domain.GlobalVariableSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]domain.GlobalVariableSummary, 0, len(s.byID))
	for _, definition := range s.byID {
		value, exists := s.values[definition.Name]
		if !exists {
			value = definition.DefaultValue
		}
		summaries = append(summaries, domain.GlobalVariableSummary{
			ID:          definition.ID,
			Name:        definition.Name,
			Description: definition.Description,
			DataType:    definition.DataType,
			Value:       value,
			UpdatedAt:   definition.UpdatedAt,
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	return summaries, nil
}

// Get returns a single declaration with the live value for the create/edit
// dialog. Errors when the ID no longer refers to a known declaration.
func (s *Service) Get(id string) (domain.GlobalVariable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, exists := s.byID[id]
	if !exists {
		return domain.GlobalVariable{}, fmt.Errorf("global variable %q not found", id)
	}
	if value, exists := s.values[definition.Name]; exists {
		definition.Value = value
	} else {
		definition.Value = definition.DefaultValue
	}
	return definition, nil
}

// Create declares a new variable. The write path remains the narrow runtime
// surface; creation still goes through the application layer because declared
// state is the user's catalogue, not execution data.
func (s *Service) Create(ctx context.Context, variable domain.GlobalVariable) (domain.GlobalVariable, error) {
	stored, err := s.store.CreateGlobalVariable(ctx, variable)
	if err != nil {
		return domain.GlobalVariable{}, err
	}
	s.mu.Lock()
	s.defs[stored.Name] = stored
	s.byID[stored.ID] = stored
	if _, exists := s.values[stored.Name]; !exists {
		s.values[stored.Name] = stored.DefaultValue
	}
	s.mu.Unlock()
	return stored, nil
}

// Update applies restricted edits (description, default value) and then
// replaces the in-memory declaration. Existing live values are kept so a
// default edit does not silently overwrite written state.
func (s *Service) Update(ctx context.Context, variable domain.GlobalVariable) (domain.GlobalVariable, error) {
	stored, err := s.store.UpdateGlobalVariableMetadata(ctx, variable)
	if err != nil {
		return domain.GlobalVariable{}, err
	}
	s.mu.Lock()
	s.defs[stored.Name] = stored
	s.byID[stored.ID] = stored
	if _, exists := s.values[stored.Name]; !exists {
		s.values[stored.Name] = stored.DefaultValue
	}
	s.mu.Unlock()
	return stored, nil
}

// Delete guards references at the persistence layer first, then drops memory.
func (s *Service) Delete(ctx context.Context, id string) error {
	stored, err := s.Get(id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteGlobalVariable(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.byID, id)
	delete(s.defs, stored.Name)
	delete(s.values, stored.Name)
	delete(s.dirty, stored.Name)
	s.mu.Unlock()
	return nil
}

// VariableOptions powers the dynamic picklist injection inside catalog.Registry.
// Sorted for stable editor datalists, with the type appended so users see at a
// glance what they are connecting to.
func (s *Service) VariableOptions() []domain.Option {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.defs))
	for name := range s.defs {
		names = append(names, name)
	}
	sort.Strings(names)
	options := make([]domain.Option, 0, len(names))
	for _, name := range names {
		definition := s.defs[name]
		options = append(options, domain.Option{Value: name, Label: fmt.Sprintf("%s (%s)", name, definition.DataType)})
	}
	return options
}

// VariableType returns the declared data type, so the Get node resolver can
// expose a typed output pin. Unknown names surface as `false`.
func (s *Service) VariableType(name string) (domain.DataType, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, exists := s.defs[name]
	if !exists {
		return domain.DataAny, false
	}
	return definition.DataType, true
}

func toFloat(value any) (float64, error) {
	switch number := value.(type) {
	case float64:
		return number, nil
	case float32:
		return float64(number), nil
	case int:
		return float64(number), nil
	case int8:
		return float64(number), nil
	case int16:
		return float64(number), nil
	case int32:
		return float64(number), nil
	case int64:
		return float64(number), nil
	case uint:
		return float64(number), nil
	case uint8:
		return float64(number), nil
	case uint16:
		return float64(number), nil
	case uint32:
		return float64(number), nil
	case uint64:
		return float64(number), nil
	}
	return 0, fmt.Errorf("value of type %T is not numeric", value)
}

func toList(value any) ([]any, error) {
	if list, ok := value.([]any); ok {
		return append([]any(nil), list...), nil
	}
	return nil, fmt.Errorf("value of type %T is not a list", value)
}
