package pipeline

import (
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestValidateTwitchEventTriggerPipeline(t *testing.T) {
	registry := catalog.New()

	// Test with edges connected to resolved output pins
	flow := domain.FlowDefinition{
		SchemaVersion: domain.GraphSchemaV3,
		Nodes: []domain.FlowNode{
			{
				ID:   "twitch-event-1",
				Type: "twitch:event",
				Data: map[string]any{
					"config": map[string]any{
						"eventType":  "channel.chat.message",
						"identityId": "some-identity",
					},
				},
			},
			{
				ID:   "notif",
				Type: "action:notification",
				Data: map[string]any{
					"config": map[string]any{
						"title":   "Test",
						"message": "Hello",
					},
				},
			},
		},
		Edges: []domain.FlowEdge{
			{ID: "e1", Source: "twitch-event-1", SourceHandle: "out", Target: "notif", TargetHandle: "in", Kind: domain.PinExec},
			{ID: "e2", Source: "twitch-event-1", SourceHandle: "text", Target: "notif", TargetHandle: "title", Kind: domain.PinData},
		},
	}
	if err := Validate(flow, registry); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
