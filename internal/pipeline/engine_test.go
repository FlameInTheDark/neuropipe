package pipeline

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestHTTPNodeSendsConfiguredHeadersAndCustomUserAgent(t *testing.T) {
	var requestHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestHeaders = request.Header.Clone()
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("request", "action:http", map[string]any{
			"url":                server.URL,
			"headers":            []any{map[string]any{"id": "accept", "name": "Accept", "value": "application/json"}, map[string]any{"id": "trace", "name": "X-Trace-ID", "value": "trace-42"}},
			"useCustomUserAgent": true,
			"userAgent":          "Neuropipe tests/1.0",
		}),
	}, Edges: []domain.FlowEdge{execEdge("start-request", "start", "out", "request", "in")}}

	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := requestHeaders.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q, want application/json", got)
	}
	if got := requestHeaders.Get("X-Trace-ID"); got != "trace-42" {
		t.Fatalf("X-Trace-ID = %q, want trace-42", got)
	}
	if got := requestHeaders.Get("User-Agent"); got != "Neuropipe tests/1.0" {
		t.Fatalf("User-Agent = %q, want custom value", got)
	}
}

func TestBlueprintExecutesV3JavaScriptWithTypedOutput(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("script", "action:javascript", map[string]any{
			"code":   "return { message: 'JavaScript ready' };",
			"inputs": []any{},
			"outputs": []any{map[string]any{
				"id": "message", "label": "Message", "type": map[string]any{"kind": "string"}, "required": true,
			}},
			"capabilities": []any{},
		}),
		cfgNode("notice", "action:notification", map[string]any{"title": "JavaScript"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-script", "start", "out", "script", "in"),
		execEdge("script-notice", "script", "out", "notice", "in"),
		dataEdge("script-message", "script", "message", "notice", "message"),
	}}
	sender := &recordingNotificationSender{}
	result, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := sender.calls, []notificationCall{{title: "JavaScript", message: "JavaScript ready"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("notifications = %#v, want %#v", got, want)
	}
	if len(result.NodeRuns) != 3 || result.NodeRuns[1].NodeType != "action:javascript" {
		t.Fatalf("node runs = %#v", result.NodeRuns)
	}
}

func TestHTTPResultCanFeedBreakObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("Blueprint ready"))
	}))
	defer server.Close()

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("request", "action:http", map[string]any{"url": server.URL, "method": "GET"}),
		cfgNode("break", "data:break_object", map[string]any{"outputs": []any{
			map[string]any{"id": "body", "label": "Body", "path": "body", "dataType": "text"},
			map[string]any{"id": "headers", "label": "Headers", "path": "headers", "dataType": "object"},
			// HTTP status is a Go int inside the packet; a declared number
			// (float) contract must still accept it.
			map[string]any{"id": "status", "label": "Status", "path": "status", "dataType": "number"},
		}}),
		cfgNode("notice", "action:notification", map[string]any{"title": "HTTP"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-request", "start", "out", "request", "in"),
		execEdge("request-notice", "request", "out", "notice", "in"),
		dataEdge("request-break", "request", "result", "break", "source"),
		dataEdge("break-notice", "break", "body", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	result, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, run := range result.NodeRuns {
		if run.NodeID != "break" {
			continue
		}
		outputs, ok := run.Output.(map[string]any)
		if !ok {
			t.Fatalf("break output = %#v", run.Output)
		}
		if got, ok := outputs["status"]; !ok || got != 200 {
			t.Fatalf("break status = %#v, want 200", outputs["status"])
		}
	}
	if got, want := sender.calls, []notificationCall{{title: "HTTP", message: "Blueprint ready"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("notifications = %#v, want %#v", got, want)
	}
}

// Regression: an HTTP status (a Go int) compared through Break Object → Equals
// with a Constant number (a float64) must report equality.
func TestHTTPStatusSurvivesBreakObjectIntoEqualsAgainstConstant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("request", "action:http", map[string]any{"url": server.URL, "method": "GET"}),
		cfgNode("break", "data:break_object", map[string]any{"outputs": []any{
			map[string]any{"id": "status", "label": "Status", "path": "status", "dataType": "number"},
		}}),
		cfgNode("constant", "data:constant", map[string]any{"type": "number", "value": 200.0}),
		cfgNode("equal", "data:equals", nil),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("match", "action:notification", map[string]any{"title": "Status", "message": "OK"}),
		cfgNode("mismatch", "action:notification", map[string]any{"title": "Status", "message": "Unexpected"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-request", "start", "out", "request", "in"),
		execEdge("request-branch", "request", "out", "branch", "in"),
		dataEdge("request-break", "request", "result", "break", "source"),
		dataEdge("break-equal", "break", "status", "equal", "left"),
		dataEdge("constant-equal", "constant", "value", "equal", "right"),
		dataEdge("equal-branch", "equal", "value", "branch", "condition"),
		execEdge("branch-match", "branch", "true", "match", "in"),
		execEdge("branch-mismatch", "branch", "false", "mismatch", "in"),
	}}

	sender := &recordingNotificationSender{}
	if _, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := sender.calls, []notificationCall{{title: "Status", message: "OK"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("notifications = %#v, want %#v", got, want)
	}
}

type notificationCall struct {
	title   string
	message string
}

type recordingNotificationSender struct {
	calls []notificationCall
	err   error
}

type recordingChatWriter struct {
	replies  []string
	statuses []string
	history  []domain.ChatMessage
}

func (w *recordingChatWriter) AppendChatReply(_ context.Context, runID, content string) (domain.ChatMessage, error) {
	w.replies = append(w.replies, runID+":"+content)
	return domain.ChatMessage{ID: "message-1", ChatRunID: runID, Content: content}, nil
}

func (w *recordingChatWriter) UpdateChatStatus(_ context.Context, runID, status string) error {
	w.statuses = append(w.statuses, runID+":"+status)
	return nil
}

func (w *recordingChatWriter) ReadChatHistory(_ context.Context, _ string, _ int) ([]domain.ChatMessage, error) {
	return append([]domain.ChatMessage(nil), w.history...), nil
}

func (s *recordingNotificationSender) Send(_ context.Context, title, message string) error {
	s.calls = append(s.calls, notificationCall{title: title, message: message})
	return s.err
}

func cfgNode(id, nodeType string, config map[string]any) domain.FlowNode {
	return domain.FlowNode{ID: id, Type: nodeType, Data: map[string]any{"config": config}}
}
func execEdge(id, source, sourcePin, target, targetPin string) domain.FlowEdge {
	return domain.FlowEdge{ID: id, Source: source, SourceHandle: sourcePin, Target: target, TargetHandle: targetPin, Kind: domain.PinExec}
}
func dataEdge(id, source, sourcePin, target, targetPin string) domain.FlowEdge {
	return domain.FlowEdge{ID: id, Source: source, SourceHandle: sourcePin, Target: target, TargetHandle: targetPin, Kind: domain.PinData}
}

func TestBlueprintCachesPureDataAndRoutesExec(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("truth", "data:constant", map[string]any{"type": "boolean", "value": true}),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"), dataEdge("truth-condition", "truth", "value", "branch", "condition"), execEdge("true-notice", "branch", "true", "notice", "in"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 4 {
		t.Fatalf("NodeRuns = %d, want event, pure value, branch, notification", len(result.NodeRuns))
	}
	if result.NodeRuns[2].NodeID != "branch" || result.NodeRuns[3].NodeID != "notice" {
		t.Fatalf("exec order = %#v", result.NodeRuns)
	}
	pureRuns := 0
	for _, run := range result.NodeRuns {
		if run.NodeID == "truth" {
			pureRuns++
		}
	}
	if pureRuns != 1 {
		t.Fatalf("pure value evaluations = %d, want one cached evaluation", pureRuns)
	}
}

func TestBlueprintWaypointEdgesExecuteLikeDirectEdges(t *testing.T) {
	waypoints := []domain.Position{{X: 40, Y: 60}, {X: -10, Y: 200}}
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("truth", "data:constant", map[string]any{"value": true, "type": "boolean"}),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		{ID: "start-branch", Source: "start", SourceHandle: "out", Target: "branch", TargetHandle: "in", Kind: domain.PinExec, Waypoints: waypoints},
		{ID: "truth-condition", Source: "truth", SourceHandle: "value", Target: "branch", TargetHandle: "condition", Kind: domain.PinData, Waypoints: waypoints},
		{ID: "branch-notice", Source: "branch", SourceHandle: "true", Target: "notice", TargetHandle: "in", Kind: domain.PinExec, Waypoints: waypoints[:1]},
	}}
	sender := &recordingNotificationSender{}
	result, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(sender.calls) != 1 || sender.calls[0].message != "Done" {
		t.Fatalf("notifications = %#v", sender.calls)
	}
	if len(result.NodeRuns) != 4 || result.NodeRuns[2].NodeID != "branch" || result.NodeRuns[3].NodeID != "notice" {
		t.Fatalf("NodeRuns = %#v", result.NodeRuns)
	}
}

