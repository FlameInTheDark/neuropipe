package pipeline

import (
	"context"
	"errors"
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

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("request", "action:http", map[string]any{
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

func TestHTTPResultCanFeedBreakObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("Blueprint ready"))
	}))
	defer server.Close()

	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("request", "action:http", map[string]any{"url": server.URL, "method": "GET"}),
		v2Node("break", "data:break_object", map[string]any{"outputs": []any{
			map[string]any{"id": "body", "label": "Body", "path": "body", "dataType": "text"},
			map[string]any{"id": "headers", "label": "Headers", "path": "headers", "dataType": "object"},
		}}),
		v2Node("notice", "action:notification", map[string]any{"title": "HTTP"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-request", "start", "out", "request", "in"),
		execEdge("request-notice", "request", "out", "notice", "in"),
		dataEdge("request-break", "request", "result", "break", "source"),
		dataEdge("break-notice", "break", "body", "notice", "message"),
	}}

	sender := &recordingNotificationSender{}
	if _, err := NewEngine(catalog.New(), nil, nil, WithNotificationSender(sender)).Execute(context.Background(), flow, "start", Packet{}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := sender.calls, []notificationCall{{title: "HTTP", message: "Blueprint ready"}}; !reflect.DeepEqual(got, want) {
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

func v2Node(id, nodeType string, config map[string]any) domain.FlowNode {
	return domain.FlowNode{ID: id, Type: nodeType, Data: map[string]any{"config": config}}
}
func execEdge(id, source, sourcePin, target, targetPin string) domain.FlowEdge {
	return domain.FlowEdge{ID: id, Source: source, SourceHandle: sourcePin, Target: target, TargetHandle: targetPin, Kind: domain.PinExec}
}
func dataEdge(id, source, sourcePin, target, targetPin string) domain.FlowEdge {
	return domain.FlowEdge{ID: id, Source: source, SourceHandle: sourcePin, Target: target, TargetHandle: targetPin, Kind: domain.PinData}
}

func TestBlueprintCachesPureDataAndRoutesExec(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("truth", "data:constant", map[string]any{"value": true}),
		v2Node("branch", "flow:branch", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
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

func TestChatTriggerPassesExplicitPinsToReplyNode(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("chat", "trigger:chat", map[string]any{"label": "Support"}),
		v2Node("reply", "action:chat_reply", nil),
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("source", "data:constant", map[string]any{"value": map[string]any{
			"terminal": map[string]any{"command": "Get-Date", "output": "Friday"},
			"ready":    true,
		}}),
		v2Node("fields", "data:get_field", map[string]any{"outputs": []any{
			map[string]any{"id": "command", "label": "Command", "path": "terminal.command", "dataType": "text"},
			map[string]any{"id": "ready", "label": "Ready", "path": "ready", "dataType": "boolean"},
		}}),
		v2Node("branch", "flow:branch", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-branch", "start", "out", "branch", "in"),
		dataEdge("source-fields", "source", "value", "fields", "source"),
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("name", "data:constant", map[string]any{"value": "Ada"}),
		v2Node("active", "data:constant", map[string]any{"value": true}),
		v2Node("build", "data:build_object", map[string]any{"fields": []any{
			map[string]any{"id": "name", "label": "Name", "key": "customer.name", "dataType": "text"},
			map[string]any{"id": "active", "label": "Active", "key": "customer.active", "dataType": "boolean"},
		}}),
		v2Node("break", "data:break_object", map[string]any{"outputs": []any{
			map[string]any{"id": "name", "label": "Name", "path": "customer.name", "dataType": "text"},
			map[string]any{"id": "active", "label": "Active", "path": "customer.active", "dataType": "boolean"},
		}}),
		v2Node("branch", "flow:branch", nil),
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
			flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
				v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
				v2Node("notice", "action:notification", map[string]any{"title": "Ready", "message": "Pipeline finished"}),
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("truth", "data:constant", map[string]any{"value": true}),
		v2Node("store", "flow:set_variable", map[string]any{"name": "Ready"}), v2Node("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("start-store", "start", "out", "store", "in"), dataEdge("truth-store", "truth", "value", "store", "value"), execEdge("store-branch", "store", "out", "branch", "in"), dataEdge("store-branch-data", "store", "result", "branch", "condition"),
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

func TestBlueprintReroutesPreserveExecutionAndData(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("exec-reroute", "flow:reroute", nil),
		v2Node("truth", "data:constant", map[string]any{"value": true}),
		v2Node("data-reroute", "data:reroute", nil),
		v2Node("branch", "flow:branch", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Ready", "message": "Done"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-reroute", "start", "out", "exec-reroute", "in"),
		execEdge("reroute-branch", "exec-reroute", "out", "branch", "in"),
		dataEdge("truth-reroute", "truth", "value", "data-reroute", "value"),
		dataEdge("reroute-condition", "data-reroute", "value", "branch", "condition"),
		execEdge("branch-notice", "branch", "true", "notice", "in"),
	}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 6 || result.NodeRuns[1].NodeID != "exec-reroute" || result.NodeRuns[3].NodeID != "data-reroute" || result.NodeRuns[5].NodeID != "notice" {
		t.Fatalf("unexpected reroute execution order: %#v", result.NodeRuns)
	}
}

func TestBlueprintDoesNotReuseVariablesAcrossRuns(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("write", "trigger:button", map[string]any{"label": "Write"}),
		v2Node("read", "trigger:button", map[string]any{"label": "Read"}),
		v2Node("value", "data:constant", map[string]any{"value": true}),
		v2Node("set", "flow:set_variable", map[string]any{"name": "RunLocal"}),
		v2Node("get", "data:get_variable", map[string]any{"name": "RunLocal"}),
		v2Node("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{
		execEdge("write-set", "write", "out", "set", "in"),
		dataEdge("value-set", "value", "value", "set", "value"),
		execEdge("read-branch", "read", "out", "branch", "in"),
		dataEdge("get-branch", "get", "value", "branch", "condition"),
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("unrun", "flow:set_variable", map[string]any{"name": "Never"}), v2Node("branch", "flow:branch", nil),
	}, Edges: []domain.FlowEdge{execEdge("start-branch", "start", "out", "branch", "in"), dataEdge("unrun-condition", "unrun", "result", "branch", "condition")}}
	_, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err == nil || !strings.Contains(err.Error(), "has an Exec pin but has not run") {
		t.Fatalf("Execute() error = %v, want unexecuted impure-data failure", err)
	}
}

func TestBlueprintForEachScopesIterationData(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}), v2Node("items", "data:constant", map[string]any{"value": []any{"a", "b"}}), v2Node("loop", "flow:for_each", nil), v2Node("store", "flow:set_variable", map[string]any{"name": "Last"}),
	}, Edges: []domain.FlowEdge{execEdge("start-loop", "start", "out", "loop", "in"), dataEdge("items-loop", "items", "value", "loop", "items"), execEdge("loop-store", "loop", "loop", "store", "in"), dataEdge("item-store", "loop", "item", "store", "value")}}
	result, err := NewEngine(catalog.New(), nil, nil).Execute(context.Background(), flow, "start", Packet{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.NodeRuns) != 5 {
		t.Fatalf("NodeRuns = %d, want event, cached array, two body runs, loop completion", len(result.NodeRuns))
	}
}

func TestBlueprintDoOnceAndBreakRespectControlFlow(t *testing.T) {
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("first", "data:constant", map[string]any{"value": 0}),
		v2Node("last", "data:constant", map[string]any{"value": 2}),
		v2Node("loop", "flow:for_loop", nil),
		v2Node("once", "flow:do_once", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Once", "message": "Only once"}),
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
	flow := domain.FlowDefinition{SchemaVersion: domain.GraphSchemaV2, Nodes: []domain.FlowNode{
		v2Node("start", "trigger:button", map[string]any{"label": "Run"}),
		v2Node("items", "data:constant", map[string]any{"value": []any{"a", "b", "c"}}),
		v2Node("loop", "flow:for_each", nil),
		v2Node("break", "flow:break", nil),
		v2Node("notice", "action:notification", map[string]any{"title": "Complete", "message": "Loop stopped"}),
	}, Edges: []domain.FlowEdge{
		execEdge("start-loop", "start", "out", "loop", "in"),
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
