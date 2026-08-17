// Package app contains the sole Wails-facing façade for Neuropipe. This file
// owns the LLM-powered code generation feature for the JavaScript and SQL
// editors. The model receives a rich context describing the editor state,
// runtime capabilities, and the user's instruction, and returns the complete
// replacement code.
package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// CodeGenerationRequest carries everything the LLM needs to generate or edit
// code in one of the Blueprint editors. The frontend constructs it from the
// editor's current state and the user's natural-language instruction.
type CodeGenerationRequest struct {
	// EditorType is "javascript" or "sql".
	EditorType string `json:"editorType"`
	// Prompt is the user's natural-language instruction.
	Prompt string `json:"prompt"`
	// CurrentCode is the code currently in the editor (may be empty for
	// generation from scratch).
	CurrentCode string `json:"currentCode"`
	// JSContext is populated when EditorType == "javascript".
	JSContext *JSCodeContext `json:"jsContext,omitempty"`
	// SQLContext is populated when EditorType == "sql".
	SQLContext *SQLCodeContext `json:"sqlContext,omitempty"`
}

// JSCodeContext describes the JavaScript node's pin contract and capabilities
// so the LLM can write code that matches the runtime.
type JSCodeContext struct {
	// Inputs is a list of {id, type} pairs for the node's input pins.
	Inputs []JSPinSpec `json:"inputs"`
	// Outputs is a list of {id, type} pairs for the node's output pins.
	Outputs []JSPinSpec `json:"outputs"`
	// Capabilities lists the enabled capabilities (e.g. "file-read",
	// "network").
	Capabilities []string `json:"capabilities"`
}

// JSPinSpec is one input or output pin on a JavaScript node.
type JSPinSpec struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// SQLCodeContext describes the SQL node's database schema and parameters.
type SQLCodeContext struct {
	// DatabaseName is the human-readable name of the selected database.
	DatabaseName string `json:"databaseName"`
	// Schema is the database schema (tables, columns, indexes).
	Schema *domain.DatabaseSchema `json:"schema,omitempty"`
	// Parameters is a list of {name, type} pairs for the node's SQL
	// parameters.
	Parameters []SQLParameterSpec `json:"parameters"`
}

// SQLParameterSpec is one SQL parameter.
type SQLParameterSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// CodeGenerationResponse is the result returned to the frontend.
type CodeGenerationResponse struct {
	Code string `json:"code"`
}

// GenerateCode uses the configured LLM provider to generate or edit code in
// one of the Blueprint editors. The editor state (current code, schema,
// parameters, pin contract) is passed as context so the model can produce
// code that matches the runtime's expectations.
func (d *Desktop) GenerateCode(request CodeGenerationRequest) (CodeGenerationResponse, error) {
	if d.providers == nil {
		return CodeGenerationResponse{}, fmt.Errorf("no LLM provider is configured")
	}
	prompt := buildCodeGenerationPrompt(request)
	// The LLM manager's Chat method uses its own per-request timeout (15min
	// via postJSON). We pass d.context() so the call inherits the app
	// lifecycle context.
	response, err := d.providers.Chat(d.context(), pipeline.ChatRequest{
		Prompt:  prompt,
		Metrics: domain.LLMMetricContext{Origin: "code-generator", NodeType: "editor:" + request.EditorType},
	})
	if err != nil {
		return CodeGenerationResponse{}, fmt.Errorf("LLM code generation: %w", err)
	}
	code := strings.TrimSpace(response.Content)
	// Strip markdown code fences if the model wrapped the output.
	code = stripCodeFences(code)
	return CodeGenerationResponse{Code: code}, nil
}

// buildCodeGenerationPrompt constructs the full prompt sent to the LLM. It
// includes runtime context (JS interpreter capabilities, np API, SQL schema,
// parameters), the current code, and the user's instruction.
func buildCodeGenerationPrompt(request CodeGenerationRequest) string {
	var builder strings.Builder
	builder.WriteString("You are a code generation assistant for Neuropipe, a local automation platform.\n")
	builder.WriteString("Return ONLY the code. Do not wrap it in markdown code fences. Do not add explanations.\n\n")

	switch request.EditorType {
	case "javascript":
		writeJavaScriptContext(&builder, request)
	case "sql":
		writeSQLContext(&builder, request)
	}

	if strings.TrimSpace(request.CurrentCode) != "" {
		builder.WriteString("\nCurrent code:\n```\n")
		builder.WriteString(request.CurrentCode)
		builder.WriteString("\n```\n")
	}

	builder.WriteString("\nInstruction: ")
	builder.WriteString(request.Prompt)
	builder.WriteString("\n\nReturn ONLY the updated code. No markdown fences, no explanations, no commentary.")
	return builder.String()
}

