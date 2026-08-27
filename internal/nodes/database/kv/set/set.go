// Package set registers the Redis set KV nodes.
package set

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func RegisterAdd(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd})
}

func RegisterMembers(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: membersDefinition(), Executor: executeMembers})
}

func RegisterRemove(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: removeDefinition(), Executor: executeRemove})
}

/* ---------------- KV Set Add ---------------- */

func addDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_set_add", "KV Set Add", "Add members to a set.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.List("members", "Members", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("added", "New members", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "tags:article-7"},
			{Name: "members", Label: "Members", Kind: "kv-string-list"},
		},
		map[string]any{"key": "", "members": []any{}},
	)
}

func executeAdd(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key, members, err := keyAndMembers(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	args := append([]string{key}, members...)
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "SADD", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"added": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Set Members ---------------- */

func membersDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_set_members", "KV Set Members", "Read every member of a set.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.List("members", "Members", domain.PinOutput, false),
			kvnodes.Number("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{{Name: "key", Label: "Key", Kind: "string"}},
		map[string]any{"key": ""},
	)
}

func executeMembers(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "SMEMBERS", Args: []string{key}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	members, _ := result.Value.([]any)
	if members == nil {
		members = []any{}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"members": members, "count": int64(len(members))},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Set Remove ---------------- */

func removeDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_set_remove", "KV Set Remove", "Remove members from a set.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.List("members", "Members", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("removed", "Removed members", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "members", Label: "Members", Kind: "kv-string-list"},
		},
		map[string]any{"key": "", "members": []any{}},
	)
}

func executeRemove(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key, members, err := keyAndMembers(invocation)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	args := append([]string{key}, members...)
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "SREM", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"removed": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- shared helpers ---------------- */

func keyAndMembers(invocation nodes.Invocation) (string, []string, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return "", nil, fmt.Errorf("key is required")
	}
	members, err := kvnodes.Strings(invocation, "members")
	if err != nil {
		return "", nil, err
	}
	if len(members) == 0 {
		return "", nil, fmt.Errorf("at least one member is required")
	}
	return key, members, nil
}

func replyInt(result domain.KVCommandResult) int64 {
	switch value := result.Value.(type) {
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return 0
	}
}
