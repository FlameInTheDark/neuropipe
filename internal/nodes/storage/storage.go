// Package storage provides shared pin, field, and resolution helpers for the
// S3/FTP storage node family. It contains no execution logic of its own so
// every node keeps owning its semantics. Nodes never import S3 or FTP
// clients; they pass domain requests through the StorageExecutor port.
package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

var (
	textType     = domain.TypeSpec{Kind: domain.TypeString}
	intType      = domain.TypeSpec{Kind: domain.TypeInt}
	boolType     = domain.TypeSpec{Kind: domain.TypeBool}
	bytesType    = domain.TypeSpec{Kind: domain.TypeBytes}
	stringType   = domain.TypeSpec{Kind: domain.TypeString}
	anyValueType = domain.TypeSpec{Kind: domain.TypeAny}
	entryType    = domain.TypeSpec{Kind: domain.TypeMap, Key: &stringType, Value: &anyValueType}
	entriesType  = domain.TypeSpec{Kind: domain.TypeList, Element: &entryType}
	resultType   = domain.TypeSpec{Kind: domain.TypeMap, Key: &stringType, Value: &anyValueType}
)

var stdBase64 = base64.StdEncoding

const timeFormat = time.RFC3339

// Exec builds the control-flow pin used by every storage node.
func Exec(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

// Text builds a string data pin.
func Text(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: required, MaxConnections: 1}
}

// Bytes builds a raw-bytes data pin. It matches the Draw Image node's image
// output type so rendered pictures and HTTP downloads wire straight in.
func Bytes(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", Required: required, MaxConnections: 1}
}

// Number builds a numeric data pin.
func Number(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &intType, Color: "#86efac", Required: required, MaxConnections: 1}
}

// Bool builds a boolean data pin.
func Bool(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", Required: required, MaxConnections: 1}
}

// Entries builds the listing output pin (list of file entries).
func Entries(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataList, Type: &entriesType, Color: "#facc15", MaxConnections: 1}
}

// Result builds the object result pin shared by the mutating nodes.
func Result(id, label string, direction domain.PinDirection, fields []domain.DataField) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataObject, Type: &resultType, Color: "#60a5fa", Fields: fields, MaxConnections: 1}
}

// StorageField is the connection picker shared by every storage node.
func StorageField() domain.ConfigField {
	return domain.ConfigField{Name: "storageId", Label: "Storage", Kind: "storage-select", Required: true}
}

// Definition assembles the common NodeDefinition skeleton for storage nodes.
func Definition(nodeType, label, description string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	allFields := append([]domain.ConfigField{StorageField()}, fields...)
	if defaults == nil {
		defaults = map[string]any{}
	}
	defaults["storageId"] = ""
	return domain.NodeDefinition{
		Type: nodeType, Category: "Storage", Label: label, Description: description,
		Icon: "cloud", Color: "#f59e0b", Mode: domain.NodeImpure, PortContractOwned: true,
		Capabilities: []domain.Capability{domain.CapabilityNetwork},
		Inputs:       inputs, Outputs: outputs, Fields: allFields,
		DefaultConfig: defaults, Source: "builtin",
	}
}

// StorageID reads and validates the selected storage connection.
func StorageID(invocation nodes.Invocation) (string, error) {
	id, _ := invocation.Config["storageId"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("select a storage first")
	}
	return id, nil
}

// Executor resolves the storage executor from the runtime. A missing
// executor (headless tests, remote executors without the port wired) becomes
// an explicit error naming the capability.
func Executor(invocation nodes.Invocation, runtime nodes.Runtime) (nodes.StorageExecutor, string, error) {
	provider, ok := runtime.(nodes.StorageExecutorProvider)
	if !ok || provider.StorageExecutor() == nil {
		return nil, "", fmt.Errorf("storage execution is unavailable")
	}
	id, err := StorageID(invocation)
	if err != nil {
		return nil, "", err
	}
	return provider.StorageExecutor(), id, nil
}

// String reads one string pin value (wired or config fallback).
func String(invocation nodes.Invocation, pinID string) string {
	value, _ := invocation.Inputs[pinID].(string)
	return value
}

// ConfigFlag reads one boolean that exists only as an inspector field.
func ConfigFlag(invocation nodes.Invocation, key string) bool {
	value, _ := invocation.Config[key].(bool)
	return value
}

// BytesValue reads one bytes pin value. Raw []byte flows through untouched;
// base64 strings (optionally data-URL prefixed) are decoded so text nodes
// can feed the pin as well.
func BytesValue(invocation nodes.Invocation, pinID string) ([]byte, error) {
	switch typed := invocation.Inputs[pinID].(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		decoded, err := Base64Text(typed)
		if err != nil {
			return nil, fmt.Errorf("pin %q requires bytes or base64 text", pinID)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("pin %q requires bytes", pinID)
	}
}

// Base64Text decodes base64 text (optionally data-URL prefixed) into bytes.
// Blank text reads as no data so an empty inspector field never errors.
func Base64Text(text string) ([]byte, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}
	if index := strings.Index(trimmed, "base64,"); index >= 0 && strings.HasPrefix(trimmed, "data:") {
		trimmed = trimmed[index+len("base64,"):]
	}
	decoded, err := stdBase64.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("value must be base64 text")
	}
	return decoded, nil
}

// configValue reads one key from a flow node's configuration. Blueprint v3
// stores config under Data["config"].
func configValue(node domain.FlowNode, key string) any {
	config := map[string]any{}
	config, _ = node.Data["config"].(map[string]any)
	return config[key]
}

// RequiredText reads one string pin and errors when it is blank.
func RequiredText(invocation nodes.Invocation, pinID, label string) (string, error) {
	value := strings.TrimSpace(String(invocation, pinID))
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

// EntryList converts storage entries into JSON-safe Blueprint packets.
func EntryList(entries []domain.StorageEntry) []any {
	result := make([]any, len(entries))
	for index, entry := range entries {
		modified := ""
		if !entry.ModTime.IsZero() {
			modified = entry.ModTime.UTC().Format(timeFormat)
		}
		result[index] = map[string]any{
			"name":     entry.Name,
			"path":     entry.Path,
			"isDir":    entry.IsDir,
			"size":     entry.Size,
			"modified": modified,
		}
	}
	return result
}

// ContextCancelled wraps the standard cancellation check node executors run
// at entry so long transfers abort promptly.
func ContextCancelled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("storage operation cancelled: %w", err)
	}
	return nil
}

// Arg converts a scalar into its string form (size counters etc.).
func Arg(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.FormatInt(int64(typed), 10), nil
	case json.Number:
		return typed.String(), nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", fmt.Errorf("encode argument: %w", err)
		}
		return string(encoded), nil
	}
}