func writeJavaScriptContext(builder *strings.Builder, request CodeGenerationRequest) {
	builder.WriteString("## JavaScript Node Context\n\n")
	builder.WriteString("You are writing code for a Neuropipe JavaScript node.\n")
	builder.WriteString("Runtime: goja (pure-Go ES5.1 interpreter). Strict mode is enforced.\n")
	builder.WriteString("The code must be a function body that returns an object.\n")
	builder.WriteString("Use `return { ... }` to return values.\n\n")

	builder.WriteString("### Available globals\n")
	builder.WriteString("- `inputs`: object containing all input pin values (also available as bare identifiers)\n")
	builder.WriteString("- `np`: host API object with the following methods:\n")
	builder.WriteString("  - `np.context`: { pipelineId, executionId, nodeId }\n")
	builder.WriteString("  - `np.uuid()`: returns a UUID v4 string\n")
	builder.WriteString("  - `np.assert(condition, message?)`: throws if condition is falsy\n")
	builder.WriteString("  - `np.fail(message?)`: always throws\n")
	builder.WriteString("  - `np.variables.get(name)`, `.has(name)`, `.set(name, value)`, `.delete(name)`: per-execution variables\n")
	builder.WriteString("  - `np.base64.encodeText(text)`, `.decodeText(b64)`, `.encodeBytes(bytes)`, `.decodeBytes(b64)`\n")
	builder.WriteString("  - `np.hash.sha256(textOrBytes)`: returns hex digest\n")
	builder.WriteString("  - `np.getPipelines()`, `np.pipelines.list()`, `np.pipelines.get(id)`: list/get published pipelines\n")
	builder.WriteString("  - `np.functions.list()`: list published functions\n")
	builder.WriteString("  - `np.triggers.list()`: list trigger bindings\n")
	builder.WriteString("  - `np.executions.list(limit?)`: list recent executions (default 20, max 100)\n")
	builder.WriteString("  - `np.reports.list(limit?)`, `.get(id)`, `.create({ title, markdown, tags? })`: manage reports\n")
	builder.WriteString("  - `np.chat.history(chatId, limit?)`, `.reply(chatRunId, text)`, `.setStatus(chatRunId, status)`: chat operations\n")
	builder.WriteString("  - `np.files.list(path)`, `.readBytes(path)`, `.readText(path)`, `.writeBytes(path, bytes)`, `.writeText(path, text)`: file I/O (requires file-read/file-write capability)\n")
	builder.WriteString("  - `np.http.request({ url, method?, headers?, body? })`: HTTP request (requires network capability)\n")
	builder.WriteString("  - `np.notify(title, message)`: show a desktop notification\n\n")

	if request.JSContext != nil {
		if len(request.JSContext.Inputs) > 0 {
			builder.WriteString("### Input pins\n")
			for _, pin := range request.JSContext.Inputs {
				builder.WriteString(fmt.Sprintf("- `%s` (type: %s) — available as `inputs.%s` or bare `%s`\n", pin.ID, pin.Type, pin.ID, pin.ID))
			}
			builder.WriteString("\n")
		}
		if len(request.JSContext.Outputs) > 0 {
			builder.WriteString("### Output pins\n")
			builder.WriteString("The returned object MUST have one property for each output pin:\n")
			for _, pin := range request.JSContext.Outputs {
				builder.WriteString(fmt.Sprintf("- `%s` (type: %s)\n", pin.ID, pin.Type))
			}
			builder.WriteString("\n")
		}
		if len(request.JSContext.Capabilities) > 0 {
			builder.WriteString("### Enabled capabilities\n")
			builder.WriteString(strings.Join(request.JSContext.Capabilities, ", "))
			builder.WriteString("\n\n")
		}
	}

	builder.WriteString("### Rules\n")
	builder.WriteString("- Code must be ES5.1 compatible (no arrow functions, no const/let, no template literals, no async/await)\n")
	builder.WriteString("- Use `var` for variable declarations\n")
	builder.WriteString("- The code runs synchronously with a 5-second timeout\n")
	builder.WriteString("- Maximum script size: 64 KB\n")
	builder.WriteString("- Return an object with properties matching the output pin IDs\n\n")
}

func writeSQLContext(builder *strings.Builder, request CodeGenerationRequest) {
	builder.WriteString("## SQL Node Context\n\n")
	builder.WriteString("You are writing SQL for a Neuropipe SQL node.\n")
	builder.WriteString("Database: SQLite (local file-based).\n")
	builder.WriteString("Only one statement per node is allowed (no multi-statement scripts).\n\n")

	if request.SQLContext != nil {
		if request.SQLContext.DatabaseName != "" {
			builder.WriteString("Database: ")
			builder.WriteString(request.SQLContext.DatabaseName)
			builder.WriteString("\n\n")
		}
		if request.SQLContext.Schema != nil {
			builder.WriteString("### Database schema\n")
			schemaJSON, err := json.MarshalIndent(request.SQLContext.Schema, "", "  ")
			if err == nil {
				builder.WriteString("```json\n")
				builder.WriteString(string(schemaJSON))
				builder.WriteString("\n```\n\n")
			}
		}
		if len(request.SQLContext.Parameters) > 0 {
			builder.WriteString("### Parameters\n")
			builder.WriteString("Use named parameters with the `:name` syntax. Available parameters:\n")
			for _, param := range request.SQLContext.Parameters {
				builder.WriteString(fmt.Sprintf("- `:%s` (type: %s)\n", param.Name, param.Type))
			}
			builder.WriteString("\nExample: `SELECT * FROM users WHERE id = :userId`\n\n")
		}
	}

	builder.WriteString("### Rules\n")
	builder.WriteString("- Use `:parameterName` syntax for parameters (not `?` or `$1`)\n")
	builder.WriteString("- Only one SQL statement\n")
	builder.WriteString("- Maximum 500 rows returned\n")
	builder.WriteString("- Output pins: columns, rows, rowsAffected, lastInsertId, truncated\n\n")
}

// stripCodeFences removes markdown code fences (```language ... ```) if the
// model wrapped its output despite being told not to.
func stripCodeFences(code string) string {
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(code, "```") {
		return code
	}
	// Remove opening fence (possibly with language tag)
	lines := strings.SplitN(code, "\n", 2)
	if len(lines) < 2 {
		return code
	}
	rest := lines[1]
	// Remove closing fence
	rest = strings.TrimSpace(rest)
	if strings.HasSuffix(rest, "```") {
		rest = strings.TrimSpace(strings.TrimSuffix(rest, "```"))
	}
	return rest
}
