package drawimage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Register the decoders used for image sources.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/gogpu/gg"
	"golang.org/x/image/webp"
)

// maxImageDownloadBytes caps remote image bodies (64 MiB).
const maxImageDownloadBytes = 64 << 20

// imageLoader loads and caches images for one render pass. Sources repeat
// across repeated elements, so a per-render cache keeps pipelines cheap.
type imageLoader struct {
	client  *http.Client
	context context.Context
	cache   map[string]*gg.ImageBuf
}

func newImageLoader(ctx context.Context) *imageLoader {
	return &imageLoader{
		client:  &http.Client{Timeout: 30 * time.Second},
		context: ctx,
		cache:   map[string]*gg.ImageBuf{},
	}
}

// Load resolves an image element source against the pin values and returns
// the decoded buffer. kind is the source kind (url/path/pin).
func (l *imageLoader) Load(source ImageSource, values map[string]any) (*gg.ImageBuf, error) {
	resolved := source.Value
	if source.Kind == "pin" {
		value, ok := values[source.Value]
		if !ok {
			return nil, fmt.Errorf("pin %q is not connected", source.Value)
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("pin %q did not provide a path or URL", source.Value)
		}
		resolved = strings.TrimSpace(text)
	}
	if resolved == "" {
		return nil, fmt.Errorf("image source is empty")
	}
	cacheKey := source.Kind + "\x00" + resolved
	if cached, ok := l.cache[cacheKey]; ok {
		return cached, nil
	}
	var (
		buf  *gg.ImageBuf
		err  error
		kind = source.Kind
	)
	if source.Kind == "pin" {
		// pin values may carry either shape
		if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
			kind = "url"
		} else {
			kind = "path"
		}
	}
	switch kind {
	case "url":
		buf, err = l.loadURL(resolved)
	default:
		buf, err = loadPath(resolved)
	}
	if err != nil {
		return nil, err
	}
	l.cache[cacheKey] = buf
	return buf, nil
}

func (l *imageLoader) loadURL(rawURL string) (*gg.ImageBuf, error) {
	req, err := http.NewRequestWithContext(l.context, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("image request: %w", err)
	}
	req.Header.Set("User-Agent", "Neuropipe/0.1")
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(body) > maxImageDownloadBytes {
		return nil, fmt.Errorf("image exceeds %d MiB", maxImageDownloadBytes>>20)
	}
	return decodeImage(body)
}

func loadPath(path string) (*gg.ImageBuf, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return nil, fmt.Errorf("image path is empty")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return decodeImage(data)
}

func decodeImage(data []byte) (*gg.ImageBuf, error) {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// webp is not auto-registered by image.Decode
		decoded, err = webp.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
	}
	return gg.ImageBufFromImage(decoded), nil
}
