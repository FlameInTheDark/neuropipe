// URL nodes: generate temporary presigned URLs for S3 objects and build
// public URLs for files and folders in any registered storage. Both are
// metadata-only operations — presigning is computed offline against the
// stored credentials and public URLs are assembled from connection settings —
// so neither node performs network I/O of its own.
package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

/* ---------------- helpers ---------------- */

// ParseExpirySeconds accepts a duration as plain seconds ("3600"), a Go
// duration string ("90s", "15m", "1h30m"), or day units ("7d"). Blank falls
// back to one hour. Bounds are enforced by the storage service (1 second to
// 7 days, the SigV4 window).
func ParseExpirySeconds(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 3600, nil
	}
	if seconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return seconds, nil
	}
	if days, ok := strings.CutSuffix(strings.ToLower(trimmed), "d"); ok {
		if value, err := strconv.ParseFloat(days, 64); err == nil {
			return int64(value * 86400), nil
		}
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("expiry %q is not a duration (use seconds, 15m, 1h30m, or 7d)", raw)
	}
	return int64(duration / time.Second), nil
}

// ParseKeyValueLines parses key=value lines used for signed headers and query
// parameters. Blank lines and #-comments are skipped; a line without "=" is
// an error so typos surface immediately instead of signing the wrong thing.
// Values may contain "=" themselves (only the first separator splits).
func ParseKeyValueLines(raw, what string) (map[string]string, error) {
	result := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("%s: line %q must be key=value", what, line)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("%s: key must not be blank", what)
		}
		result[name] = strings.TrimSpace(value)
	}
	return result, nil
}

// urlDefinition assembles the definition skeleton for the URL nodes. Unlike
// the transfer nodes they never touch the network — presigning is computed
// locally and public URLs are pure metadata — so no capability is declared.
func urlDefinition(nodeType, label, description string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	allFields := append([]domain.ConfigField{StorageField()}, fields...)
	if defaults == nil {
		defaults = map[string]any{}
	}
	defaults["storageId"] = ""
	return domain.NodeDefinition{
		Type: nodeType, Category: "Storage", Label: label, Description: description,
		Icon: "cloud", Color: "#f59e0b", Mode: domain.NodeImpure, PortContractOwned: true,
		Inputs: inputs, Outputs: outputs, Fields: allFields,
		DefaultConfig: defaults, Source: "builtin",
	}
}

/* ---------------- Presign URL ---------------- */

func presignDefinition() domain.NodeDefinition {
	return urlDefinition("action:storage_presign_url", "Presign URL", "Generate a temporary signed URL (GET, PUT, HEAD, or DELETE) for one object in an S3 storage.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("path", "Path", domain.PinInput, true),
			Text("expires", "Expires", domain.PinInput, false),
			Text("headers", "Headers", domain.PinInput, false),
			Text("params", "Params", domain.PinInput, false),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Text("url", "URL", domain.PinOutput, false),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "url", Label: "URL", DataType: domain.DataText, Description: "The presigned URL."},
				{Path: "method", Label: "Method", DataType: domain.DataText, Description: "HTTP method the URL is signed for (GET, PUT, HEAD, or DELETE)."},
				{Path: "expiresInSeconds", Label: "Expires in", DataType: domain.DataNumber, Description: "Validity window in seconds."},
				{Path: "expiresAt", Label: "Expires at", DataType: domain.DataText, Description: "Moment the URL stops working (UTC, RFC 3339)."},
				{Path: "headers", Label: "Headers", DataType: domain.DataObject, Description: "Required headers — the consumer must send exactly these for the signature to validate."},
			}),
		},
		[]domain.ConfigField{
			{Name: "method", Label: "Method", Kind: "select", Options: []domain.Option{
				{Value: "GET", Label: "GET — download"},
				{Value: "PUT", Label: "PUT — upload"},
				{Value: "HEAD", Label: "HEAD — metadata"},
				{Value: "DELETE", Label: "DELETE — remove"},
			}},
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "reports/2026/chart.png", Required: true},
			{Name: "expires", Label: "Expires", Kind: "string", Placeholder: "1h (seconds, 15m, 1h30m, or 7d)"},
			{Name: "headers", Label: "Required headers", Kind: "textarea", Placeholder: "Content-Type=image/png\nx-amz-meta-owner=neuropipe"},
			{Name: "params", Label: "Query parameters", Kind: "textarea", Placeholder: "response-content-disposition=attachment; filename=\"chart.png\""},
		},
		map[string]any{"method": "GET", "path": "", "expires": "", "headers": "", "params": ""},
	)
}

func executePresign(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	path, err := RequiredText(invocation, "path", "path")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	method, _ := invocation.Config["method"].(string)
	if strings.TrimSpace(method) == "" {
		method = "GET"
	}
	expires, err := ParseExpirySeconds(String(invocation, "expires"))
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	headers, err := ParseKeyValueLines(String(invocation, "headers"), "headers")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	params, err := ParseKeyValueLines(String(invocation, "params"), "params")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StoragePresignURL(ctx, domain.StoragePresignRequest{
		StorageID: id, Path: path, Method: method,
		ExpiresSeconds: expires, Headers: headers, Params: params,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	requiredHeaders := map[string]any{}
	for name, value := range result.Headers {
		requiredHeaders[name] = value
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"url": result.URL,
			"result": map[string]any{
				"url":              result.URL,
				"method":           result.Method,
				"expiresInSeconds": result.ExpiresInSeconds,
				"expiresAt":        result.ExpiresAt,
				"headers":          requiredHeaders,
			},
		},
		Ports: []string{"out"},
	}, nil
}

/* ---------------- Public URL ---------------- */

func publicURLDefinition() domain.NodeDefinition {
	return urlDefinition("action:storage_public_url", "Public URL", "Build the public URL of one file or folder — the storage's public base URL when set, the direct S3 or FTP address otherwise.",
		[]domain.NodePort{
			Exec("in", "Exec", domain.PinInput),
			Text("path", "Path", domain.PinInput, false),
		},
		[]domain.NodePort{
			Exec("out", "Then", domain.PinOutput),
			Text("url", "URL", domain.PinOutput, false),
			Result("result", "Result", domain.PinOutput, []domain.DataField{
				{Path: "url", Label: "URL", DataType: domain.DataText, Description: "The public URL."},
				{Path: "kind", Label: "Kind", DataType: domain.DataText, Description: "Where the URL points: public-base (connection's public base URL), s3 (direct object address), or ftp (protocol URL)."},
			}),
		},
		[]domain.ConfigField{
			{Name: "path", Label: "Path", Kind: "string", Placeholder: "reports/2026/chart.png (empty = storage root)"},
		},
		map[string]any{"path": ""},
	)
}

func executePublicURL(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ContextCancelled(ctx); err != nil {
		return nodes.ExecutionResult{}, err
	}
	executor, id, err := Executor(invocation, runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	result, err := executor.StoragePublicURL(ctx, domain.StoragePublicURLRequest{
		StorageID: id, Path: String(invocation, "path"),
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"url":    result.URL,
			"result": map[string]any{"url": result.URL, "kind": result.Kind},
		},
		Ports: []string{"out"},
	}, nil
}
