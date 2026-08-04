package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/neuropipe/pkg/pluginapi"
)

func TestReloadExposesOnlyValidatedPluginDocumentation(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "sample")
	if err := os.MkdirAll(filepath.Join(bundle, "docs"), 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(bundle, "sidecar.exe"), []byte("stub"), 0o600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(bundle, "docs", "node.md"), []byte("# Node\n"), 0o600); err != nil { t.Fatal(err) }
	manifest := `{"id":"sample","name":"Sample","version":"1.0.0","apiVersion":"v1","executable":"sidecar.exe","documentation":[{"id":"node","title":"Sample node","categoryPath":["Node reference","Plugins"],"path":"docs/node.md","nodeTypes":["plugin:sample"]}]}`
	if err := os.WriteFile(filepath.Join(bundle, "plugin.json"), []byte(manifest), 0o600); err != nil { t.Fatal(err) }
	manager := NewManager(root)
	statuses, err := manager.Reload()
	if err != nil || len(statuses) != 1 || !statuses[0].Healthy { t.Fatalf("Reload() = %#v, %v", statuses, err) }
	documents := manager.Documentation()
	if len(documents) != 1 || documents[0].ID != "plugin:sample:node" { t.Fatalf("Documentation() = %#v", documents) }
}

func TestPluginDocumentationRejectsBundleEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "outside.md"), []byte("# outside"), 0o600); err != nil { t.Fatal(err) }
	_, err := readDocumentation(filepath.Join(root, "bundle"), manifest{Bundle: pluginapi.Bundle{ID: "sample", Documentation: []pluginapi.DocumentationEntry{{ID: "escape", Title: "Escape", CategoryPath: []string{"Reference"}, Path: "../outside.md"}}}})
	if err == nil { t.Fatal("readDocumentation() accepted a path outside the bundle") }
}
