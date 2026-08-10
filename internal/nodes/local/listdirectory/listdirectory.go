// Package listdirectory registers the local directory-listing Blueprint node.
package listdirectory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the List Directory module implementation.
func New() Node {
	pathType := typespec.String()
	fileType := fileRecordType()
	filesType := domain.TypeSpec{Kind: domain.TypeList, Element: &fileType}
	return Node{
		Metadata: domain.NodeDefinition{
			Type:        "action:list_directory",
			Category:    "Local",
			Label:       "List Directory",
			Description: "List files and folders from an approved local directory.",
			Icon:        "folder-open",
			Color:       "#c4b5fd",
			Mode:        domain.NodeImpure,
			Inputs: []domain.NodePort{
				{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
				{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &pathType, Color: "#e879f9", Required: true, MaxConnections: 1},
			},
			Outputs: []domain.NodePort{
				{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
				{
					ID:        "result",
					Label:     "Files",
					Kind:      domain.PinData,
					Direction: domain.PinOutput,
					DataType:  domain.DataList,
					Type:      &filesType,
					Color:     "#facc15",
					Fields: []domain.DataField{
						{Path: "name", Label: "Name", DataType: domain.DataText, Description: "Entry name."},
						{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Absolute entry path."},
						{Path: "size", Label: "Size", DataType: domain.DataNumber, Description: "Entry size in bytes."},
						{Path: "type", Label: "Type", DataType: domain.DataText, Description: "file, directory, or symlink."},
						{Path: "createdAt", Label: "Created at", DataType: domain.DataText, Description: "Creation time in RFC 3339 format, when available.", Optional: true},
						{Path: "updatedAt", Label: "Updated at", DataType: domain.DataText, Description: "Last update time in RFC 3339 format."},
					},
					MaxConnections: 1,
				},
			},
			Fields: []domain.ConfigField{
				{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work", Required: true},
			},
			Capabilities:  []domain.Capability{domain.CapabilityFileRead},
			DefaultConfig: map[string]any{},
			Source:        "builtin",
		},
		Executor: execute,
	}
}

// Register contributes the complete List Directory module to the node registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("list directory cancelled: %w", err)
	}
	path, _ := invocation.Inputs["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("directory path is required")
	}
	entries, err := os.ReadDir(filepath.Clean(path))
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("list directory: %w", err)
	}

	files := make([]any, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("list directory cancelled: %w", err)
		}
		info, err := entry.Info()
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("inspect directory entry %q: %w", entry.Name(), err)
		}
		item := map[string]any{
			"name":      entry.Name(),
			"path":      filepath.Join(path, entry.Name()),
			"size":      info.Size(),
			"type":      entryType(entry, info),
			"updatedAt": info.ModTime().UTC().Format(time.RFC3339Nano),
		}
		if createdAt, found := creationTime(info); found {
			item["createdAt"] = createdAt.UTC().Format(time.RFC3339Nano)
		}
		files = append(files, item)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": files}, Ports: []string{"out"}}, nil
}

func entryType(entry os.DirEntry, info os.FileInfo) string {
	if entry.Type()&os.ModeSymlink != 0 {
		return "symlink"
	}
	if info.IsDir() {
		return "directory"
	}
	return "file"
}

func fileRecordType() domain.TypeSpec {
	return domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "name", Name: "name", Type: typespec.String()},
		{ID: "path", Name: "path", Type: typespec.String()},
		{ID: "size", Name: "size", Type: typespec.Int()},
		{ID: "type", Name: "type", Type: typespec.String()},
		{ID: "createdAt", Name: "createdAt", Type: typespec.String(), Optional: true},
		{ID: "updatedAt", Name: "updatedAt", Type: typespec.String()},
	}}
}
