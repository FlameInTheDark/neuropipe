// Package pathops registers pure path-utility Blueprint nodes:
//   - data:get_path_part — split a path into dir/base/name/ext/volume
//   - data:build_path     — join a list of path parts
//   - data:clean_path     — normalize a path
//
// All three nodes use slash-only path semantics so one pipeline produces the
// same text on every platform: backslash separators are folded into forward
// slashes on input and results always use forward slashes. Forward-slash
// paths are accepted by the Windows file APIs, so the normalized output
// stays directly usable on every supported OS.
package pathops

import (
	"context"
	"path"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// ---------- Get Path Part ----------

type getPathPartNode = nodes.Implementation

var _ nodes.Node = getPathPartNode{}

func NewGetPathPart() getPathPartNode {
	return getPathPartNode{Metadata: getPathPartDefinition(), Executor: executeGetPathPart}
}

func RegisterGetPathPart(registrar nodes.Registrar) error {
	return registrar.Register(NewGetPathPart())
}

func getPathPartDefinition() domain.NodeDefinition {
	textType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "dir", Name: "dir", Type: typespec.String()},
		{ID: "base", Name: "base", Type: typespec.String()},
		{ID: "name", Name: "name", Type: typespec.String()},
		{ID: "ext", Name: "ext", Type: typespec.String()},
		{ID: "volume", Name: "volume", Type: typespec.String()},
	}}
	return domain.NodeDefinition{
		Type:        "data:get_path_part",
		Category:    "Data",
		Label:       "Get Path Part",
		Description: "Split a filesystem path into directory, base name, file name, extension, and volume.",
		Icon:        "route",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1, Fields: []domain.DataField{
				{Path: "dir", Label: "Directory", DataType: domain.DataText},
				{Path: "base", Label: "Base name", DataType: domain.DataText},
				{Path: "name", Label: "File name", DataType: domain.DataText},
				{Path: "ext", Label: "Extension", DataType: domain.DataText},
				{Path: "volume", Label: "Volume", DataType: domain.DataText},
			}},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\file.txt", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeGetPathPart(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	input, _ := invocation.Inputs["path"].(string)
	value := normalizeSeparators(strings.TrimSpace(input))
	dir, base := splitPath(value)
	ext := pathExt(base)
	name := strings.TrimSuffix(base, ext)
	return nodes.ExecutionResult{Outputs: map[string]any{
		"result": map[string]any{
			"dir":    dir,
			"base":   base,
			"name":   name,
			"ext":    ext,
			"volume": volumeName(value),
		},
	}}, nil
}

// ---------- Build Path ----------

type buildPathNode = nodes.Implementation

var _ nodes.Node = buildPathNode{}

func NewBuildPath() buildPathNode {
	return buildPathNode{Metadata: buildPathDefinition(), Executor: executeBuildPath}
}

func RegisterBuildPath(registrar nodes.Registrar) error { return registrar.Register(NewBuildPath()) }

func buildPathDefinition() domain.NodeDefinition {
	textType := typespec.String()
	partsType := domain.TypeSpec{Kind: domain.TypeList, Element: &textType}
	return domain.NodeDefinition{
		Type:        "data:build_path",
		Category:    "Data",
		Label:       "Build Path",
		Description: "Join a list of path parts into a single normalized path.",
		Icon:        "route",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "parts", Label: "Parts", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataList, Type: &partsType, Color: "#facc15", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "parts", Label: "Parts", Kind: "textarea", Placeholder: "C:\\Work\nsubfolder\nfile.txt", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeBuildPath(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	parts := toStringList(invocation.Inputs["parts"])
	if len(parts) == 0 {
		parts = toStringList(invocation.Config["parts"])
	}
	if len(parts) == 0 {
		return nodes.ExecutionResult{}, nil
	}
	normalized := make([]string, len(parts))
	for index, part := range parts {
		normalized[index] = normalizeSeparators(part)
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"result": path.Join(normalized...)}}, nil
}

func toStringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		// Some inspectors persist this as a newline-separated string. Support
		// both shapes so the inspector can use a textarea without breaking the
		// wired contract.
		if raw, ok := value.(string); ok {
			var lines []string
			for _, line := range strings.Split(raw, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					lines = append(lines, trimmed)
				}
			}
			return lines
		}
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

// ---------- Clean Path ----------

type cleanPathNode = nodes.Implementation

var _ nodes.Node = cleanPathNode{}

func NewCleanPath() cleanPathNode {
	return cleanPathNode{Metadata: cleanPathDefinition(), Executor: executeCleanPath}
}

func RegisterCleanPath(registrar nodes.Registrar) error { return registrar.Register(NewCleanPath()) }

func cleanPathDefinition() domain.NodeDefinition {
	textType := typespec.String()
	return domain.NodeDefinition{
		Type:        "data:clean_path",
		Category:    "Data",
		Label:       "Clean Path",
		Description: "Normalize a filesystem path by removing redundant separators and resolving . and .. segments.",
		Icon:        "route",
		Color:       "#22c55e",
		Mode:        domain.NodePure,
		Inputs: []domain.NodePort{
			{ID: "path", Label: "Path", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "C:\\Work\\.\\..\\Work\\file.txt", Required: true},
		},
		DefaultConfig:     map[string]any{},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func executeCleanPath(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	input, _ := invocation.Inputs["path"].(string)
	value := normalizeSeparators(strings.TrimSpace(input))
	return nodes.ExecutionResult{Outputs: map[string]any{"result": path.Clean(value)}}, nil
}

// ---------- slash-only path helpers ----------

// normalizeSeparators folds Windows separators into forward slashes so the
// nodes behave identically on every platform.
func normalizeSeparators(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

// splitPath splits value into its directory (including the trailing
// separator) and base name using slash semantics.
func splitPath(value string) (dir, base string) {
	index := strings.LastIndex(value, "/")
	if index < 0 {
		return "", value
	}
	return value[:index+1], value[index+1:]
}

// pathExt returns the suffix of base beginning at its final dot, mirroring
// filepath.Ext for slash-separated paths.
func pathExt(base string) string {
	index := strings.LastIndex(base, ".")
	if index < 0 {
		return ""
	}
	return base[index:]
}

// volumeName returns the Windows drive-letter prefix (such as "C:") when
// value starts with one, so Windows-style paths split the same way on every
// platform.
func volumeName(value string) string {
	if len(value) < 2 || value[1] != ':' || !isASCIILetter(value[0]) {
		return ""
	}
	return value[:2]
}

func isASCIILetter(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z'
}
