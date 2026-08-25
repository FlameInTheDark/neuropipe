package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// Authoring is the seam between the assistant's authoring tools and the
// application's pipeline/function orchestration (validation, persistence,
// publish side-effects such as schedule reloads and executor deployment).
// Package app implements it; package chat must not import app.
type Authoring interface {
	// ValidatePipeline reports why a definition cannot be saved or published.
	ValidatePipeline(def domain.FlowDefinition) error
	// CreatePipelineDraft stores a new editable pipeline.
	CreatePipelineDraft(ctx context.Context, name, description string, def domain.FlowDefinition) (domain.Pipeline, error)
	// SavePipelineDraft updates name/description/definition of one draft.
	SavePipelineDraft(ctx context.Context, id, name, description string, def domain.FlowDefinition) (domain.Pipeline, error)
	// GetPipelineFull returns the editor-ready aggregate including its draft.
	GetPipelineFull(ctx context.Context, id string) (domain.Pipeline, error)
	// PublishPipeline snapshots the draft as an immutable revision and
	// activates it (bindings, schedules, executor deployment).
	PublishPipeline(ctx context.Context, id string) (domain.Pipeline, error)
	// DeletePipeline permanently removes the pipeline and its data.
	DeletePipeline(ctx context.Context, id string) error

	// ValidateFunction reports why a custom function cannot be saved.
	ValidateFunction(fn domain.CustomFunction) error
	// SaveFunctionDraft creates (empty ID) or updates a function draft.
	SaveFunctionDraft(ctx context.Context, fn domain.CustomFunction) (domain.CustomFunction, error)
	// GetFunction returns the full function aggregate.
	GetFunction(ctx context.Context, id string) (domain.CustomFunction, error)
	// ListFunctions returns published + draft summaries.
	ListFunctions(ctx context.Context) ([]domain.FunctionSummary, error)
	// PublishFunction snapshots the draft for use inside graphs.
	PublishFunction(ctx context.Context, fn domain.CustomFunction) (domain.CustomFunction, error)
	// DeleteFunction removes the function.
	DeleteFunction(ctx context.Context, id string) error
}

// nodeCatalogEntry is one compact list_nodes row.
type nodeCatalogEntry struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// listNodeEntries flattens the catalog for the assistant, optionally filtered
// by a case-insensitive substring over type/label/description.
func listNodeEntries(registry *catalog.Registry, query string) []nodeCatalogEntry {
	needle := strings.ToLower(strings.TrimSpace(query))
	definitions := registry.All()
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Type < definitions[j].Type })
	entries := make([]nodeCatalogEntry, 0, len(definitions))
	for _, definition := range definitions {
		if needle != "" &&
			!strings.Contains(strings.ToLower(definition.Type), needle) &&
			!strings.Contains(strings.ToLower(definition.Label), needle) &&
			!strings.Contains(strings.ToLower(definition.Description), needle) {
			continue
		}
		entries = append(entries, nodeCatalogEntry{
			Type:        definition.Type,
			Label:       definition.Label,
			Category:    definition.Category,
			Description: definition.Description,
		})
	}
	return entries
}

// pinContract is the machine-readable description of one node pin.
type pinContract struct {
	ID             string         `json:"id"`
	Label          string         `json:"label,omitempty"`
	Kind           string         `json:"kind"`
	DataType       string         `json:"dataType,omitempty"`
	TypeSpec       any            `json:"typeSpec,omitempty"`
	Required       bool           `json:"required,omitempty"`
	Default        any            `json:"default,omitempty"`
	MaxConnections int            `json:"maxConnections,omitempty"`
	Fields         []dataFieldRef `json:"fields,omitempty"`
}

type dataFieldRef struct {
	Path  string `json:"path"`
	Label string `json:"label,omitempty"`
	Kind  string `json:"dataType,omitempty"`
}

// fieldContract describes one config field key.
type fieldContract struct {
	Key         string   `json:"key"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	VisibleWhen string   `json:"visibleWhen,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

// nodeContract is the full get_node_contract payload.
type nodeContract struct {
	Type         string          `json:"type"`
	Label        string          `json:"label"`
	Category     string          `json:"category"`
	Description  string          `json:"description"`
	Mode         string          `json:"executionMode,omitempty"`
	TriggerKind  string          `json:"triggerKind,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Inputs       []pinContract   `json:"inputs"`
	Outputs      []pinContract   `json:"outputs"`
	Fields       []fieldContract `json:"fields"`
}

func pinsToContracts(pins []domain.NodePort) []pinContract {
	result := make([]pinContract, 0, len(pins))
	for _, pin := range pins {
		contract := pinContract{
			ID:             pin.ID,
			Label:          pin.Label,
			Kind:           string(pin.Kind),
			DataType:       string(pin.DataType),
			TypeSpec:       pin.Type,
			Required:       pin.Required,
			Default:        pin.Default,
			MaxConnections: pin.MaxConnections,
		}
		for _, field := range pin.Fields {
			contract.Fields = append(contract.Fields, dataFieldRef{Path: field.Path, Label: field.Label, Kind: string(field.DataType)})
		}
		result = append(result, contract)
	}
	return result
}

func fieldsToContracts(definition domain.NodeDefinition, values map[string]any) []fieldContract {
	result := make([]fieldContract, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		contract := fieldContract{Key: field.Name, Label: field.Label, Type: field.Kind, Required: field.Required, Secret: field.Secret, VisibleWhen: field.VisibleWhen}
		if value, ok := values[field.Name]; ok {
			contract.Default = value
		}
		for _, option := range field.Options {
			contract.Options = append(contract.Options, option.Value)
		}
		result = append(result, contract)
	}
	return result
}

// nodeContractFor builds the get_node_contract payload. Dynamic nodes whose
// pins depend on configuration resolve against empty config so the model sees
// the base contract; per-instance contracts arrive through save validation.
func nodeContractFor(registry *catalog.Registry, nodeType string) (nodeContract, error) {
	definition, ok := registry.Get(strings.TrimSpace(nodeType))
	if !ok {
		return nodeContract{}, fmt.Errorf("unknown node type %q; call list_nodes to see available types", nodeType)
	}
	contract := nodeContract{
		Type:         definition.Type,
		Label:        definition.Label,
		Category:     definition.Category,
		Description:  definition.Description,
		Mode:         string(definition.Mode),
		TriggerKind:  string(definition.TriggerKind),
		Capabilities: capabilitiesToStrings(definition.Capabilities),
		Inputs:       pinsToContracts(definition.Inputs),
		Outputs:      pinsToContracts(definition.Outputs),
		Fields:       fieldsToContracts(definition, definition.DefaultConfig),
	}
	return contract, nil
}

func capabilitiesToStrings(capabilities []domain.Capability) []string {
	values := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		values = append(values, string(capability))
	}
	return values
}