func TestBlueprintTypeAssertRejectsMismatchedRuntimeValue(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("constant", "data:constant", map[string]any{"value": "not a Boolean"}),
		cfgNode("assert", "data:type_assert", map[string]any{"typeSpec": map[string]any{"kind": "bool"}}),
		cfgNode("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("exec", "start", "out", "branch", "in"),
		dataEdge("source-assert", "constant", "value", "assert", "value"),
		dataEdge("assert-condition", "assert", "value", "branch", "condition"),
	}}
	if _, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{}); err == nil || !strings.Contains(err.Error(), "type assertion failed") {
		t.Fatalf("Execute() error = %v, want type assertion failure", err)
	}
}

func TestChatTriggerPassesExplicitPinsToReplyNode(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("chat", "trigger:chat", map[string]any{"label": "Support"}),
		cfgNode("reply", "action:chat_reply", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("chat-reply", "chat", "out", "reply", "in"),
		dataEdge("chat-text", "chat", "text", "reply", "text"),
		dataEdge("chat-run", "chat", "chatRunId", "reply", "chatRunId"),
	}}
	writer := &recordingChatWriter{}
	_, err := NewEngine(catalog.New(), nil, nil, WithChatWriter(writer)).Execute(context.Background(), flow, "chat", Packet{"text": "Hello", "chatId": "conversation-1", "chatRunId": "run-1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if want := []string{"run-1:Hello"}; !reflect.DeepEqual(writer.replies, want) {
		t.Fatalf("replies = %#v, want %#v", writer.replies, want)
	}
}

