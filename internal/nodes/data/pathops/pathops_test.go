package pathops

import (
	"context"
	"reflect"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

func invocation(nodeType string, module nodes.Node, config map[string]any, inputs map[string]any) nodes.Invocation {
	return nodes.Invocation{
		Node:            domain.FlowNode{Type: nodeType, Data: map[string]any{"config": config}},
		Definition:      module.Definition(),
		SchemaVersion:   3,
		Config:          config,
		Inputs:          inputs,
		ConnectedInputs: map[string]bool{},
	}
}

func TestRegisterPathOpsNodes(t *testing.T) {
	registry := nodes.New()
	for _, register := range []func(nodes.Registrar) error{RegisterGetPathPart, RegisterBuildPath, RegisterCleanPath} {
		if err := register(registry); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	getPart, ok := registry.Get("data:get_path_part")
	if !ok {
		t.Fatal("data:get_path_part was not registered")
	}
	build, ok := registry.Get("data:build_path")
	if !ok {
		t.Fatal("data:build_path was not registered")
	}
	clean, ok := registry.Get("data:clean_path")
	if !ok {
		t.Fatal("data:clean_path was not registered")
	}

	partDefinition := getPart.Definition()
	if partDefinition.Mode != domain.NodePure || !partDefinition.PortContractOwned || partDefinition.Category != "Data" {
		t.Fatalf("get_path_part definition header = %#v", partDefinition)
	}
	if got := partDefinition.Inputs[0]; got.ID != "path" || got.DataType != domain.DataText || !got.Required {
		t.Fatalf("get_path_part path input = %#v", got)
	}
	if got := partDefinition.Outputs[0]; got.ID != "result" || got.DataType != domain.DataObject || len(got.Fields) != 5 {
		t.Fatalf("get_path_part result output = %#v", got)
	}

	buildDefinition := build.Definition()
	if buildDefinition.Mode != domain.NodePure || !buildDefinition.PortContractOwned {
		t.Fatalf("build_path definition header = %#v", buildDefinition)
	}
	if got := buildDefinition.Inputs[0]; got.ID != "parts" || got.DataType != domain.DataList || !got.Required {
		t.Fatalf("build_path parts input = %#v", got)
	}
	if got := buildDefinition.Outputs[0]; got.ID != "result" || got.DataType != domain.DataText {
		t.Fatalf("build_path result output = %#v", got)
	}

	cleanDefinition := clean.Definition()
	if cleanDefinition.Mode != domain.NodePure || !cleanDefinition.PortContractOwned {
		t.Fatalf("clean_path definition header = %#v", cleanDefinition)
	}
	if got := cleanDefinition.Inputs[0]; got.ID != "path" || got.DataType != domain.DataText || !got.Required {
		t.Fatalf("clean_path path input = %#v", got)
	}
	if got := cleanDefinition.Outputs[0]; got.ID != "result" || got.DataType != domain.DataText {
		t.Fatalf("clean_path result output = %#v", got)
	}
}

func TestGetPathPartSplitsCorrectly(t *testing.T) {
	module := NewGetPathPart()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": "/Work/file.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out, ok := result.Outputs["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not a record: %#v", result.Outputs["result"])
	}
	if got, want := out["ext"], ".txt"; got != want {
		t.Fatalf("ext = %q, want %q", got, want)
	}
	if got, want := out["name"], "file"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}

// The exact split expectations pin the node's slash-only contract: inputs
// may use either separator, and the results are identical on every platform
// this suite runs on.
func TestGetPathPartSplitsEveryComponent(t *testing.T) {
	module := NewGetPathPart()
	tests := []struct {
		name string
		path string
		want map[string]any
	}{
		{"regular file", "/Work/file.txt", map[string]any{"dir": "/Work/", "base": "file.txt", "name": "file", "ext": ".txt", "volume": ""}},
		{"windows drive path folds separators", "C:\\Work\\file.txt", map[string]any{"dir": "C:/Work/", "base": "file.txt", "name": "file", "ext": ".txt", "volume": "C:"}},
		{"double extension keeps the last", "dir/file.tar.gz", map[string]any{"dir": "dir/", "base": "file.tar.gz", "name": "file.tar", "ext": ".gz", "volume": ""}},
		{"trailing slash has no base", "a/b/", map[string]any{"dir": "a/b/", "base": "", "name": "", "ext": "", "volume": ""}},
		{"hidden file keeps its dot as extension", "a/.gitignore", map[string]any{"dir": "a/", "base": ".gitignore", "name": "", "ext": ".gitignore", "volume": ""}},
		{"trims surrounding whitespace", "  /Work/file.txt  ", map[string]any{"dir": "/Work/", "base": "file.txt", "name": "file", "ext": ".txt", "volume": ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation("data:get_path_part", module, map[string]any{}, map[string]any{"path": test.path}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(result.Outputs["result"], test.want) {
				t.Fatalf("result = %#v, want %#v", result.Outputs["result"], test.want)
			}
		})
	}
}

func TestGetPathPartMissingInputYieldsEmptyRecord(t *testing.T) {
	module := NewGetPathPart()
	result, err := module.Execute(context.Background(), invocation("data:get_path_part", module, map[string]any{}, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"dir": "", "base": "", "name": "", "ext": "", "volume": ""}
	if !reflect.DeepEqual(result.Outputs["result"], want) {
		t.Fatalf("result = %#v, want the empty record %#v", result.Outputs["result"], want)
	}
}

func TestBuildPathJoinsListItems(t *testing.T) {
	module := NewBuildPath()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"parts": []any{"C:\\Work", "subfolder", "file.txt"}},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["result"]; got == "" {
		t.Fatalf("expected non-empty joined path, got %q", got)
	}
}

func TestCleanPathNormalizes(t *testing.T) {
	module := NewCleanPath()
	result, err := module.Execute(context.Background(), nodes.Invocation{
		Inputs: map[string]any{"path": "C:\\Work\\..\\Work\\.\\file.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["result"]; got == "" {
		t.Fatalf("expected non-empty cleaned path")
	}
}

func TestBuildPathJoinsAndCleansParts(t *testing.T) {
	module := NewBuildPath()
	tests := []struct {
		name  string
		parts []any
		want  string
	}{
		{"joins with separators", []any{"a", "b", "c"}, "a/b/c"},
		{"windows separators are normalized", []any{"C:\\Work", "subfolder", "file.txt"}, "C:/Work/subfolder/file.txt"},
		{"absolute parts", []any{"/var", "log", "syslog"}, "/var/log/syslog"},
		{"cleans dot segments", []any{"a", "b", ".", "c"}, "a/b/c"},
		{"resolves parent segments", []any{"a", "b", "..", "c"}, "a/c"},
		{"skips blank and non-string items", []any{"a", "", 42.0, "b"}, "a/b"},
		{"single part is returned as-is", []any{"file.txt"}, "file.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation("data:build_path", module, map[string]any{}, map[string]any{"parts": test.parts}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := result.Outputs["result"]; got != test.want {
				t.Fatalf("result = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildPathFallsBackToConfiguredParts(t *testing.T) {
	module := NewBuildPath()
	tests := []struct {
		name   string
		config map[string]any
		inputs map[string]any
		want   string
	}{
		// A wired but empty list leaves the inspector textarea in charge.
		{"newline-separated textarea", map[string]any{"parts": " a \n\n b \n"}, map[string]any{"parts": []any{}}, "a/b"},
		{"list editor shape", map[string]any{"parts": []any{"a", "b"}}, map[string]any{}, "a/b"},
		{"non-string config is ignored", map[string]any{"parts": 42.0}, map[string]any{"parts": []any{"a", "b"}}, "a/b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation("data:build_path", module, test.config, test.inputs), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := result.Outputs["result"]; got != test.want {
				t.Fatalf("result = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildPathWithoutPartsProducesNoOutputs(t *testing.T) {
	module := NewBuildPath()
	result, err := module.Execute(context.Background(), invocation("data:build_path", module, map[string]any{}, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Outputs != nil {
		t.Fatalf("outputs = %#v, want none without configured parts", result.Outputs)
	}
}

func TestCleanPathNormalizesExactly(t *testing.T) {
	module := NewCleanPath()
	tests := []struct {
		name string
		path string
		want string
	}{
		{"removes dot and parent segments", "a/./b/../b/c/", "a/b/c"},
		{"windows separators are normalized", "C:\\Work\\..\\Work\\.\\file.txt", "C:/Work/file.txt"},
		{"collapses doubled separators and up-levels", "/var/log/../..//tmp/.", "/tmp"},
		{"already clean path", "/Work/file.txt", "/Work/file.txt"},
		{"trims surrounding whitespace", "  /Work/file.txt  ", "/Work/file.txt"},
		{"blank path cleans to dot", "   ", "."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := module.Execute(context.Background(), invocation("data:clean_path", module, map[string]any{}, map[string]any{"path": test.path}), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := result.Outputs["result"]; got != test.want {
				t.Fatalf("result = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCleanPathMissingInputCleansToDot(t *testing.T) {
	module := NewCleanPath()
	result, err := module.Execute(context.Background(), invocation("data:clean_path", module, map[string]any{}, map[string]any{}), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := result.Outputs["result"]; got != "." {
		t.Fatalf("result = %q, want the Clean(\"\") dot", got)
	}
}
