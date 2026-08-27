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
	"action:kv_get": {
		{Path: "value", Label: "Value", DataType: domain.DataText, Description: "Stored string value."},
		{Path: "found", Label: "Found", DataType: domain.DataBoolean, Description: "Whether the key existed."},
	},
	"action:kv_set": {
		{Path: "ok", Label: "Set", DataType: domain.DataBoolean, Description: "Whether the write was applied."},
		{Path: "previous", Label: "Previous", DataType: domain.DataText, Description: "Previous value, when requested via GET.", Optional: true},
	},
	"action:kv_delete": {
		{Path: "deleted", Label: "Deleted", DataType: domain.DataNumber, Description: "Number of keys removed."},
	},
	"action:kv_exists": {
		{Path: "count", Label: "Existing keys", DataType: domain.DataNumber, Description: "Number of keys that existed."},
		{Path: "exists", Label: "Exists", DataType: domain.DataBoolean, Description: "Whether at least one key existed."},
	},
	"action:kv_increment": {
		{Path: "value", Label: "New value", DataType: domain.DataNumber, Description: "Counter value after the increment."},
	},
	"action:kv_rename": {
		{Path: "ok", Label: "Renamed", DataType: domain.DataBoolean, Description: "Whether the rename succeeded."},
	},
	"action:kv_expire": {
		{Path: "ok", Label: "Applied", DataType: domain.DataBoolean, Description: "Whether the expiry change was applied."},
	},
	"action:kv_ttl": {
		{Path: "ttl", Label: "TTL seconds", DataType: domain.DataNumber, Description: "Remaining time to live; -1 means no expiry, -2 means the key is gone."},
	},
	"action:kv_hash_get": {
		{Path: "value", Label: "Value", DataType: domain.DataText, Description: "Single field value."},
		{Path: "found", Label: "Found", DataType: domain.DataBoolean, Description: "Whether the field or hash existed."},
		{Path: "fields", Label: "Fields", DataType: domain.DataObject, Description: "Whole hash as an object."},
	},
	"action:kv_hash_set": {
		{Path: "added", Label: "New fields", DataType: domain.DataNumber, Description: "Fields newly created by HSET or removed by HDEL."},
	},
	"action:kv_list_push": {
		{Path: "length", Label: "List length", DataType: domain.DataNumber, Description: "List length after the push."},
	},
	"action:kv_list_pop": {
		{Path: "values", Label: "Values", DataType: domain.DataList, Description: "Popped values in pop order."},
		{Path: "value", Label: "Value", DataType: domain.DataText, Description: "First popped value."},
		{Path: "found", Label: "Found", DataType: domain.DataBoolean, Description: "Whether the list had elements."},
	},
	"action:kv_list_range": {
		{Path: "items", Label: "Items", DataType: domain.DataList, Description: "Requested list slice."},
		{Path: "count", Label: "Count", DataType: domain.DataNumber, Description: "Number of returned items."},
	},
	"action:kv_set_add": {
		{Path: "added", Label: "New members", DataType: domain.DataNumber, Description: "Members newly added."},
	},
	"action:kv_set_members": {
		{Path: "members", Label: "Members", DataType: domain.DataList, Description: "All set members."},
		{Path: "count", Label: "Count", DataType: domain.DataNumber, Description: "Member count."},
	},
	"action:kv_set_remove": {
		{Path: "removed", Label: "Removed members", DataType: domain.DataNumber, Description: "Members actually removed."},
	},
	"action:kv_zset_add": {
		{Path: "added", Label: "New members", DataType: domain.DataNumber, Description: "Members newly added."},
	},
	"action:kv_zset_range": {
		{Path: "entries", Label: "Entries", DataType: domain.DataList, Description: "Member/score objects in rank order."},
		{Path: "count", Label: "Count", DataType: domain.DataNumber, Description: "Number of returned entries."},
	},
	"action:kv_zset_remove": {
		{Path: "removed", Label: "Removed members", DataType: domain.DataNumber, Description: "Members actually removed."},
	},
	"action:kv_scan": {
		{Path: "keys", Label: "Keys", DataType: domain.DataList, Description: "Keys from this SCAN page."},
		{Path: "nextCursor", Label: "Next cursor", DataType: domain.DataNumber, Description: "Cursor for the next page; 0 means done."},
		{Path: "done", Label: "Done", DataType: domain.DataBoolean, Description: "Whether the scan is complete."},
	},
	"action:kv_publish": {
		{Path: "receivers", Label: "Receivers", DataType: domain.DataNumber, Description: "Clients that received the message."},
	},
	"action:kv_info": {
		{Path: "info.version", Label: "Version", DataType: domain.DataText, Description: "Server version."},
		{Path: "info.flavor", Label: "Flavor", DataType: domain.DataText, Description: "Server flavour."},
		{Path: "info.totalKeys", Label: "Total keys", DataType: domain.DataNumber, Description: "Keys in the selected database."},
		{Path: "version", Label: "Version", DataType: domain.DataText, Description: "Server version."},
		{Path: "flavor", Label: "Flavor", DataType: domain.DataText, Description: "Server flavour."},
		{Path: "keyCount", Label: "Key count", DataType: domain.DataNumber, Description: "Keys in the selected database."},
	},
	"action:kv_command": {
		{Path: "value", Label: "Value", DataType: domain.DataAny, Description: "Normalized command reply."},
		{Path: "valueText", Label: "Value (text)", DataType: domain.DataText, Description: "Reply rendered as text."},
		{Path: "isNil", Label: "Is nil", DataType: domain.DataBoolean, Description: "Whether Redis replied with nil."},
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