func TestGetFieldReadsNestedValueFromPacket(t *testing.T) {
	tests := []struct {
		name  string
		value any
		path  string
		want  any
	}{
		{
			name: "terminal output",
			value: Packet{"terminal": map[string]any{
				"command": "ping google.com",
				"output":  "Pinging google.com",
			}},
			path: "terminal.output",
			want: "Pinging google.com",
		},
		{
			name:  "plain JSON object",
			value: map[string]any{"result": map[string]any{"value": true}},
			path:  "result.value",
			want:  true,
		},
		{
			name:  "HTTP header map and list",
			value: http.Header{"Content-Type": []string{"application/json"}},
			path:  "Content-Type.0",
			want:  "application/json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := valueAtAny(test.value, test.path); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("valueAtAny(%#v, %q) = %#v, want %#v", test.value, test.path, got, test.want)
			}
		})
	}
}

func TestGetFieldEmitsMultipleTypedOutputs(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `{"terminal":{"command":"Get-Date","output":"Friday"},"ready":true}`}),
		cfgNode("parsed", "data:json_parse", nil),
		cfgNode("fields", "data:get_field", map[string]any{"outputs": []any{
			map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
			map[string]any{"id": "ready", "label": "Ready", "path": "ready", "dataType": "boolean"},
		}}),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"),
		dataEdge("source-parsed", "source", "value", "parsed", "text"),
		dataEdge("parsed-fields", "parsed", "value", "fields", "source"),
		dataEdge("fields-branch", "fields", "ready", "branch", "condition"),
		execEdge("branch-notice", "branch", "true", "notice", "in"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, run := range result.NodeRuns {
		if run.NodeID != "fields" {
			continue
		}
		want := map[string]any{"command": "Get-Date", "ready": true}
		if !reflect.DeepEqual(run.Output, want) {
			t.Fatalf("Get Field output = %#v, want %#v", run.Output, want)
		}
		return
	}
	t.Fatal("Get Field node was not evaluated")
}

