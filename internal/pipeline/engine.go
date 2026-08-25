package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/htmlutil"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

const maxNodeVisits = 10_000

// CapabilityGate authorizes sensitive node execution for a concrete revision.
type CapabilityGate interface {
	Allow(ctx context.Context, node domain.FlowNode, capabilities []domain.Capability) error
}

// GlobalVariablesStore exposes workspace-scoped variables to running graphs.
// It deliberately exposes only the narrow operations nodes rely on; catalogue
// management remains with the composition root.
type GlobalVariablesStore interface {
	Read(name string) (any, error)
	Set(name string, value any) error
	Increment(name string, delta float64) (float64, error)
	Append(name string, item any) ([]any, error)
}

// Engine executes a validated graph without knowing where definitions are stored.
type Engine struct {
	registry      *catalog.Registry
	llm           LLMRunner
	gate          CapabilityGate
	http          *http.Client
	reports       ReportWriter
	reportContext ReportContext
	functions     FunctionResolver
	notifications NotificationSender
	chat          ChatWriter
	javascript    nodes.JavaScriptHost
	twitch        nodes.TwitchChatSender
	globals       GlobalVariablesStore
	databases     nodes.SQLExecutor
	dialogs       nodes.DialogOpener
	inputDialogs  nodes.InputDialogOpener
	formDialogs   nodes.FormDialogOpener
	variables     Packet
}

