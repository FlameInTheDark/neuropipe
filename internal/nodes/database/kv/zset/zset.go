// Package zset registers the Redis sorted-set KV nodes.
package zset

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

func RegisterAdd(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: addDefinition(), Executor: executeAdd})
}

func RegisterRange(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange})
}

func RegisterRemove(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: removeDefinition(), Executor: executeRemove})
}

/* ---------------- KV Sorted Set Add ---------------- */

func addDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_zset_add", "KV Sorted Set Add", "Add or update scored members in a sorted set.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.List("entries", "Entries", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("added", "New members", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "leaderboard:weekly"},
			{Name: "entries", Label: "Entries", Kind: "kv-scored-entries"},
		},
		map[string]any{"key": "", "entries": []any{}},
	)
}

func executeAdd(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	entries, err := kvnodes.ScoredEntries(invocation, "entries")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(entries) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one member/score entry is required")
	}
	args := make([]string, 0, len(entries)*2+1)
	args = append(args, key)
	for _, entry := range entries {
		args = append(args, strconv.FormatFloat(entry.Score, 'f', -1, 64), entry.Member)
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "ZADD", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"added": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Sorted Set Range ---------------- */

func rangeDefinition() domain.NodeDefinition {
	startPin := kvnodes.Number("start", "Start rank", domain.PinInput, false)
	startPin.Default = float64(0)
	stopPin := kvnodes.Number("stop", "Stop rank (-1 = last)", domain.PinInput, false)
	stopPin.Default = float64(-1)
	return kvnodes.Definition("action:kv_zset_range", "KV Sorted Set Range", "Read scored members of a sorted set by rank.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			startPin,
			stopPin,
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.List("entries", "Entries", domain.PinOutput, false),
			kvnodes.Number("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "start", Label: "Start rank", Kind: "number", Placeholder: "0"},
			{Name: "stop", Label: "Stop rank (-1 = last)", Kind: "number", Placeholder: "-1"},
			{Name: "order", Label: "Order", Kind: "select", Options: []domain.Option{
				{Value: "asc", Label: "Lowest score first"},
				{Value: "desc", Label: "Highest score first"},
			}},
		},
		map[string]any{"key": "", "order": "asc"},
	)
}

func executeRange(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	start, err := kvnodes.Int(invocation, "start", 0)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	stop, err := kvnodes.Int(invocation, "stop", -1)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	command := "ZRANGE"
	if order, _ := invocation.Config["order"].(string); order == "desc" {
		command = "ZREVRANGE"
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{
		Command: command, Args: []string{key, fmt.Sprintf("%d", start), fmt.Sprintf("%d", stop), "WITHSCORES"},
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	// The flat [member, score, member, score] reply is reshaped into objects.
	flat, _ := result.Value.([]any)
	entries := make([]any, 0, len(flat)/2)
	for index := 0; index+1 < len(flat); index += 2 {
		member := fmt.Sprintf("%v", flat[index])
		score := 0.0
		switch value := flat[index+1].(type) {
		case string:
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				score = parsed
			}
		case float64:
			score = value
		case int64:
			score = float64(value)
		}
		entries = append(entries, map[string]any{"member": member, "score": score})
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"entries": entries, "count": int64(len(entries))},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Sorted Set Remove ---------------- */

func removeDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_zset_remove", "KV Sorted Set Remove", "Remove members from a sorted set.",
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
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	members, err := kvnodes.Strings(invocation, "members")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(members) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one member is required")
	}
	args := append([]string{key}, members...)
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "ZREM", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"removed": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
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
