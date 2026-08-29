// Package attachments resolves message file attachments from the sources a
// pipeline can produce: download URLs, local paths, and raw byte or base64
// pin values. Discord and Telegram nodes share it so both platforms enforce
// the same pre-flight limits with the same precise rejection reasons.
package attachments

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Attachment is one fully loaded file ready for upload. ContentType is derived
// from the file extension when the caller does not know better.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// Limits bounds one Load call. MaxBytes applies per file, MaxCount to the
// combined result; zero values disable the respective check.
type Limits struct {
	MaxBytes int64
	MaxCount int
}

// Sources describes every attachment input of a node. URLs and Paths carry
// one entry per line so a single text pin can attach several files; Data is
// the raw bytes (or base64 text) pin value named by DataName.
type Sources struct {
	URLs     string
	Paths    string
	Data     any
	DataName string
}

// downloadTimeout caps one URL fetch. Files large enough to need longer are
// past both platforms' upload caps anyway.
const downloadTimeout = 60 * time.Second

// Load resolves every non-empty source into attachments, enforcing limits
// before the first byte reaches the platform. Empty sources return nil.
func Load(ctx context.Context, sources Sources, limits Limits) ([]Attachment, error) {
	var attachments []Attachment
	for _, rawURL := range splitLines(sources.URLs) {
		attachment, err := loadURL(ctx, rawURL, limits.MaxBytes)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	for _, rawPath := range splitLines(sources.Paths) {
		attachment, err := loadPath(rawPath, limits.MaxBytes)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if data := sources.Data; data != nil {
		attachment, err := loadData(data, sources.DataName, limits.MaxBytes)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if limits.MaxCount > 0 && len(attachments) > limits.MaxCount {
		return nil, fmt.Errorf("too many attachments: %d (limit %d)", len(attachments), limits.MaxCount)
	}
	return attachments, nil
}

// splitLines turns a newline-separated list into trimmed non-empty entries.
func splitLines(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	entries := make([]string, 0, 4)
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			entries = append(entries, line)
		}
	}
	return entries
}

func loadURL(ctx context.Context, rawURL string, maxBytes int64) (Attachment, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Attachment{}, fmt.Errorf("attachment URL %q is not a valid absolute URL", rawURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Attachment{}, fmt.Errorf("attachment URL %q: %w", rawURL, err)
	}
	client := &http.Client{Timeout: downloadTimeout}
	response, err := client.Do(request)
	if err != nil {
		return Attachment{}, fmt.Errorf("download attachment %q: %w", rawURL, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Attachment{}, fmt.Errorf("download attachment %q failed: HTTP %d", rawURL, response.StatusCode)
	}
	name := NameFromURL(rawURL)
	data, err := readBody(response.Body, maxBytes, name)
	if err != nil {
		return Attachment{}, err
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = ContentTypeForName(name)
	} else if separator := strings.IndexByte(contentType, ';'); separator >= 0 {
		contentType = strings.TrimSpace(contentType[:separator])
	}
	return Attachment{Name: name, ContentType: contentType, Data: data}, nil
}

func loadPath(rawPath string, maxBytes int64) (Attachment, error) {
	cleanPath := filepath.Clean(rawPath)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return Attachment{}, fmt.Errorf("attachment path %q: %w", rawPath, err)
	}
	if info.IsDir() {
		return Attachment{}, fmt.Errorf("attachment path %q is a directory", rawPath)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return Attachment{}, sizeError(filepath.Base(cleanPath), info.Size(), maxBytes)
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return Attachment{}, fmt.Errorf("read attachment %q: %w", rawPath, err)
	}
	name := filepath.Base(cleanPath)
	return Attachment{Name: name, ContentType: ContentTypeForName(name), Data: data}, nil
}

// loadData accepts a []byte pin value directly (for example the Draw Image
// node's image output) or a base64 string with an optional data URL prefix.
func loadData(value any, name string, maxBytes int64) (Attachment, error) {
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return Attachment{}, fmt.Errorf("attachment data is empty")
		}
		if before, after, found := strings.Cut(trimmed, ","); found && strings.HasPrefix(before, "data:") {
			trimmed = strings.TrimSpace(after)
			if trimmed == "" {
				return Attachment{}, fmt.Errorf("attachment data URL has no payload")
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			return Attachment{}, fmt.Errorf("attachment data is not valid base64: %w", err)
		}
		data = decoded
	default:
		return Attachment{}, fmt.Errorf("attachment data must be bytes or base64 text")
	}
	if len(data) == 0 {
		return Attachment{}, fmt.Errorf("attachment data is empty")
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return Attachment{}, sizeError(name, int64(len(data)), maxBytes)
	}
	if strings.TrimSpace(name) == "" {
		name = "file.bin"
	}
	return Attachment{Name: name, ContentType: ContentTypeForName(name), Data: data}, nil
}

func readBody(body io.Reader, maxBytes int64, name string) ([]byte, error) {
	if maxBytes > 0 {
		body = io.LimitReader(body, maxBytes+1)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("download attachment %q: %w", name, err)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, sizeError(name, int64(len(data)), maxBytes)
	}
	return data, nil
}

func sizeError(name string, size, maxBytes int64) error {
	return fmt.Errorf("attachment %q is %s, over the %s limit", name, formatBytes(size), formatBytes(maxBytes))
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/unit)
}

// NameFromURL derives a file name from a URL path, falling back to "download"
// when the path carries nothing usable.
func NameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}
	base := filepath.Base(strings.TrimSuffix(parsed.Path, "/"))
	if base == "" || base == "/" || base == "." {
		return "download"
	}
	return base
}

// ContentTypeForName guesses the MIME type from the file extension with an
// octet-stream fallback that both platforms accept.
func ContentTypeForName(name string) string {
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); contentType != "" {
		if separator := strings.IndexByte(contentType, ';'); separator >= 0 {
			contentType = strings.TrimSpace(contentType[:separator])
		}
		return contentType
	}
	return "application/octet-stream"
}