func TestBuildAndBreakObjectUseConfiguredTypedPins(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("name", "data:constant", map[string]any{"type": "text", "value": "Ada"}),
		cfgNode("active", "data:constant", map[string]any{"type": "boolean", "value": true}),
		cfgNode("build", "data:build_object", map[string]any{"fields": []any{
			map[string]any{"id": "name", "label": "Name", "key": "customer.name", "dataType": "text"},
			map[string]any{"id": "active", "label": "Active", "key": "customer.active", "dataType": "boolean"},
		}}),
		cfgNode("break", "data:break_object", map[string]any{"outputs": []any{
			map[string]any{"id": "name", "label": "Name", "path": "customer.name", "dataType": "text"},
			map[string]any{"id": "active", "label": "Active", "path": "customer.active", "dataType": "boolean"},
		}}),
		cfgNode("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"),
		dataEdge("name-build", "name", "value", "build", "name"),
		dataEdge("active-build", "active", "value", "build", "active"),
		dataEdge("build-break", "build", "object", "break", "source"),
		dataEdge("break-branch", "break", "active", "branch", "condition"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	outputs := map[string]any{}
	for _, run := range result.NodeRuns {
		if run.NodeID == "build" || run.NodeID == "break" {
			outputs[run.NodeID] = run.Output
		}
	}
	if got, want := outputs["build"], map[string]any{"object": map[string]any{"customer": map[string]any{"name": "Ada", "active": true}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Build Object output = %#v, want %#v", got, want)
	}
	if got, want := outputs["break"], map[string]any{"name": "Ada", "active": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Break Object output = %#v, want %#v", got, want)
	}
}

func TestBreakObjectReadsListKeyPaths(t *testing.T) {
	if got, want := valueAtAny(map[string]any{"items": []any{map[string]any{"name": "First"}}}, "items.0.name"), "First"; got != want {
		t.Fatalf("valueAtAny list path = %#v, want %#v", got, want)
	}
}

func TestBlueprintObjectTypeAcceptsNamedMapsAndStructs(t *testing.T) {
	type pluginResult struct {
		Message string `json:"message"`
	}
	for _, value := range []any{
		http.Header{"Content-Type": []string{"application/json"}},
		&pluginResult{Message: "Ready"},
	} {
		if !matchesDataType(value, domain.DataObject) {
			t.Fatalf("matchesDataType(%T, object) = false, want true", value)
		}
	}
	if got, want := valueAtAny(&pluginResult{Message: "Ready"}, "message"), "Ready"; got != want {
		t.Fatalf("valueAtAny struct field = %#v, want %#v", got, want)
	}
}

func TestDesktopNotificationUsesConfiguredSender(t *testing.T) {
	tests := []struct {
		name    string
		sendErr error
		wantErr string
	}{
		{name: "sends Windows notification"},
		{name: "reports native delivery failure", sendErr: errors.New("notifications are disabled"), wantErr: "send desktop notification"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := &recordingNotificationSender{err: test.sendErr}
			flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
				cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
				cfgNode("notice", "action:notification", map[string]any{"title": "Ready", "message": "Pipeline finished"}),
			}, Edges: []domain.FlowEdge{
				execEdge("start-notice", "start", "out", "notice", "in"),
			}}

			result, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Execute() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(sender.calls) != 1 || sender.calls[0] != (notificationCall{title: "Ready", message: "Pipeline finished"}) {
				t.Fatalf("notification calls = %#v", sender.calls)
			}
			wantOutput := map[string]any{"result": Packet{"notification": map[string]any{"title": "Ready", "message": "Pipeline finished"}}}
			if got := result.NodeRuns[len(result.NodeRuns)-1].Output; !reflect.DeepEqual(got, wantOutput) {
				t.Fatalf("notification output = %#v, want %#v", got, wantOutput)
			}
		})
	}
}

