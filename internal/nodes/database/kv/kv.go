// Package kv provides shared pin, field, and conversion helpers for
// first-party key/value node modules. It contains no execution logic of its
// own so every node package keeps owning its semantics.
package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

var (
	textType   = domain.TypeSpec{Kind: domain.TypeString}
	intType    = domain.TypeSpec{Kind: domain.TypeInt}
	floatType  = domain.TypeSpec{Kind: domain.TypeFloat}
	boolType   = domain.TypeSpec{Kind: domain.TypeBool}
	anyType    = domain.TypeSpec{Kind: domain.TypeAny}
	stringType = domain.TypeSpec{Kind: domain.TypeString}
	listType   = domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeAny}}
)

// Exec builds the control-flow pin used by every impure KV node.
func Exec(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

// Text builds a string data pin.
func Text(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &textType, Color: "#e879f9", Required: required, MaxConnections: 1}
}

// Number builds a numeric data pin.
func Number(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataNumber, Type: &floatType, Color: "#86efac", Required: required, MaxConnections: 1}
}

// Bool builds a boolean data pin.
func Bool(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", Required: required, MaxConnections: 1}
}

// List builds a list-of-any data pin.
func List(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataList, Type: &listType, Color: "#facc15", Required: required, MaxConnections: 1}
}

// Object builds a map<string, any> data pin. Hash nodes use it so an
// inspector-configured field object survives config-fallback validation
// (a map can never satisfy the list pin contract).
func Object(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	keyType := domain.TypeSpec{Kind: domain.TypeString}
	anyValueType := domain.TypeSpec{Kind: domain.TypeAny}
	objectType := domain.TypeSpec{Kind: domain.TypeMap, Key: &keyType, Value: &anyValueType}
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataObject, Type: &objectType, Color: "#60a5fa", Required: required, MaxConnections: 1}
}

// Any builds an untyped data pin (the generic command node's value output).
func Any(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataAny, Type: &anyType, Color: "#a1a1aa", MaxConnections: 1}
}

// DatabaseField is the connection picker shared by every KV node.
func DatabaseField() domain.ConfigField {
	return domain.ConfigField{Name: "databaseId", Label: "KV database", Kind: "kv-database-select", Required: true}
}

// Definition assembles the common NodeDefinition skeleton for KV nodes.
func Definition(nodeType, label, description string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	allFields := append([]domain.ConfigField{DatabaseField()}, fields...)
	if defaults == nil {
		defaults = map[string]any{}
	}
	defaults["databaseId"] = ""
	return domain.NodeDefinition{
		Type: nodeType, Category: "KV Store", Label: label, Description: description,
		Icon: "database", Color: "#dc382d", Mode: domain.NodeImpure, PortContractOwned: true,
		Capabilities: []domain.Capability{domain.CapabilityNetwork},
		Inputs:       inputs, Outputs: outputs, Fields: allFields,
		DefaultConfig: defaults, Source: "builtin",
	}
}

// DatabaseID reads and validates the selected KV connection.
func DatabaseID(invocation nodes.Invocation) (string, error) {
	id, _ := invocation.Config["databaseId"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("select a KV database first")
	}
	return id, nil
}

// ExecuteCommand resolves the KV executor from the runtime and runs one
// command. Nodes never import go-redis; they only pass string arguments.
func ExecuteCommand(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime, request domain.KVCommandRequest) (domain.KVCommandResult, error) {
	provider, ok := runtime.(nodes.KVExecutorProvider)
	if !ok || provider.KVExecutor() == nil {
		return domain.KVCommandResult{}, fmt.Errorf("key/value database execution is unavailable")
	}
	databaseID, err := DatabaseID(invocation)
	if err != nil {
		return domain.KVCommandResult{}, err
	}
	request.DatabaseID = databaseID
	return provider.KVExecutor().ExecuteCommand(ctx, request)
}

// String reads one string pin value (wired or config fallback).
func String(invocation nodes.Invocation, pinID string) string {
	value, _ := invocation.Inputs[pinID].(string)
	return value
}

