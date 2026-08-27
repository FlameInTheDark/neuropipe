// Package scan registers the Redis key scanning, publish, and server info nodes.
package scan

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func RegisterScan(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: scanDefinition(), Executor: executeScan})
}

func RegisterPublish(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: publishDefinition(), Executor: executePublish})
}

func RegisterInfo(registrar nodes.Registrar) error {
	return registrar.Register(nodes.Implementation{Metadata: infoDefinition(), Resolver: resolveInfo, Executor: executeInfo})
}

/* ---------------- KV Scan ---------------- */

func scanDefinition() domain.NodeDefinition {
	cursorPin := kvnodes.Number("cursor", "Cursor", domain.PinInput, false)
	cursorPin.Default = float64(0)
	return kvnodes.Definition("action:kv_scan", "KV Scan", "Page through matching keys with a cursor.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("pattern", "Pattern", domain.PinInput, false),
			cursorPin,
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.List("keys", "Keys", domain.PinOutput, false),
			kvnodes.Number("nextCursor", "Next cursor", domain.PinOutput, false),
			kvnodes.Bool("done", "Done", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "pattern", Label: "Match pattern", Kind: "string", Placeholder: "user:*"},
			{Name: "count", Label: "Page size", Kind: "number"},
			{Name: "typeFilter", Label: "Type filter", Kind: "select", Options: []domain.Option{
				{Value: "", Label: "Any type"},
				{Value: "string", Label: "String"},
				{Value: "hash", Label: "Hash"},
				{Value: "list", Label: "List"},
				{Value: "set", Label: "Set"},
				{Value: "zset", Label: "Sorted set"},
				{Value: "stream", Label: "Stream"},
			}},
		},
		map[string]any{"pattern": "", "count": 100, "typeFilter": ""},
	)
}

func executeScan(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	pattern := kvnodes.String(invocation, "pattern")
	cursor, err := kvnodes.Int(invocation, "cursor", 0)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if cursor < 0 {
		cursor = 0
	}
	count := int64(100)
	if raw, exists := invocation.Config["count"]; exists {
		switch typed := raw.(type) {
		case float64:
			count = int64(typed)
		case int64:
			count = typed
		case int:
			count = int64(typed)
		case string:
			if parsed, err := strconv.ParseInt(typed, 10, 64); err == nil {
				count = parsed
			}
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 500 {
		count = 500
	}
	typeFilter, _ := invocation.Config["typeFilter"].(string)
	args := []string{strconv.FormatUint(uint64(cursor), 10)}
	if pattern != "" {
		args = append(args, "MATCH", pattern)
	}
	args = append(args, "COUNT", strconv.FormatInt(count, 10))
	if typeFilter != "" {
		args = append(args, "TYPE", typeFilter)
	}
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "SCAN", Args: args})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	// SCAN replies with [cursor, keys].
	pair, _ := result.Value.([]any)
	nextCursor := uint64(0)
	keys := []any{}
	if len(pair) == 2 {
		nextCursor = replyCursor(pair[0])
		if list, ok := pair[1].([]any); ok {
			keys = list
		}
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{
			"keys":       keys,
			"nextCursor": nextCursor,
			"done":       nextCursor == 0,
		},
		Ports: []string{"out"},
	}, nil
}

func replyCursor(value any) uint64 {
	switch typed := value.(type) {
	case int64:
		if typed < 0 {
			return 0
		}
		return uint64(typed)
	case float64:
		if typed < 0 {
			return 0
		}
		return uint64(typed)
	case string:
		parsed, _ := strconv.ParseUint(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

/* ---------------- KV Publish ---------------- */

func publishDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_publish", "KV Publish", "Publish a message on a channel.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
			kvnodes.Text("channel", "Channel", domain.PinInput, true),
			kvnodes.Text("message", "Message", domain.PinInput, true),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Number("receivers", "Receivers", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "channel", Label: "Channel", Kind: "string", Placeholder: "events:user-signup"},
			{Name: "message", Label: "Message", Kind: "string"},
		},
		map[string]any{"channel": "", "message": ""},
	)
}

