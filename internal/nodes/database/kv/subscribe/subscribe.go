// Package subscribe registers the Redis pub/sub trigger module. The node
// itself owns only metadata; message delivery belongs to the kvsub service.
package subscribe

import (
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	kvnodes "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

// New creates the KV Subscribe trigger module implementation.
func New() Node {
	return Node{Metadata: definition()}
}

func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	fields := append([]domain.ConfigField{kvnodes.DatabaseField()}, []domain.ConfigField{
		{Name: "channels", Label: "Channels (comma-separated)", Kind: "string", Placeholder: "events:signup, events:order"},
		{Name: "patterns", Label: "Channel patterns (comma-separated)", Kind: "string", Placeholder: "events:*"},
	}...)
	return domain.NodeDefinition{
		Type: "kv:subscribe", Category: "KV Store", Label: "KV Subscribe Trigger",
		Description: "Start a trusted pipeline from a pub/sub message on a registered KV database.",
		Icon:        "radio", Color: "#dc382d", Mode: domain.NodeEvent, TriggerKind: domain.TriggerKV,
		PortContractOwned: true, Capabilities: []domain.Capability{domain.CapabilityNetwork},
		Inputs: []domain.NodePort{},
		Outputs: []domain.NodePort{
			kvnodes.Exec("out", "Start", domain.PinOutput),
			kvnodes.Text("channel", "Channel", domain.PinOutput, false),
			kvnodes.Text("message", "Message", domain.PinOutput, false),
			kvnodes.Text("pattern", "Pattern", domain.PinOutput, false),
			kvnodes.Text("receivedAt", "Received at", domain.PinOutput, false),
		},
		Fields:        fields,
		DefaultConfig: map[string]any{"databaseId": "", "channels": "", "patterns": ""},
		Source:        "builtin",
	}
}