func TestBlueprintReusesImpureOutputInsteadOfExecutingAgain(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}), cfgNode("truth", "data:constant", map[string]any{"type": "boolean", "value": true}),
		cfgNode("store", "flow:set_variable", map[string]any{"name": "Ready"}), cfgNode("branch", "flow:branch", nil), cfgNode("truthCast", "data:cast", map[string]any{"target": "boolean"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"), dataEdge("truth-store", "truth", "value", "store", "value"), execEdge("store-branch", "store", "out", "branch", "in"), dataEdge("store-truthCast", "store", "result", "truthCast", "value"), dataEdge("truthCast-branch", "truthCast", "value", "branch", "condition"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	count := 0
	for _, run := range result.NodeRuns {
		if run.NodeID == "store" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("store executions = %d, want one cached execution", count)
	}
}

func TestBlueprintDoesNotReuseVariablesAcrossRuns(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("write", "trigger:button", map[string]any{"label": "Write"}),
		cfgNode("read", "trigger:button", map[string]any{"label": "Read"}),
		cfgNode("value", "data:constant", map[string]any{"type": "boolean", "value": true}),
		cfgNode("set", "flow:set_variable", map[string]any{"name": "RunLocal"}),
		cfgNode("get", "data:get_variable", map[string]any{"name": "RunLocal"}),
		cfgNode("asBool", "data:cast", map[string]any{"target": "boolean"}),
		cfgNode("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("write-set", "write", "out", "set", "in"),
		dataEdge("value-set", "value", "value", "set", "value"),
		execEdge("read-branch", "read", "out", "branch", "in"),
		dataEdge("get-asBool", "get", "value", "asBool", "value"),
		dataEdge("asBool-branch", "asBool", "value", "branch", "condition"),
	}}
	engine := NewEngine(catalog.New(), nil, nil)
	if _, err := engine.Execute(context.Background(), flow, "write", Packet{}); err != nil {
		t.Fatalf("write Execute() error = %v", err)
	}
	if _, err := engine.Execute(context.Background(), flow, "read", Packet{}); err == nil || !strings.Contains(err.Error(), "has not been set in this execution") {
		t.Fatalf("read Execute() error = %v, want execution-scoped variable error", err)
	}
}

func TestBlueprintFailsWhenImpureDataWasNotExecuted(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}), cfgNode("unrun", "flow:set_variable", map[string]any{"name": "Never"}), cfgNode("asBool", "data:cast", map[string]any{"target": "boolean"}), cfgNode("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{execEdge("start-branch", "start", "out", "branch", "in"), dataEdge("unrun-asBool", "unrun", "result", "asBool", "value"), dataEdge("asBool-condition", "asBool", "value", "branch", "condition")}}
	_, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "has an Exec pin but has not run") {
		t.Fatalf("Execute() error = %v, want unexecuted impure-data failure", err)
	}
}

func TestBlueprintForEachScopesIterationData(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}), cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `["a","b"]`}), cfgNode("items", "data:cast", map[string]any{"target": "list"}), cfgNode("loop", "flow:for_each", nil), cfgNode("store", "flow:set_variable", map[string]any{"name": "Last"}),
	}, Edges: []domain.FlowEdge{execEdge("start-loop", "start", "out", "loop", "in"), dataEdge("source-items", "source", "value", "items", "value"), dataEdge("items-loop", "items", "value", "loop", "items"), execEdge("loop-store", "loop", "loop", "store", "in"), dataEdge("item-store", "loop", "item", "store", "value")}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 6 {
		t.Fatalf("NodeRuns = %d, want event, JSON source, cached list, two body runs, loop completion", len(result.NodeRuns))
	}
}

