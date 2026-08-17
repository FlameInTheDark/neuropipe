// Package download registers the Download from Web Blueprint node. It fetches
// a single URL with HTTP GET and writes the response body to a local path
// derived from a destination directory and the URL's file name.
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// New creates the Download module implementation.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Executor: execute}
}

// Register contributes the complete Download module to the registry.
func Register(registrar nodes.Registrar) error {
	return registrar.Register(New())
}

func definition() domain.NodeDefinition {
	textType := typespec.String()
	resultType := domain.TypeSpec{Kind: domain.TypeRecord, Fields: []domain.TypeFieldSpec{
		{ID: "path", Name: "path", Type: typespec.String()},
		{ID: "bytes", Name: "bytes", Type: typespec.Int()},
		{ID: "status", Name: "status", Type: typespec.Int()},
	}}
	return domain.NodeDefinition{
		Type:        "action:download",
		Category:    "Local",
		Label:       "Download from Web",
		Description: "Download a file from a URL and save it to a local directory.",
		Icon:        "download",
		Color:       "#c4b5fd",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: "url", Label: "URL", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
			{ID: "location", Label: "Location", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: true, MaxConnections: 1},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{
				ID: "result", Label: "Result", Kind: domain.PinData, Direction: domain.PinOutput,
				DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", MaxConnections: 1,
				Fields: []domain.DataField{
					{Path: "path", Label: "Path", DataType: domain.DataText, Description: "Absolute path of the saved file."},
					{Path: "bytes", Label: "Bytes", DataType: domain.DataNumber, Description: "Number of bytes written."},
					{Path: "status", Label: "Status", DataType: domain.DataNumber, Description: "HTTP status code returned by the server."},
				},
			},
		},
		Fields: []domain.ConfigField{
			{Name: "url", Label: "URL", Kind: "string", Placeholder: "https://example.com/file.zip", Required: true},
			{Name: "location", Label: "Location", Kind: "string", Placeholder: "C:\\Downloads", Required: true},
		},
		Capabilities:      []domain.Capability{domain.CapabilityNetwork, domain.CapabilityFileWrite},
		DefaultConfig:     map[string]any{"url": "", "location": ""},
		Source:            "builtin",
		PortContractOwned: true,
	}
}

func execute(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("download cancelled: %w", err)
	}
	rawURL := strings.TrimSpace(textValue(invocation.Inputs["url"], invocation.Config["url"]))
	if rawURL == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("download URL is required")
	}
	location := strings.TrimSpace(textValue(invocation.Inputs["location"], invocation.Config["location"]))
	if location == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("download location is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("download URL is invalid")
	}
	fileName := fileNameFromURL(parsed)
	if fileName == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("download URL does not expose a file name; supply one in the URL path")
	}
	cleanDir := filepath.Clean(location)
	if err := os.MkdirAll(cleanDir, 0o700); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create download directory: %w", err)
	}
	destination := filepath.Join(cleanDir, fileName)
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("download cancelled: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("build download request: %w", err)
	}
	request.Header.Set("User-Agent", "Neuropipe/0.1")
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("download request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= http.StatusBadRequest {
		return nodes.ExecutionResult{}, fmt.Errorf("download request returned %s", response.Status)
	}
	temporary := destination + ".part"
	file, err := os.Create(temporary)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("create download file: %w", err)
	}
	written, err := io.Copy(file, io.LimitReader(response.Body, 2*1024*1024*1024)) // 2 GiB safety bound
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return nodes.ExecutionResult{}, fmt.Errorf("write download file: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return nodes.ExecutionResult{}, fmt.Errorf("finalize download file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("download cancelled: %w", err)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"result": map[string]any{
				"path":   destination,
				"bytes":  int64(written),
				"status": response.StatusCode,
			},
		},
		Ports: []string{"out"},
	}, nil
}

// textValue resolves a string input by prioritising connected data pins over
// the inspector field. Empty strings from pins fall back to the inspector.
func textValue(value any, fallback any) string {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		return text
	}
	if text, ok := fallback.(string); ok {
		return text
	}
	return ""
}

// fileNameFromURL extracts the last path segment, falling back to a stable
// name when the URL has none. Query parameters are ignored.
func fileNameFromURL(parsed *url.URL) string {
	path := strings.TrimSpace(parsed.Path)
	if path == "" || path == "/" {
		return ""
	}
	segment := path
	if index := strings.LastIndex(segment, "/"); index >= 0 {
		segment = segment[index+1:]
	}
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	return segment
}
