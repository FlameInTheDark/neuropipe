// Package list registers the Redis list KV nodes.
package list

import (
	"context"
	"fmt"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func RegisterPush(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: pushDefinition(), Executor: executePush})
}

func RegisterPop(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: popDefinition(), Executor: executePop})
}

func RegisterRange(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: rangeDefinition(), Executor: executeRange})
}

/* ---------------- KV List Push ---------------- */

func pushDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_list_push", "KV List Push", "Push values onto the head or tail of a list.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.List("values", "Values", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("length", "List length", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string", Placeholder: "queue:jobs"},
			{Name: "values", Label: "Values", Kind: "kv-string-list"},
			{Name: "side", Label: "Side", Kind: "select", Options: []domain.Option{
				{Value: "right", Label: "Tail (RPUSH)"},
				{Value: "left", Label: "Head (LPUSH)"},
			}},
		},
		map[string]any{"key": "", "values": []any{}, "side": "right"},
	)
}

func executePush(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	values, err := kvnodes.Strings(invocation, "values")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if len(values) == 0 {
		return nodes.ExecutionResult{}, fmt.Errorf("at least one value is required")
	}
	command := "RPUSH"
	if side, _ := invocation.Config["side"].(string); side == "left" {
		command = "LPUSH"
	}
	args := append([]string{key}, values...)
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"length": replyInt(result)},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV List Pop ---------------- */

func popDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_list_pop", "KV List Pop", "Pop one or more values off a list.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			kvnodes.Number("count", "Count", domain.PinInput, false),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.List("values", "Values", domain.PinOutput, false),
			kvnodes.Text("value", "Value", domain.PinOutput, false),
			kvnodes.Bool("found", "Found", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "count", Label: "Count (1 = single value)", Kind: "number"},
			{Name: "side", Label: "Side", Kind: "select", Options: []domain.Option{
				{Value: "left", Label: "Head (LPOP)"},
				{Value: "right", Label: "Tail (RPOP)"},
			}},
		},
		map[string]any{"key": "", "side": "left"},
	)
}

func executePop(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	key := kvnodes.String(invocation, "key")
	if key == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("key is required")
	}
	count, err := kvnodes.Int(invocation, "count", 1)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	command := "LPOP"
	if side, _ := invocation.Config["side"].(string); side == "right" {
		command = "RPOP"
	}
	args := []string{key}
	if count > 0 && count != 1 {
		args = append(args, fmt.Sprintf("%d", count))
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: command, Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"values": []any{}, "value": "", "found": false}
	if result.IsNil {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
	}
	switch value := result.Value.(type) {
	case string:
		// Redis 6 single-element form.
		outputs["value"] = value
		outputs["values"] = []any{value}
		outputs["found"] = true
	case []any:
		outputs["values"] = value
		if len(value) > 0 {
			outputs["found"] = true
			if text, ok := value[0].(string); ok {
				outputs["value"] = text
			}
		}
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"out"}}, nil
}

/* ---------------- KV List Range ---------------- */

func rangeDefinition() domain.NodeDefinition {
	startPin := kvnodes.Number("start", "Start index", domain.PinInput, false)
	startPin.Default = float64(0)
	stopPin := kvnodes.Number("stop", "Stop index (-1 = last)", domain.PinInput, false)
	stopPin.Default = float64(-1)
	return kvnodes.Definition("action:kv_list_range", "KV List Range", "Read a slice of a list by index.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("key", "Key", domain.PinInput, true),
			startPin,
			stopPin,
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.List("items", "Items", domain.PinOutput, false),
			kvnodes.Number("count", "Count", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "key", Label: "Key", Kind: "string"},
			{Name: "start", Label: "Start index", Kind: "number", Placeholder: "0"},
			{Name: "stop", Label: "Stop index (-1 = last)", Kind: "number", Placeholder: "-1"},
		},
		map[string]any{"key": ""},
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
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{
		Command: "LRANGE", Args: []string{key, fmt.Sprintf("%d", start), fmt.Sprintf("%d", stop)},
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	items, _ := result.Value.([]any)
	if items == nil {
		items = []any{}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"items": items, "count": int64(len(items))},
		Ports:   []string{"out"},
	}, nil
}

func replyInt(result domain.KVCommandResult) int64 {
	switch value := result.Value.(type) {
	case int64:
		return value
	case float64:
		return int64(value)
	case string:
		var parsed int64
		fmt.Sscanf(value, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}
