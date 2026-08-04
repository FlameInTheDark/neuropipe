package catalog

import "github.com/FlameInTheDark/neuropipe/internal/domain"

// knownResultFields documents fields that a first-party action adds to its
// result packet. The input packet is deliberately not repeated: it continues
// through unchanged, while these fields are the dependable additions users can
// discover in the editor. Plugins can declare the same metadata on NodePort.
var knownResultFields = map[string][]domain.DataField{
	"action:http": {
		{Path: "status", Label: "Status", DataType: domain.DataNumber, Description: "HTTP status code."},
		{Path: "body", Label: "Body", DataType: domain.DataText, Description: "Response body as text."},
		{Path: "headers", Label: "Headers", DataType: domain.DataObject, Description: "Response headers."},
		{Path: "json", Label: "JSON", DataType: domain.DataAny, Description: "Parsed JSON response, when the body is JSON.", Optional: true},
	},
	"action:file_read": {
		{Path: "file.path", Label: "Path", DataType: domain.DataText, Description: "Read file path."},
		{Path: "file.content", Label: "Content", DataType: domain.DataText, Description: "File content as text."},
		{Path: "file.json", Label: "JSON", DataType: domain.DataAny, Description: "Parsed JSON content, when available.", Optional: true},
	},
	"action:file_write": {
		{Path: "file.path", Label: "Path", DataType: domain.DataText, Description: "Written file path."},
		{Path: "file.written", Label: "Written", DataType: domain.DataBoolean, Description: "Whether the write completed."},
	},
	"action:terminal": {
		{Path: "terminal.command", Label: "Command", DataType: domain.DataText, Description: "Command that was run."},
		{Path: "terminal.output", Label: "Output", DataType: domain.DataText, Description: "Combined standard output and error."},
	},
	"action:notification": {
		{Path: "notification.title", Label: "Title", DataType: domain.DataText, Description: "Displayed notification title."},
		{Path: "notification.message", Label: "Message", DataType: domain.DataText, Description: "Displayed notification message."},
	},
	"action:report": {
		{Path: "report.id", Label: "Report ID", DataType: domain.DataText, Description: "Created report identifier."},
		{Path: "report.title", Label: "Title", DataType: domain.DataText, Description: "Created report title."},
		{Path: "report.createdAt", Label: "Created at", DataType: domain.DataText, Description: "Creation time in RFC 3339 format."},
	},
	"action:git": {
		{Path: "git.operation", Label: "Operation", DataType: domain.DataText, Description: "Git operation that was run."},
		{Path: "git.output", Label: "Output", DataType: domain.DataText, Description: "Git command output."},
	},
	"llm:prompt":       llmResultFields(),
	"llm:extract":      llmResultFields(),
	"llm:summarize":    llmResultFields(),
	"llm:agent":        llmResultFields(),
	"llm:coding_agent": llmResultFields(),
	"llm:boolean": {
		{Path: "llm.decision", Label: "Decision", DataType: domain.DataText, Description: "Constrained true or false decision."},
		{Path: "llm.content", Label: "Content", DataType: domain.DataText, Description: "Model response text."},
	},
	"llm:choice": {
		{Path: "llm.choice", Label: "Choice", DataType: domain.DataText, Description: "Selected configured option ID."},
		{Path: "llm.content", Label: "Content", DataType: domain.DataText, Description: "Model response text."},
	},
}

func llmResultFields() []domain.DataField {
	return []domain.DataField{
		{Path: "llm.content", Label: "Content", DataType: domain.DataText, Description: "Model response text."},
		{Path: "llm.json", Label: "JSON", DataType: domain.DataObject, Description: "Structured model response, when requested or available.", Optional: true},
	}
}

func addKnownOutputShape(definition domain.NodeDefinition) domain.NodeDefinition {
	fields, exists := knownResultFields[definition.Type]
	if !exists {
		return definition
	}
	for index := range definition.Outputs {
		if definition.Outputs[index].ID == "result" && definition.Outputs[index].Kind == domain.PinData {
			definition.Outputs[index].Fields = append([]domain.DataField(nil), fields...)
			break
		}
	}
	return definition
}