// Strings reads one list pin as string arguments. Scalar values are
// converted with Arg so a single wired string still works.
func Strings(invocation nodes.Invocation, pinID string) ([]string, error) {
	raw, exists := invocation.Inputs[pinID]
	if !exists || raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case []any:
		result := make([]string, len(typed))
		for index, item := range typed {
			converted, err := Arg(item)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", index+1, err)
			}
			result[index] = converted
		}
		return result, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		return []string{typed}, nil
	default:
		converted, err := Arg(raw)
		if err != nil {
			return nil, err
		}
		return []string{converted}, nil
	}
}

// StringMap reads one object pin as field/value pairs for hash operations.
func StringMap(invocation nodes.Invocation, pinID string) (map[string]string, error) {
	raw, exists := invocation.Inputs[pinID]
	if !exists || raw == nil {
		return map[string]string{}, nil
	}
	typed, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("pin %q requires an object value", pinID)
	}
	result := make(map[string]string, len(typed))
	for key, value := range typed {
		converted, err := Arg(value)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		result[key] = converted
	}
	return result, nil
}

// Int reads one numeric pin with a fallback default when unconnected.
func Int(invocation nodes.Invocation, pinID string, fallback int64) (int64, error) {
	raw, exists := invocation.Inputs[pinID]
	if !exists || raw == nil {
		return fallback, nil
	}
	switch typed := raw.(type) {
	case float64:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case json.Number:
		return strconv.ParseInt(typed.String(), 10, 64)
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback, nil
		}
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, fmt.Errorf("pin %q requires a number", pinID)
		}
		return int64(parsed), nil
	default:
		return 0, fmt.Errorf("pin %q requires a number", pinID)
	}
}

// Flag reads one boolean pin with a fallback default when unconnected.
func Flag(invocation nodes.Invocation, pinID string, fallback bool) bool {
	raw, exists := invocation.Inputs[pinID]
	if !exists || raw == nil {
		return fallback
	}
	if value, ok := raw.(bool); ok {
		return value
	}
	return fallback
}

// ConfigFlag reads one boolean that exists only as an inspector field with no
// matching data pin, so the engine never copies it into Inputs.
func ConfigFlag(invocation nodes.Invocation, key string) bool {
	value, _ := invocation.Config[key].(bool)
	return value
}

// ConfigStrings reads one inspector list field as the array of values the
// visual list editor persists. Blank items are dropped.
func ConfigStrings(invocation nodes.Invocation, key string) []string {
	typed, ok := invocation.Config[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(typed))
	for _, item := range typed {
		converted, err := Arg(item)
		if err != nil {
			continue
		}
		if strings.TrimSpace(converted) != "" {
			result = append(result, converted)
		}
	}
	return result
}

// Arg converts one typed pin value into its Redis string argument form.
// Lists and objects are JSON-encoded so complex values stay lossless.
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
	case []any, map[string]any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", fmt.Errorf("encode argument: %w", err)
		}
		return string(encoded), nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", fmt.Errorf("encode argument: %w", err)
		}
		return string(encoded), nil
	}
}

// ScoredEntry is one member/score pair read from a sorted-set list pin.
type ScoredEntry struct {
	Member string
	Score  float64
}

// ScoredEntries reads one list pin of {member, score} objects.
func ScoredEntries(invocation nodes.Invocation, pinID string) ([]ScoredEntry, error) {
	raw, exists := invocation.Inputs[pinID]
	if !exists || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("pin %q requires a list of member/score entries", pinID)
	}
	result := make([]ScoredEntry, 0, len(items))
	for index, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entry %d requires an object with member and score", index+1)
		}
		member, err := Arg(entry["member"])
		if err != nil {
			return nil, fmt.Errorf("entry %d member: %w", index+1, err)
		}
		score := 0.0
		switch value := entry["score"].(type) {
		case float64:
			score = value
		case int64:
			score = float64(value)
		case int:
			score = float64(value)
		case string:
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("entry %d score is required", index+1)
			}
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("entry %d score: %w", index+1, err)
			}
			score = parsed
		case nil:
			return nil, fmt.Errorf("entry %d score is required", index+1)
		default:
			return nil, fmt.Errorf("entry %d score must be a number", index+1)
		}
		result = append(result, ScoredEntry{Member: member, Score: score})
	}
	return result, nil
}
