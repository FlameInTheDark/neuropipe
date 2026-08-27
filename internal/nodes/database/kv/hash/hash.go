// Package hash registers the Redis hash KV nodes.
package hash

import (
	"context"
	"fmt"
	"sort"

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

/* ---------------- KV Hash Get ---------------- */

func getDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_hash_get", "KV Hash Get", "Read one field or the whole hash stored at a key.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Text("field", "Field", domain.PinInput, false),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Text("value", "Value", domain.PinOutput, false),
			kvnodes.Bool("found", "Found", domain.PinOutput, false),
			kvnodes.List("fields", "Fields", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "user:42"},
			{Name: "field", Label: "Field (empty reads the whole hash)", Kind: "string"},
		},
		map[string]any{"key": "", "field": ""},
	)
}

func executeGet(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	field := kvnodes.String(invocation, "field")
	command := "HGETALL"
	args := []string{key}
	if field != "" {
		command = "HGET"
		args = append(args, field)
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"value": "", "found": false, "fields": map[string]any{}}
	if command == "HGET" {
		if !result.IsNil {
			if text, ok := result.Value.(string); ok {
				outputs["value"] = text
			}
			outputs["found"] = true
		}
	} else {
		fields, _ := result.Value.(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		outputs["fields"] = fields
		outputs["found"] = len(fields) > 0
		if len(fields) == 1 {
			// A single-field hash still exposes its lone value for convenience.
			for _, value := range fields {
				if text, ok := value.(string); ok {
					outputs["value"] = text
				}
			}
		}
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

/* ---------------- KV Hash Set ---------------- */

func setDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_hash_set", "KV Hash Set", "Write or remove fields on a hash.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Object("fields", "Fields", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("added", "New fields", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "fields", Label: "Fields", Kind: "kv-hash-fields", Placeholder: `email = ada@example.com`},
			{Name: "mode", Label: "Mode", Kind: "select", Options: []domain.Option{
				{Value: "set", Label: "Set fields (HSET)"},
				{Value: "remove", Label: "Remove fields (HDEL)"},
			}},
		},
		map[string]any{"key": "", "fields": map[string]any{}, "mode": "set"},
	)
}

func executeSet(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	fields, err := kvnodes.StringMap(invocation, "fields")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(fields) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one field is required")
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	command := "HSET"
	args := make([]string, 0, len(fields)*2+1)
	args = append(args, key)
	if mode, _ := invocation.Config["mode"].(string); mode == "remove" {
		command = "HDEL"
		args = append(args, names...)
	} else {
		for _, name := range names {
			args = append(args, name, fields[name])
		}
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	added := int64(0)
	switch value := result.Value.(type) {
	case int64:
		added = value
	case float64:
		added = int64(value)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"added": added},
		Ports:   []string{"out"},
	}, nil
}
