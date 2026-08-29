package drawimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/* ------------------------------------------------------------------ */
/* editor support (backend preview + image source loading)             */
/* ------------------------------------------------------------------ */

// RenderPreviewJSON renders a document (as raw JSON) with sample values
// (raw JSON object keyed by pin name) and returns base64 PNG bytes. It backs
// the editor's "backend preview" mode so users can verify the true output.
func RenderPreviewJSON(ctx context.Context, documentJSON, valuesJSON string) (string, error) {
	var documentValue any
	if err := json.Unmarshal([]byte(documentJSON), &documentValue); err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	values := map[string]any{}
	if strings.TrimSpace(valuesJSON) != "" {
		if err := json.Unmarshal([]byte(valuesJSON), &values); err != nil {
			return "", fmt.Errorf("parse sample values: %w", err)
		}
	}
	document := ParseDocument(documentValue)
	encoded, _, err := Render(ctx, document, values, RenderOptions{Format: "png"})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}

// LoadImageSourceDataURL resolves an editor image source (kind url|path) and
// returns a data URL. The backend performs URL fetches so the editor preview
// is not restricted by browser CORS.
func LoadImageSourceDataURL(ctx context.Context, kind, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("image source is empty")
	}
	var data []byte
	if kind == "url" || (kind == "pin" && isURLLike(trimmed)) {
		var err error
		data, err = fetchImage(ctx, trimmed)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		data, err = os.ReadFile(filepath.Clean(trimmed))
		if err != nil {
			return "", fmt.Errorf("read image: %w", err)
		}
	}
	return "data:" + DetectImageMIME(data) + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func fetchImage(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("image request: %w", err)
	}
	req.Header.Set("User-Agent", "Neuropipe/0.1")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}
	const limit = 64 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(data) > limit {
		return nil, fmt.Errorf("image exceeds %d MiB", limit>>20)
	}
	return data, nil
}

func isURLLike(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// DetectImageMIME sniffs common image magic bytes.
func DetectImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