// NewEngine builds an execution engine with explicit dependencies.
func NewEngine(registry *catalog.Registry, llm LLMRunner, gate CapabilityGate, options ...EngineOption) *Engine {
	engine := &Engine{
		registry: registry,
		llm:      llm,
		gate:     gate,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, option := range options {
		option(engine)
	}
	return engine
}

func (e *Engine) executeNode(ctx context.Context, node domain.FlowNode, input Packet) (Result, error) {
	config := configFor(node)
	switch node.Type {
	case "trigger:button", "trigger:cron", "trigger:file_watch", "trigger:hotkey", "trigger:webhook", "trigger:chat":
		return Result{"out": {clonePacket(input)}}, nil
	case "action:http":
		return e.executeHTTP(ctx, config, input)
	case "action:terminal":
		return executeTerminal(ctx, config, input)
	case "action:notification":
		return e.executeNotification(ctx, config, input)
	case "action:report":
		return e.executeReport(ctx, node.ID, config, input)
	case "action:chat_reply":
		return e.executeChatReply(ctx, config, input)
	case "action:chat_status":
		return e.executeChatStatus(ctx, config, input)
	case "action:git":
		return executeGit(ctx, config, input)
	case "action:subpipeline":
		return nil, fmt.Errorf("sub-pipeline execution is not available in this release")
	case "llm:prompt", "llm:extract", "llm:summarize", "llm:agent", "llm:coding_agent":
		return e.executeLLM(ctx, node, config, input)
	case "llm:boolean":
		return e.executeBoolean(ctx, node, config, input)
	case "llm:choice":
		return e.executeChoice(ctx, node, config, input)
	default:
		return nil, fmt.Errorf("node type %q does not have an executor", node.Type)
	}
}

func (e *Engine) executeChatReply(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	if e.chat == nil {
		return nil, fmt.Errorf("chat delivery is unavailable for this execution")
	}
	runID := strings.TrimSpace(text(config, "chatRunId"))
	if runID == "" {
		return nil, fmt.Errorf("chat run ID is required")
	}
	content := strings.TrimSpace(text(config, "text"))
	if content == "" {
		return nil, fmt.Errorf("chat reply text is required")
	}
	message, err := e.chat.AppendChatReply(ctx, runID, content)
	if err != nil {
		return nil, fmt.Errorf("send chat reply: %w", err)
	}
	return Result{"out": {mergePacket(input, Packet{"chat": map[string]any{"messageId": message.ID, "chatRunId": runID}})}}, nil
}

func (e *Engine) executeChatStatus(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	if e.chat == nil {
		return nil, fmt.Errorf("chat delivery is unavailable for this execution")
	}
	runID := strings.TrimSpace(text(config, "chatRunId"))
	if runID == "" {
		return nil, fmt.Errorf("chat run ID is required")
	}
	status := strings.TrimSpace(text(config, "status"))
	if status == "" {
		status = "Working"
	}
	if err := e.chat.UpdateChatStatus(ctx, runID, status); err != nil {
		return nil, fmt.Errorf("update chat status: %w", err)
	}
	return Result{"out": {mergePacket(input, Packet{"chat": map[string]any{"chatRunId": runID, "status": status}})}}, nil
}

func (e *Engine) executeNotification(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	title := strings.TrimSpace(text(config, "title"))
	if title == "" {
		title = "Neuropipe"
	}
	message := strings.TrimSpace(text(config, "message"))
	if message == "" {
		message = "Pipeline completed."
	}
	if e.notifications != nil {
		if err := e.notifications.Send(ctx, title, message); err != nil {
			return nil, fmt.Errorf("send desktop notification: %w", err)
		}
	}
	return Result{"out": {Packet{"notification": map[string]any{"title": title, "message": message}}}}, nil
}

func (e *Engine) executeReport(ctx context.Context, nodeID string, config map[string]any, input Packet) (Result, error) {
	if e.reports == nil || e.reportContext.PipelineID == "" || e.reportContext.ExecutionID == "" {
		return nil, fmt.Errorf("report storage is unavailable for this execution")
	}
	report, err := e.reports.CreateReport(ctx, domain.Report{
		PipelineID:  e.reportContext.PipelineID,
		ExecutionID: e.reportContext.ExecutionID,
		NodeID:      nodeID,
		Title:       text(config, "title"),
		Tags:        domain.ParseTags(text(config, "tags")),
		Markdown:    text(config, "markdown"),
	})
	if err != nil {
		return nil, err
	}
	return Result{"out": {mergePacket(input, Packet{"report": map[string]any{"id": report.ID, "title": report.Title, "createdAt": report.CreatedAt.Format(time.RFC3339)}})}}, nil
}

func (e *Engine) executeHTTP(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	url := text(config, "url")
	if url == "" {
		return nil, fmt.Errorf("HTTP URL is required")
	}
	body := text(config, "body")
	request, err := http.NewRequestWithContext(ctx, strings.ToUpper(defaultText(text(config, "method"), "GET")), url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	for _, header := range configuredHTTPHeaders(config["headers"]) {
		request.Header.Add(header.Name, header.Value)
	}
	if body != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if boolValue(config["useCustomUserAgent"]) {
		userAgent := text(config, "userAgent")
		if userAgent == "" {
			return nil, fmt.Errorf("custom User-Agent is required when enabled")
		}
		request.Header.Set("User-Agent", userAgent)
	}
	response, err := e.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send HTTP request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}
	responseBody := string(data)
	stripScripts := boolValue(config["stripScripts"])
	stripStyles := boolValue(config["stripStyles"])
	if (stripScripts || stripStyles) && isHTMLResponse(response.Header.Get("Content-Type"), responseBody) {
		responseBody, err = htmlutil.Clean(responseBody, stripScripts, stripStyles)
		if err != nil {
			return nil, fmt.Errorf("clean HTML response: %w", err)
		}
		data = []byte(responseBody)
	}
	output := Packet{"status": response.StatusCode, "body": responseBody, "headers": response.Header}
	var decoded any
	if json.Unmarshal(data, &decoded) == nil {
		output["json"] = decoded
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("HTTP request returned %s", response.Status)
	}
	return Result{"out": {mergePacket(input, output)}}, nil
}

type httpHeader struct {
	Name  string
	Value string
}

// isHTMLResponse reports whether cleaning toggles may treat the payload as
// HTML: an HTML content type wins, and markup-sniffing covers servers that
// answer without a content type.
func isHTMLResponse(contentType, body string) bool {
	if contentType != "" {
		return strings.Contains(strings.ToLower(contentType), "html")
	}
	trimmed := strings.ToLower(strings.TrimSpace(body))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func configuredHTTPHeaders(value any) []httpHeader {
	var headers []httpHeader
	appendHeader := func(name, value string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		headers = append(headers, httpHeader{Name: name, Value: strings.TrimSpace(value)})
	}
	appendEntry := func(entry map[string]any) {
		name, _ := entry["name"].(string)
		if name == "" {
			name, _ = entry["key"].(string)
		}
		value, _ := entry["value"].(string)
		appendHeader(name, value)
	}

	switch raw := value.(type) {
	case []any:
		for _, item := range raw {
			if entry, ok := item.(map[string]any); ok {
				appendEntry(entry)
			}
		}
	case []map[string]any:
		for _, entry := range raw {
			appendEntry(entry)
		}
	case map[string]any:
		for name, value := range raw {
			if text, ok := value.(string); ok {
				appendHeader(name, text)
			}
		}
	case map[string]string:
		for name, value := range raw {
			appendHeader(name, value)
		}
	}
	return headers
}

func boolValue(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func executeTerminal(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	command := text(config, "command")
	if command == "" {
		return nil, fmt.Errorf("terminal command is required")
	}
	shell := text(config, "shell")
	var executable string
	var args []string
	switch shell {
	case "cmd":
		executable, args = "cmd.exe", []string{"/C", command}
	case "Windows PowerShell":
		executable, args = "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", command}
	default:
		executable, args = "pwsh.exe", []string{"-NoProfile", "-NonInteractive", "-Command", command}
	}
	process := exec.CommandContext(ctx, executable, args...)
	if directory := text(config, "workingDirectory"); directory != "" {
		process.Dir = filepath.Clean(directory)
	}
	output, err := process.CombinedOutput()
	result := Packet{"terminal": map[string]any{"command": command, "output": string(output)}}
	if err != nil {
		return nil, fmt.Errorf("run terminal command: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return Result{"out": {mergePacket(input, result)}}, nil
}

func executeGit(ctx context.Context, config map[string]any, input Packet) (Result, error) {
	operation := defaultText(text(config, "operation"), "status")
	repository := text(config, "repository")
	process := exec.CommandContext(ctx, "git.exe", operation)
	process.Dir = filepath.Clean(repository)
	output, err := process.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run git %s: %w: %s", operation, err, strings.TrimSpace(string(output)))
	}
	return Result{"out": {mergePacket(input, Packet{"git": map[string]any{"operation": operation, "output": string(output)}})}}, nil
}

func (e *Engine) executeLLM(ctx context.Context, node domain.FlowNode, config map[string]any, input Packet) (Result, error) {
	if e.llm == nil {
		return nil, fmt.Errorf("configure an LLM provider in Settings before running AI nodes")
	}
	prompt := text(config, "prompt")
	if node.Type == "llm:summarize" || node.Type == "llm:agent" {
		prompt = text(config, "instructions")
	}
	if node.Type == "llm:coding_agent" {
		prompt = "Coding task:\n" + text(config, "task") + "\n\nWorkspace: " + text(config, "workspace")
	}
	if node.Type == "llm:agent" || node.Type == "llm:coding_agent" {
		if history, err := e.agentHistory(ctx, node, config); err != nil {
			return nil, err
		} else if history != nil {
			return e.converseAgent(ctx, node, config, prompt, history, input)
		}
	}
	status, err := e.chatStatusReporter(ctx, node, config)
	if err != nil {
		return nil, err
	}
	if err := reportModelStatus(status, chatStatusThinking); err != nil {
		return nil, err
	}
	request := ChatRequest{Prompt: promptWithInput(prompt, input), Model: text(config, "model"), Metrics: e.llmMetricContext(node)}
	if node.Type == "llm:extract" {
		request.Schema = jsonObject(config["schema"])
	}
	response, err := e.llm.Chat(ctx, request)
	if err != nil {
		return nil, err
	}
	output := Packet{"llm": map[string]any{"content": response.Content}}
	if len(response.JSON) > 0 {
		output["llm"].(map[string]any)["json"] = response.JSON
	}
	return Result{"out": {mergePacket(input, output)}}, nil
}

// converseAgent runs an agent without connected tools as a multi-turn chat:
// the system prompt (instructions or coding task) followed by the loaded
// conversation, whose final user message the agent answers directly.
func (e *Engine) converseAgent(ctx context.Context, node domain.FlowNode, config map[string]any, prompt string, history []domain.ChatMessage, input Packet) (Result, error) {
	assistant, supported := e.llm.(AssistantRunner)
	if !supported {
		return nil, fmt.Errorf("the configured LLM provider does not support chat history")
	}
	status, err := e.chatStatusReporter(ctx, node, config)
	if err != nil {
		return nil, err
	}
	if err := reportModelStatus(status, chatStatusThinking); err != nil {
		return nil, err
	}
	response, err := assistant.Converse(ctx, domain.AssistantChatRequest{
		Messages: agentHistoryMessages(prompt, history),
		Model:    text(config, "model"),
		Metrics:  e.llmMetricContext(node),
	})
	if err != nil {
		return nil, err
	}
	return Result{"out": {mergePacket(input, Packet{"llm": map[string]any{"content": response.Content}})}}, nil
}

func (e *Engine) executeBoolean(ctx context.Context, node domain.FlowNode, config map[string]any, input Packet) (Result, error) {
	if e.llm == nil {
		return nil, fmt.Errorf("configure an LLM provider in Settings before running AI nodes")
	}
	status, err := e.chatStatusReporter(ctx, node, config)
	if err != nil {
		return nil, err
	}
	if err := reportModelStatus(status, chatStatusThinking); err != nil {
		return nil, err
	}
	response, err := e.llm.Chat(ctx, ChatRequest{Prompt: promptWithInput(text(config, "prompt"), input), ToolName: "route", ToolChoices: []string{"true", "false"}, Metrics: e.llmMetricContext(node)})
	if err != nil {
		return nil, err
	}
	decision := strings.ToLower(strings.TrimSpace(fmt.Sprint(response.JSON["decision"])))
	if decision != "true" && decision != "false" {
		return nil, fmt.Errorf("model did not return a valid true/false route")
	}
	return Result{decision: {mergePacket(input, Packet{"llm": map[string]any{"decision": decision, "content": response.Content}})}}, nil
}

func (e *Engine) executeChoice(ctx context.Context, node domain.FlowNode, config map[string]any, input Packet) (Result, error) {
	if e.llm == nil {
		return nil, fmt.Errorf("configure an LLM provider in Settings before running AI nodes")
	}
	status, err := e.chatStatusReporter(ctx, node, config)
	if err != nil {
		return nil, err
	}
	if err := reportModelStatus(status, chatStatusThinking); err != nil {
		return nil, err
	}
	optionsConfig := config["options"]
	options := choiceIDs(optionsConfig)
	if len(options) < 2 {
		return nil, fmt.Errorf("LLM Choice Router needs at least two options")
	}
	response, err := e.llm.Chat(ctx, ChatRequest{
		Prompt:                 promptWithInput(text(config, "prompt"), input),
		ToolName:               "choose",
		ToolChoices:            options,
		ToolChoiceDescriptions: choiceGuidance(optionsConfig),
		Metrics:                e.llmMetricContext(node),
	})
	if err != nil {
		return nil, err
	}
	choice := fmt.Sprint(response.JSON["choice"])
	for _, option := range options {
		if choice == option {
			return Result{choice: {mergePacket(input, Packet{"llm": map[string]any{"choice": choice, "content": response.Content}})}}, nil
		}
	}
	return nil, fmt.Errorf("model selected an unknown option")
}

func (e *Engine) llmMetricContext(node domain.FlowNode) domain.LLMMetricContext {
	return domain.LLMMetricContext{ExecutionID: e.reportContext.ExecutionID, PipelineID: e.reportContext.PipelineID, NodeID: node.ID, NodeType: node.Type, Origin: "pipeline"}
}

func clonePacket(input Packet) Packet {
	copy := make(Packet, len(input))
	for key, value := range input {
		copy[key] = value
	}
	return copy
}

func mergePacket(input, output Packet) Packet {
	merged := clonePacket(input)
	for key, value := range output {
		merged[key] = value
	}
	return merged
}

func promptWithInput(prompt string, input Packet) string {
	value, exists := input["input"]
	if !exists {
		return prompt
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return prompt + "\n\nInput:\n" + fmt.Sprint(value)
	}
	return prompt + "\n\nInput:\n" + string(encoded)
}

func jsonObject(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	if textValue, ok := value.(string); ok {
		var object map[string]any
		if json.Unmarshal([]byte(textValue), &object) == nil {
			return object
		}
	}
	return nil
}

func choiceIDs(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	choices := make([]string, 0, len(raw))
	for _, item := range raw {
		if option, ok := item.(map[string]any); ok {
			if id := strings.TrimSpace(fmt.Sprint(option["id"])); id != "" {
				choices = append(choices, id)
			}
		}
	}
	return choices
}

func choiceGuidance(value any) map[string]string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	guidance := make(map[string]string, len(raw))
	for _, item := range raw {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(option["id"]))
		if id == "" {
			continue
		}
		parts := make([]string, 0, 2)
		if label, ok := option["label"].(string); ok {
			label = strings.TrimSpace(label)
			if label != "" && label != id {
				parts = append(parts, "Display name: "+label)
			}
		}
		if description, ok := option["description"].(string); ok {
			description = strings.TrimSpace(description)
			if description != "" {
				parts = append(parts, description)
			}
		}
		if len(parts) > 0 {
			guidance[id] = strings.Join(parts, " ")
		}
	}
	if len(guidance) == 0 {
		return nil
	}
	return guidance
}

func defaultText(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
