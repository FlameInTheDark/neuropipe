// Package keys registers the core string and key-management KV nodes.
package keys

import (
	"context"
	"fmt"
	"strconv"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func RegisterGet(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: getDefinition(), Executor: executeGet})
}

func RegisterSet(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: setDefinition(), Executor: executeSet})
}

func RegisterDelete(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: deleteDefinition(), Executor: executeDelete})
}

func RegisterExists(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: existsDefinition(), Executor: executeExists})
}

func RegisterIncrement(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: incrementDefinition(), Executor: executeIncrement})
}

func RegisterRename(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: renameDefinition(), Executor: executeRename})
}

func RegisterExpire(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: expireDefinition(), Executor: executeExpire})
}

func RegisterTTL(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: ttlDefinition(), Executor: executeTTL})
}

/* ---------------- KV Get ---------------- */

func getDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_get", "KV Get", "Read one string value from a registered KV database.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Text("value", "Value", domain.PinOutput, false),
			kvnodes.Bool("found", "Found", domain.PinOutput, false),
		},
		[]domain.ConfigField{{Name: "key", Label: "Key", Kind: "string", Placeholder: "user:42:name"}},
		map[string]any{"key": ""},
	)
}

func executeGet(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "GET", Args: []string{key}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	value := ""
	if !result.IsNil {
		if text, ok := result.Value.(string); ok {
			value = text
		} else if result.Value != nil {
			value = fmt.Sprintf("%v", result.Value)
		}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"value": value, "found": !result.IsNil},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Set ---------------- */

func setDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_set", "KV Set", "Write one string value with an optional expiry in seconds.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Text("value", "Value", domain.PinInput, true),
			kvnodes.Number("ttlSeconds", "TTL seconds", domain.PinInput, false),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Bool("ok", "Set", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "user:42:name"},
			{Name: "value", Label: "Value", Kind: "textarea", Placeholder: "Ada Lovelace"},
			{Name: "ttlSeconds", Label: "TTL seconds (0 = keep forever)", Kind: "number", Placeholder: "0"},
			{Name: "condition", Label: "Write condition", Kind: "select", Options: []domain.Option{
				{Value: "always", Label: "Always"},
				{Value: "if-not-exists", Label: "Only if the key does not exist (NX)"},
				{Value: "if-exists", Label: "Only if the key exists (XX)"},
			}},
			{Name: "returnPrevious", Label: "Return previous value", Kind: "boolean"},
		},
		map[string]any{"key": "", "value": "", "condition": "always", "returnPrevious": false},
	)
}

