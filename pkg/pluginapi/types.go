// Package pluginapi defines the stable, Go-first contract for Neuropipe extensions.
package pluginapi

import (
	"context"
	"encoding/json"
)

// APIVersion changes only for breaking plugin contract changes.
const APIVersion = "v1"

// Bundle describes one independently versioned plugin process.
type Bundle struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Version       string               `json:"version"`
	Description   string               `json:"description"`
	APIVersion    string               `json:"apiVersion"`
	Nodes         []NodeSpec           `json:"nodes"`
	Documentation []DocumentationEntry `json:"documentation,omitempty"`
}

// DocumentationEntry declares an optional local Markdown page bundled with a
// plugin. Paths are validated by Neuropipe before files are ever exposed to
// the renderer.
type DocumentationEntry struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	CategoryPath []string `json:"categoryPath"`
	Summary      string   `json:"summary,omitempty"`
	Path         string   `json:"path"`
	NodeTypes    []string `json:"nodeTypes,omitempty"`
}

// NodeSpec declares one node that can be rendered through the generic editor.
type NodeSpec struct {
	ID           string       `json:"id"`
	Kind         string       `json:"kind"`
	Label        string       `json:"label"`
	Description  string       `json:"description"`
	Icon         string       `json:"icon"`
	Color        string       `json:"color"`
	Capabilities []string     `json:"capabilities"`
	Outputs      []OutputPort `json:"outputs"`
	Fields       []FieldSpec  `json:"fields"`
}

type OutputPort struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type FieldSpec struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
	Secret   bool   `json:"secret"`
}

// Action is implemented by a plugin node's sidecar handler.
type Action interface {
	Validate(ctx context.Context, config json.RawMessage) error
	Execute(ctx context.Context, config json.RawMessage, input map[string]any) (map[string]any, error)
}
