// Package plugins discovers local plugin bundles and exposes diagnostic state.
package plugins

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/pkg/pluginapi"
)

// Manager owns plugin discovery. Bundles are explicit local installations only.
type Manager struct {
	mu            sync.RWMutex
	directory     string
	statuses      []domain.PluginStatus
	documentation []domain.DocumentationDocument
}

// NewManager creates a plugin manager rooted in the user-owned plugin directory.
func NewManager(directory string) *Manager { return &Manager{directory: directory} }

// Directory returns the current discovery root.
func (m *Manager) Directory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.directory
}

// SetDirectory updates the root used at the next discovery pass.
func (m *Manager) SetDirectory(directory string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.directory = directory
}

// Status returns a copy of the last discovery result.
func (m *Manager) Status() []domain.PluginStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]domain.PluginStatus, len(m.statuses))
	copy(statuses, m.statuses)
	return statuses
}

// Documentation returns validated Markdown pages from enabled, healthy plugin
// bundles. Its values have already been size and path checked during Reload,
// so callers never need to access a plugin directory themselves.
func (m *Manager) Documentation() []domain.DocumentationDocument {
	m.mu.RLock()
	defer m.mu.RUnlock()
	documents := make([]domain.DocumentationDocument, len(m.documentation))
	copy(documents, m.documentation)
	return documents
}

// Reload scans plugin.json manifests and validates their basic executable contract.
func (m *Manager) Reload() ([]domain.PluginStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	statuses := make([]domain.PluginStatus, 0)
	documents := make([]domain.DocumentationDocument, 0)
	err := filepath.WalkDir(m.directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && path != m.directory {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != "plugin.json" {
			return nil
		}
		bundle := readBundle(path)
		statuses = append(statuses, bundle.status)
		if bundle.status.Enabled && bundle.status.Healthy {
			documents = append(documents, bundle.documentation...)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan plugins: %w", err)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	m.statuses = statuses
	m.documentation = documents
	result := make([]domain.PluginStatus, len(statuses))
	copy(result, statuses)
	return result, nil
}

type manifest struct {
	pluginapi.Bundle
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

type discoveredBundle struct {
	status        domain.PluginStatus
	documentation []domain.DocumentationDocument
}

func readBundle(path string) discoveredBundle {
	status := domain.PluginStatus{Path: filepath.Dir(path), Enabled: false, Healthy: false}
	data, err := os.ReadFile(path)
	if err != nil {
		status.Error = err.Error()
		return discoveredBundle{status: status}
	}
	var manifest manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		status.Error = "invalid plugin.json: " + err.Error()
		return discoveredBundle{status: status}
	}
	status.ID, status.Name, status.Version, status.Description = manifest.ID, manifest.Name, manifest.Version, manifest.Description
	status.NodeCount = len(manifest.Nodes)
	if status.ID == "" || status.Name == "" || manifest.APIVersion != pluginapi.APIVersion {
		status.Error = "manifest needs id, name, and supported apiVersion"
		return discoveredBundle{status: status}
	}
	if manifest.Executable == "" {
		status.Error = "manifest needs an executable sidecar"
		return discoveredBundle{status: status}
	}
	executable := manifest.Executable
	if !filepath.IsAbs(executable) {
		executable = filepath.Join(filepath.Dir(path), executable)
	}
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		status.Error = "sidecar executable is unavailable"
		return discoveredBundle{status: status}
	}
	status.Enabled, status.Healthy = true, true
	documents, docsErr := readDocumentation(filepath.Dir(path), manifest)
	if docsErr != nil {
		// Documentation is optional. A faulty reference page must not make an
		// otherwise healthy automation plugin unavailable.
		status.DocumentationError = docsErr.Error()
	}
	return discoveredBundle{status: status, documentation: documents}
}

const maxDocumentationFileBytes = 1 << 20

func readDocumentation(root string, manifest manifest) ([]domain.DocumentationDocument, error) {
	documents := make([]domain.DocumentationDocument, 0, len(manifest.Documentation))
	seen := make(map[string]struct{}, len(manifest.Documentation))
	for _, entry := range manifest.Documentation {
		id := strings.TrimSpace(entry.ID)
		if id == "" || strings.TrimSpace(entry.Title) == "" || len(entry.CategoryPath) == 0 || strings.TrimSpace(entry.Path) == "" {
			return nil, fmt.Errorf("plugin documentation needs id, title, categoryPath, and path")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate plugin documentation id %q", id)
		}
		seen[id] = struct{}{}
		if filepath.IsAbs(entry.Path) || !strings.EqualFold(filepath.Ext(entry.Path), ".md") {
			return nil, fmt.Errorf("documentation path for %q must be a relative Markdown file", id)
		}
		fullPath := filepath.Clean(filepath.Join(root, entry.Path))
		relativePath, err := filepath.Rel(root, fullPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("documentation path for %q escapes the bundle", id)
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("documentation file for %q is unavailable", id)
		}
		if info.Size() > maxDocumentationFileBytes {
			return nil, fmt.Errorf("documentation file for %q exceeds 1 MiB", id)
		}
		markdown, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read documentation file for %q: %w", id, err)
		}
		category := make([]string, 0, len(entry.CategoryPath))
		for _, segment := range entry.CategoryPath {
			if trimmed := strings.TrimSpace(segment); trimmed != "" {
				category = append(category, trimmed)
			}
		}
		if len(category) == 0 {
			return nil, fmt.Errorf("documentation category for %q is empty", id)
		}
		documents = append(documents, domain.DocumentationDocument{
			DocumentationEntry: domain.DocumentationEntry{
				ID:        "plugin:" + manifest.ID + ":" + id,
				Title:     strings.TrimSpace(entry.Title),
				Summary:   strings.TrimSpace(entry.Summary),
				Category:  category,
				NodeTypes: append([]string(nil), entry.NodeTypes...),
				Source:    "plugin",
				PluginID:  manifest.ID,
			},
			Markdown: string(markdown),
		})
	}
	return documents, nil
}