func executePublish(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	channel := kvnodes.String(invocation, "channel")
	if channel == "" {
		return nodes.ExecutionResult{}, fmt.Errorf("channel is required")
	}
	message := kvnodes.String(invocation, "message")
	result, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "PUBLISH", Args: []string{channel, message}})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	receivers := int64(0)
	switch value := result.Value.(type) {
	case int64:
		receivers = value
	case float64:
		receivers = int64(value)
	}
	return nodes.ExecutionResult{
		Outputs: map[string]any{"receivers": receivers},
		Ports:   []string{"out"},
	}, nil
}

/* ---------------- KV Server Info ---------------- */

func infoDefinition() domain.NodeDefinition {
	return kvnodes.Definition("action:kv_info", "KV Server Info", "Read the connected server's version, flavour, and key counts.",
		[]domain.NodePort{
			kvnodes.Exec("in", "Exec", domain.PinInput),
		},
		[]domain.NodePort{
			kvnodes.Exec("out", "Then", domain.PinOutput),
			kvnodes.Any("info", "Info", domain.PinOutput),
			kvnodes.Text("version", "Version", domain.PinOutput, false),
			kvnodes.Text("flavor", "Flavor", domain.PinOutput, false),
			kvnodes.Number("keyCount", "Key count", domain.PinOutput, false),
		},
		nil,
		map[string]any{},
	)
}

// resolveInfo documents the info object's fields for editor discovery.
func resolveInfo(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := infoDefinition()
	infoPin := result.Outputs[1]
	infoPin.Fields = []domain.DataField{
		{Path: "flavor", Label: "Flavor", DataType: domain.DataText, Description: "Server flavour (redis or valkey)."},
		{Path: "version", Label: "Version", DataType: domain.DataText, Description: "Server version string."},
		{Path: "uptimeSeconds", Label: "Uptime seconds", DataType: domain.DataNumber, Description: "Server uptime in seconds."},
		{Path: "connectedClients", Label: "Connected clients", DataType: domain.DataNumber, Description: "Active client connections."},
		{Path: "usedMemory", Label: "Used memory", DataType: domain.DataNumber, Description: "Memory used in bytes."},
		{Path: "usedMemoryHuman", Label: "Used memory (human)", DataType: domain.DataText, Description: "Memory used as text."},
		{Path: "totalKeys", Label: "Total keys", DataType: domain.DataNumber, Description: "Keys in the selected logical database."},
		{Path: "databases", Label: "Databases", DataType: domain.DataList, Description: "Key counts per logical database index."},
	}
	result.Outputs[1] = infoPin
	return result, nil
}

func executeInfo(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	server, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "INFO"})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	keyspace, err := kvnodes.ExecuteCommand(ctx, invocation, runtime, domain.KVCommandRequest{Command: "DBSIZE"})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	info := map[string]any{}
	if fields, ok := server.Value.(string); ok {
		info = parseInfoText(fields)
	}
	totalKeys := int64(0)
	switch value := keyspace.Value.(type) {
	case int64:
		totalKeys = value
	case float64:
		totalKeys = int64(value)
	}
	info["totalKeys"] = totalKeys
	version, _ := info["version"].(string)
	flavor, _ := info["flavor"].(string)
	return nodes.ExecutionResult{
		Outputs: map[string]any{"info": info, "version": version, "flavor": flavor, "keyCount": totalKeys},
		Ports:   []string{"out"},
	}, nil
}

// parseInfoText extracts the display fields from an INFO server section.
func parseInfoText(section string) map[string]any {
	result := map[string]any{"flavor": "redis"}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch name {
		case "redis_version":
			result["version"] = value
		case "valkey_version":
			result["version"] = value
			result["flavor"] = "valkey"
		case "uptime_in_seconds":
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				result["uptimeSeconds"] = parsed
			}
		case "connected_clients":
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				result["connectedClients"] = parsed
			}
		case "used_memory":
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				result["usedMemory"] = parsed
			}
		case "used_memory_human":
			result["usedMemoryHuman"] = value
		}
	}
	return result
}