func executeSet(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	value := kvnodes.String(invocation, "value")
	args := []string{key, value}
	ttl, err := kvnodes.Int(invocation, "ttlSeconds", 0)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if ttl > 0 {
		args = append(args, "EX", strconv.FormatInt(ttl, 10))
	}
	condition, _ := invocation.Config["condition"].(string)
	previousRequested := kvnodes.ConfigFlag(invocation, "returnPrevious")
	switch condition {
	case "if-not-exists":
		args = append(args, "NX")
	case "if-exists":
		args = append(args, "XX")
	}
	if previousRequested {
		args = append(args, "GET")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "SET", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	// Without GET, a nil reply means the NX/XX condition rejected the write.
	// With GET, nil only reports that no previous value existed, so the write
	// outcome is interpreted through the previous output instead.
	ok := true
	if result.IsNil && !previousRequested {
		ok = false
	}
	outputs := map[string]any{"ok": ok}
	if previousRequested {
		previous := ""
		if !result.IsNil {
			if text, ok := result.Value.(string); ok {
				previous = text
			}
		}
		outputs["previous"] = previous
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

/* ---------------- KV Delete ---------------- */

func deleteDefinition() domain.NodeDefinition {
	keysPin := kvnodes.List("keys", "Keys", domain.PinInput, true)
	// The multiline textarea config is parsed by the node itself; letting the
	// engine copy it into the list pin would fail list type validation.
	keysPin.IgnoreConfigFallback = true
	return kvnodes.Definition("action:kv_delete", "KV Delete", "Delete one or more keys.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			keysPin,
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("deleted", "Deleted", domain.PinOutput, false),
		},
		[]domain.ConfigField{{Name: "keys", Label: "Keys", Kind: "kv-string-list", Placeholder: "user:42"}},
		map[string]any{"keys": []any{}},
	)
}

func executeDelete(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	keys, err := keyList(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(keys) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one key is required")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "DEL", Args: keys})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"deleted": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Exists ---------------- */

func existsDefinition() domain.NodeDefinition {
	keysPin := kvnodes.List("keys", "Keys", domain.PinInput, true)
	keysPin.IgnoreConfigFallback = true
	return kvnodes.Definition("action:kv_exists", "KV Exists", "Count how many of the given keys exist.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			keysPin,
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("count", "Existing keys", domain.PinOutput, false),
			kvnodes.Bool("exists", "Exists", domain.PinOutput, false),
		},
		[]domain.ConfigField{{Name: "keys", Label: "Keys", Kind: "kv-string-list", Placeholder: "user:42"}},
		map[string]any{"keys": []any{}},
	)
}

func executeExists(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	keys, err := keyList(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(keys) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one key is required")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "EXISTS", Args: keys})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	count := replyInt(result)
	return nodes.ExecutionResult{
		Outputs: map[string]any{"count": count, "exists": count > 0},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Increment ---------------- */

func incrementDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_increment", "KV Increment", "Increment a counter and return the new value.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Number("delta", "Delta", domain.PinInput, false),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("value", "New value", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "counter:page-views"},
			{Name: "delta", Label: "Delta", Kind: "number", Placeholder: "1"},
			{Name: "mode", Label: "Mode", Kind: "select", Options: []domain.Option{
				{Value: "integer", Label: "Integer (INCRBY)"},
				{Value: "float", Label: "Float (INCRBYFLOAT)"},
			}},
		},
		map[string]any{"key": "", "mode": "integer"},
	)
}

func executeIncrement(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	delta, err := kvnodes.Int(invocation, "delta", 1)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	command := "INCRBY"
	if mode, _ := invocation.Config["mode"].(string); mode == "float" {
		command = "INCRBYFLOAT"
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: []string{key, strconv.FormatInt(delta, 10)}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	value := any(replyFloat(result))
	if command == "INCRBY" {
		value = replyInt(result)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"value": value},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Rename ---------------- */

func renameDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_rename", "KV Rename", "Rename a key, optionally failing when the target exists.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Text("newKey", "New key", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Bool("ok", "Renamed", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "newKey", Label: "New key", Kind: "string"},
			{Name: "onlyIfMissing", Label: "Fail when the target exists", Kind: "boolean"},
		},
		map[string]any{"key": "", "newKey": "", "onlyIfMissing": false},
	)
}

func executeRename(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	newKey := kvnodes.String(invocation, "newKey")
	if key == "" || newKey == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key and new key are required")
	}
	command := "RENAME"
	if kvnodes.ConfigFlag(invocation, "onlyIfMissing") {
		command = "RENAMENX"
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: []string{key, newKey}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	ok := true
	if command == "RENAMENX" {
		ok = replyInt(result) == 1
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"ok": ok},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Expire ---------------- */

func expireDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_expire", "KV Expire", "Set or remove a key's expiry in seconds.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Number("ttlSeconds", "TTL seconds", domain.PinInput, false),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Bool("ok", "Applied", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "ttlSeconds", Label: "TTL seconds", Kind: "number", Placeholder: "60"},
			{Name: "mode", Label: "Mode", Kind: "select", Options: []domain.Option{
				{Value: "expire", Label: "Set expiry (seconds)"},
				{Value: "persist", Label: "Remove expiry (PERSIST)"},
			}},
		},
		map[string]any{"key": "", "mode": "expire"},
	)
}

func executeExpire(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	mode, _ := invocation.Config["mode"].(string)
	command := "EXPIRE"
	args := []string{key}
	if mode == "persist" {
		command = "PERSIST"
	} else {
		ttl, err := kvnodes.Int(invocation, "ttlSeconds", 0)
		if err != nil {
			return nodes.ExecutionResult{}, err
		}
		if ttl <= 0 {
			return nodes.ExecutionResult{}, fmt.Errorf("ttl seconds must be greater than zero")
		}
		args = append(args, strconv.FormatInt(ttl, 10))
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"ok": replyInt(result) == 1},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV TTL ---------------- */

func ttlDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_ttl", "KV TTL", "Read a key's remaining time to live in seconds.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("ttl", "TTL seconds", domain.PinOutput, false),
		},
		[]domain.ConfigField{{Name: "key", Label: "Key", Kind: "string"}},
		map[string]any{"key": ""},
	)
}

func executeTTL(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "TTL", Args: []string{key}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"ttl": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- shared helpers ---------------- */

// keyList reads the keys pin when wired, falling back to the inspector list
// field (the visual list editor's array of values) otherwise.
func keyList(invocation nodes.Invocation) ([]string, error) {
	if invocation.ConnectedInputs["keys"] {
		wired, err := kvnodes.Strings(invocation, "keys")
		if err != nil {
			return nil, err
		}
		if len(wired) > 0 {
			return wired, nil
		}
	}
	return kvnodes.ConfigStrings(invocation, "keys"), nil
}

// replyInt coerces a normalised reply into an int64.
func replyInt(result domain.KVCommandResult) int64 {
	switch value := result.Value.(type) {
	case int64:
		return value
	case float64:
		return int64(value)
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	default:
		return 0
	}
}

// replyFloat coerces a normalised reply into a float64.
func replyFloat(result domain.KVCommandResult) float64 {
	switch value := result.Value.(type) {
	case float64:
		return value
	case int64:
		return float64(value)
	case string:
		parsed, _ := strconv.ParseFloat(value, 64)
		return parsed
	default:
		return 0
	}
}
