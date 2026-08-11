package persistence

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestCollapseRerouteNodesKeepsFanoutAndStoresWaypoint(t *testing.T) {
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button"},
		{ID: "relay", Type: "flow:reroute", Position: domain.Position{X: 180, Y: 90}},
		{ID: "left", Type: "action:notification"},
		{ID: "right", Type: "action:notification"},
	}, Edges: []domain.FlowEdge{
		{ID: "into", Source: "start", SourceHandle: "out", Target: "relay", TargetHandle: "in", Kind: domain.PinExec},
		{ID: "left", Source: "relay", SourceHandle: "out", Target: "left", TargetHandle: "in", Kind: domain.PinExec},
		{ID: "right", Source: "relay", SourceHandle: "out", Target: "right", TargetHandle: "in", Kind: domain.PinExec},
	}}
	next, err := collapseRerouteNodes(definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Nodes) != 3 || len(next.Edges) != 2 {
		t.Fatalf("collapsed graph = %#v", next)
	}
	for _, edge := range next.Edges {
		if edge.Source != "start" || edge.SourceHandle != "out" || len(edge.Waypoints) != 1 || edge.Waypoints[0] != (domain.Position{X: 180, Y: 90}) {
			t.Fatalf("edge = %#v", edge)
		}
	}
}

func TestCollapseDataRerouteKeepsWireAndExistingWaypoints(t *testing.T) {
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{
		{ID: "source", Type: "data:constant"},
		{ID: "relay", Type: "data:reroute", Position: domain.Position{X: 50, Y: 60}},
		{ID: "sink", Type: "flow:branch"},
	}, Edges: []domain.FlowEdge{
		{ID: "into", Source: "source", SourceHandle: "value", Target: "relay", TargetHandle: "value", Kind: domain.PinData},
		{ID: "out", Source: "relay", SourceHandle: "value", Target: "sink", TargetHandle: "condition", Kind: domain.PinData, Waypoints: []domain.Position{{X: 300, Y: 12}}},
	}}
	next, err := collapseRerouteNodes(definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Nodes) != 2 || len(next.Edges) != 1 {
		t.Fatalf("collapsed graph = %#v", next)
	}
	edge := next.Edges[0]
	if edge.Source != "source" || edge.SourceHandle != "value" || edge.Target != "sink" || edge.TargetHandle != "condition" {
		t.Fatalf("collapsed edge = %#v", edge)
	}
	if len(edge.Waypoints) != 2 || edge.Waypoints[0] != (domain.Position{X: 50, Y: 60}) || edge.Waypoints[1] != (domain.Position{X: 300, Y: 12}) {
		t.Fatalf("waypoints = %#v", edge.Waypoints)
	}
}

func TestCollapseChainedReroutesPreservesWaypointOrder(t *testing.T) {
	definition := domain.FlowDefinition{Nodes: []domain.FlowNode{
		{ID: "start", Type: "trigger:button"},
		{ID: "first", Type: "flow:reroute", Position: domain.Position{X: 100, Y: 40}},
		{ID: "second", Type: "flow:reroute", Position: domain.Position{X: 220, Y: 60}},
		{ID: "sink", Type: "action:notification"},
	}, Edges: []domain.FlowEdge{
		{ID: "into-first", Source: "start", SourceHandle: "out", Target: "first", TargetHandle: "in", Kind: domain.PinExec},
		{ID: "first-second", Source: "first", SourceHandle: "out", Target: "second", TargetHandle: "in", Kind: domain.PinExec},
		{ID: "out", Source: "second", SourceHandle: "out", Target: "sink", TargetHandle: "in", Kind: domain.PinExec},
	}}
	next, err := collapseRerouteNodes(definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Nodes) != 2 || len(next.Edges) != 1 {
		t.Fatalf("collapsed graph = %#v", next)
	}
	edge := next.Edges[0]
	if edge.Source != "start" || edge.Target != "sink" {
		t.Fatalf("collapsed edge = %#v", edge)
	}
	want := []domain.Position{{X: 100, Y: 40}, {X: 220, Y: 60}}
	if len(edge.Waypoints) != len(want) {
		t.Fatalf("waypoints = %#v, want %#v", edge.Waypoints, want)
	}
	for index, point := range want {
		if edge.Waypoints[index] != point {
			t.Fatalf("waypoint %d = %#v, want %#v", index, edge.Waypoints[index], point)
		}
	}
}

func TestCollapseRerouteNodesRejectsMalformedGraphs(t *testing.T) {
	tests := []struct {
		name       string
		definition domain.FlowDefinition
		wantErr    string
	}{
		{
			name: "no incoming wire",
			definition: domain.FlowDefinition{Nodes: []domain.FlowNode{
				{ID: "relay", Type: "flow:reroute"},
				{ID: "sink", Type: "action:notification"},
			}, Edges: []domain.FlowEdge{
				{ID: "out", Source: "relay", SourceHandle: "out", Target: "sink", TargetHandle: "in", Kind: domain.PinExec},
			}},
			wantErr: "one input",
		},
		{
			name: "no outgoing wires",
			definition: domain.FlowDefinition{Nodes: []domain.FlowNode{
				{ID: "start", Type: "trigger:button"},
				{ID: "relay", Type: "data:reroute"},
			}, Edges: []domain.FlowEdge{
				{ID: "in", Source: "start", SourceHandle: "payload", Target: "relay", TargetHandle: "value", Kind: domain.PinData},
			}},
			wantErr: "at least one output",
		},
		{
			name: "mixed pin kinds",
			definition: domain.FlowDefinition{Nodes: []domain.FlowNode{
				{ID: "start", Type: "trigger:button"},
				{ID: "relay", Type: "flow:reroute"},
				{ID: "sink", Type: "flow:branch"},
			}, Edges: []domain.FlowEdge{
				{ID: "in", Source: "start", SourceHandle: "out", Target: "relay", TargetHandle: "in", Kind: domain.PinExec},
				{ID: "out", Source: "relay", SourceHandle: "value", Target: "sink", TargetHandle: "condition", Kind: domain.PinData},
			}},
			wantErr: "mixes pin kinds",
		},
		{
			name: "multiple incoming wires",
			definition: domain.FlowDefinition{Nodes: []domain.FlowNode{
				{ID: "a", Type: "trigger:button"},
				{ID: "b", Type: "trigger:button"},
				{ID: "relay", Type: "flow:reroute"},
				{ID: "sink", Type: "action:notification"},
			}, Edges: []domain.FlowEdge{
				{ID: "in-a", Source: "a", SourceHandle: "out", Target: "relay", TargetHandle: "in", Kind: domain.PinExec},
				{ID: "in-b", Source: "b", SourceHandle: "out", Target: "relay", TargetHandle: "in", Kind: domain.PinExec},
				{ID: "out", Source: "relay", SourceHandle: "out", Target: "sink", TargetHandle: "in", Kind: domain.PinExec},
			}},
			wantErr: "one input",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := collapseRerouteNodes(test.definition); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("collapseRerouteNodes() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