func TestBlueprintDoOnceAndBreakRespectControlFlow(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("first", "data:constant", map[string]any{"type": "number", "value": 0}),
		cfgNode("last", "data:constant", map[string]any{"type": "number", "value": 2}),
		cfgNode("loop", "flow:for_loop", nil),
		cfgNode("once", "flow:do_once", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Once", "message": "Only once"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-loop", "start", "out", "loop", "in"),
		dataEdge("first-loop", "first", "value", "loop", "first"),
		dataEdge("last-loop", "last", "value", "loop", "last"),
		execEdge("loop-once", "loop", "loop", "once", "in"),
		execEdge("once-notice", "once", "out", "notice", "in"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	notifications := 0
	for _, run := range result.NodeRuns {
		if run.NodeID == "notice" {
			notifications++
		}
	}
	if notifications != 1 {
		t.Fatalf("notification executions = %d, want one Do Once output", notifications)
	}
}

func TestBlueprintBreakStopsInnermostLoop(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("source", "data:constant", map[string]any{"type": "text", "value": `["a","b","c"]`}),
		cfgNode("items", "data:cast", map[string]any{"target": "list"}),
		cfgNode("loop", "flow:for_each", nil),
		cfgNode("break", "flow:break", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Complete", "message": "Loop stopped"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-loop", "start", "out", "loop", "in"),
		dataEdge("source-items", "source", "value", "items", "value"),
		dataEdge("items-loop", "items", "value", "loop", "items"),
		execEdge("loop-break", "loop", "loop", "break", "in"),
		execEdge("loop-completed", "loop", "completed", "notice", "in"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	breaks, notifications := 0, 0
	for _, run := range result.NodeRuns {
		if run.NodeID == "break" {
			breaks++
		}
		if run.NodeID == "notice" {
			notifications++
		}
	}
	if breaks != 1 || notifications != 1 {
		t.Fatalf("breaks = %d, notifications = %d; want one each", breaks, notifications)
	}
}

func TestDateNodesInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("now", "date:now", map[string]any{"timezone": "utc"}),
		cfgNode("extract", "date:extract", map[string]any{"timezone": "utc"}),
		cfgNode("threshold", "data:constant", map[string]any{"type": "number", "value": -1.0}),
		cfgNode("isWeekday", "data:greater_than", nil),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("notice", "action:notification", map[string]any{"title": "Date Test", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"),
		dataEdge("now-extract-ts", "now", "timestamp", "extract", "timestamp"),
		dataEdge("extract-gt-left", "extract", "weekday", "isWeekday", "left"),
		dataEdge("threshold-gt-right", "threshold", "value", "isWeekday", "right"),
		dataEdge("gt-branch-cond", "isWeekday", "value", "branch", "condition"),
		execEdge("branch-notice", "branch", "true", "notice", "in"),
	}}

	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var extractRun *domain.NodeRun
	for _, run := range result.NodeRuns {
		if run.NodeID == "extract" {
			extractRun = &run
			break
		}
	}
	if extractRun == nil {
		t.Fatal("extract node was not evaluated")
	}

	outputs, ok := extractRun.Output.(map[string]any)
	if !ok {
		t.Fatalf("extract output is not a map: %#v", extractRun.Output)
	}

	year := outputs["year"].(float64)
	if year < 2024 || year > 2030 {
		t.Errorf("year = %v, want reasonable current year", year)
	}
}

func TestDateCreateAndFormatInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("create", "date:create", map[string]any{"year": 2024.0, "month": 6.0, "day": 15.0, "hour": 14.0, "minute": 30.0, "second": 45.0, "timezone": "utc"}),
		cfgNode("format", "date:format", map[string]any{"format": "2006-01-02 15:04:05", "timezone": "utc"}),
		cfgNode("notice", "action:notification", map[string]any{"title": "Date Test", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-notice", "start", "out", "notice", "in"),
		dataEdge("create-format-ts", "create", "timestamp", "format", "timestamp"),
		dataEdge("format-notice-msg", "format", "text", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	_, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(sender.calls) != 1 || sender.calls[0].message != "2024-06-15 14:30:45" {
		t.Fatalf("notification message = %#v, want formatted date", sender.calls)
	}
}

func TestDateParseInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:chat", map[string]any{"label": "Run"}),
		cfgNode("parse", "date:parse", map[string]any{"timezone": "utc"}),
		cfgNode("extract", "date:extract", map[string]any{"timezone": "utc"}),
		cfgNode("notice", "action:notification", map[string]any{"title": "Parse Test", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-notice", "start", "out", "notice", "in"),
		dataEdge("start-parse-text", "start", "text", "parse", "text"),
		dataEdge("parse-extract-ts", "parse", "timestamp", "extract", "timestamp"),
		dataEdge("extract-notice-msg", "extract", "iso", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	_, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{"text": "2024-06-15T14:30:45Z"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(sender.calls) != 1 || !strings.Contains(sender.calls[0].message, "2024-06-15") {
		t.Fatalf("notification message = %#v, want parsed date", sender.calls)
	}
}

func TestDateCompareWithBranchInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("now", "date:now", map[string]any{"timezone": "utc"}),
		cfgNode("create", "date:create", map[string]any{"year": 2020.0, "month": 1.0, "day": 1.0, "timezone": "utc"}),
		cfgNode("compare", "date:compare", nil),
		cfgNode("branch", "flow:branch", nil),
		cfgNode("noticeBefore", "action:notification", map[string]any{"title": "Compare", "message": "Before"}),
		cfgNode("noticeAfter", "action:notification", map[string]any{"title": "Compare", "message": "After"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"),
		dataEdge("now-compare-left", "now", "timestamp", "compare", "left"),
		dataEdge("create-compare-right", "create", "timestamp", "compare", "right"),
		dataEdge("compare-branch-cond", "compare", "after", "branch", "condition"),
		execEdge("branch-before", "branch", "true", "noticeAfter", "in"),
		execEdge("branch-after", "branch", "false", "noticeBefore", "in"),
	}}

	sender := &recordingNotificationSender{}
	_, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(sender.calls) != 1 || sender.calls[0].message != "After" {
		t.Fatalf("notification message = %#v, want After (now is after 2020)", sender.calls)
	}
}

func TestDateAddSubtractInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("create", "date:create", map[string]any{"year": 2024.0, "month": 6.0, "day": 15.0, "timezone": "utc"}),
		cfgNode("add", "date:add", map[string]any{"days": 10.0, "timezone": "utc"}),
		cfgNode("subtract", "date:subtract", map[string]any{"days": 5.0, "timezone": "utc"}),
		cfgNode("extractAdd", "date:extract", map[string]any{"timezone": "utc"}),
		cfgNode("extractSub", "date:extract", map[string]any{"timezone": "utc"}),
		cfgNode("castDay", "data:cast", map[string]any{"target": "text"}),
		cfgNode("notice", "action:notification", map[string]any{"title": "Math Test", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-notice", "start", "out", "notice", "in"),
		dataEdge("create-add-ts", "create", "timestamp", "add", "timestamp"),
		dataEdge("add-subtract-ts", "add", "timestamp", "subtract", "timestamp"),
		dataEdge("add-extract-ts", "add", "timestamp", "extractAdd", "timestamp"),
		dataEdge("subtract-extract-ts", "subtract", "timestamp", "extractSub", "timestamp"),
		dataEdge("extract-cast-msg", "extractSub", "day", "castDay", "value"),
		dataEdge("cast-notice-msg", "castDay", "value", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	_, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	day := sender.calls[0].message
	if day != "20" {
		t.Fatalf("final day = %q, want 20 (15 + 10 - 5)", day)
	}
}

func TestDateToUnixInBlueprint(t *testing.T) {
	t.Parallel()
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV3, Nodes: []domain.FlowNode{
		cfgNode("start", "trigger:button", map[string]any{"label": "Run"}),
		cfgNode("create", "date:create", map[string]any{"year": 2024.0, "month": 6.0, "day": 15.0, "hour": 14.0, "minute": 30.0, "second": 45.0, "millisecond": 123.0, "timezone": "utc"}),
		cfgNode("toUnix", "date:to_unix", nil),
		cfgNode("toUnixMs", "date:to_unix_ms", nil),
		cfgNode("castValue", "data:cast", map[string]any{"target": "text"}),
		cfgNode("notice", "action:notification", map[string]any{"title": "Unix Test", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-notice", "start", "out", "notice", "in"),
		dataEdge("create-toUnix-ts", "create", "timestamp", "toUnix", "timestamp"),
		dataEdge("create-toUnixMs-ts", "create", "timestamp", "toUnixMs", "timestamp"),
		dataEdge("toUnixMs-cast-msg", "toUnixMs", "value", "castValue", "value"),
		dataEdge("cast-notice-msg", "castValue", "value", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	_, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	val := sender.calls[0].message
	msVal := parseFloat(t, val)
	secVal := msVal / 1000
	if secVal < 1718461845 || secVal > 1718461846 {
		t.Fatalf("unix seconds = %v, want ~1718461845", secVal)
	}
}

func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		t.Fatalf("parse float %q: %v", s, err)
	}
	return f
}
