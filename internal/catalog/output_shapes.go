package catalog

import (
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

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
		pin := &definition.Outputs[index]
		if pin.ID != "result" || pin.Kind != domain.PinData {
			continue
		}
		if len(pin.Fields) == 0 {
			pin.Fields = append([]domain.DataField(nil), fields...)
		}
		// Give the pin a record spec derived from the documented fields so
		// tooltips and dot-completion expose the structure even when the
		// node relies on the generic result contract.
		if pin.Type == nil || pin.Type.Kind != domain.TypeRecord || len(pin.Type.Fields) == 0 {
			spec := recordFromDataFields(fields)
			pin.Type = &spec
			pin.DataType = domain.DataObject
			pin.Color = "#60a5fa"
		}
	}
	return definition
}

// recordFromDataFields builds a nested record TypeSpec from dotted field
// paths ("llm.content" → record llm { content }).
func recordFromDataFields(fields []domain.DataField) domain.TypeSpec {
	root := domain.TypeSpec{Kind: domain.TypeRecord}
	for _, field := range fields {
		segments := strings.Split(field.Path, ".")
		current := &root
		for _, segment := range segments[:len(segments)-1] {
			var nested *domain.TypeSpec
			for index := range current.Fields {
				if current.Fields[index].ID == segment {
					nested = &current.Fields[index].Type
					break
				}
			}
			if nested == nil || nested.Kind != domain.TypeRecord {
				record := domain.TypeSpec{Kind: domain.TypeRecord}
				current.Fields = append(current.Fields, domain.TypeFieldSpec{ID: segment, Name: segment, Type: record})
				nested = &current.Fields[len(current.Fields)-1].Type
			}
			current = nested
		}
		leaf := segments[len(segments)-1]
		if hasTypeField(current, leaf) {
			continue
		}
		current.Fields = append(current.Fields, domain.TypeFieldSpec{
			ID:   leaf,
			Name: leaf,
			Type: dataTypeToSpec(field.DataType),
		})
	}
	return root
}

func hasTypeField(spec *domain.TypeSpec, id string) bool {
	for _, existing := range spec.Fields {
		if existing.ID == id {
			return true
		}
	}
	return false
}

func dataTypeToSpec(dataType domain.DataType) domain.TypeSpec {
	switch dataType {
	case domain.DataNumber:
		return domain.TypeSpec{Kind: domain.TypeFloat}
	case domain.DataBoolean:
		return domain.TypeSpec{Kind: domain.TypeBool}
	case domain.DataText:
		return domain.TypeSpec{Kind: domain.TypeString}
	case domain.DataList:
		return domain.TypeSpec{Kind: domain.TypeList, Value: &domain.TypeSpec{Kind: domain.TypeAny}}
	case domain.DataObject:
		return domain.TypeSpec{Kind: domain.TypeMap, Key: &domain.TypeSpec{Kind: domain.TypeString}, Value: &domain.TypeSpec{Kind: domain.TypeAny}}
	default:
		return domain.TypeSpec{Kind: domain.TypeAny}
	}
}
