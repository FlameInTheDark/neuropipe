package catalog

import (
	"strconv"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// blueprintBuiltins are the explicit value and control nodes used by graph v2.
// Existing first-party nodes are also normalised into Blueprint pins, but these
// nodes make common data and control-flow work discoverable without templates.
func blueprintBuiltins() []domain.NodeDefinition {
	execIn := func() []domain.NodePort { return []domain.NodePort{execPin("in", "Exec", domain.PinInput)} }
	execOut := func() []domain.NodePort { return []domain.NodePort{execPin("out", "Then", domain.PinOutput)} }
	return []domain.NodeDefinition{
		blueprintNode("data:constant", "Data", "Constant", "Provide a literal typed value.", "circle-dot", "#22c55e", domain.NodePure,
			nil, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{field("value", "Value", "string", "", false)}, map[string]any{}),
		blueprintNode("data:format_text", "Data", "Format Text", "Format text with an explicit Value data pin.", "text", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("text", "Text", domain.PinOutput, domain.DataText)}, []domain.ConfigField{field("format", "Format", "string", "{.value}", true)}, map[string]any{"format": "{.value}"}),
		blueprintNode("data:get_field", "Data", "Get Field", "Read a field from an object or list value.", "braces", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("source", "Source", domain.PinInput, domain.DataAny)}, nil, []domain.ConfigField{fieldOutputsField("outputs", "Outputs")}, map[string]any{"outputs": defaultFieldOutputs()}),
		// Key and Value remain the legacy port contract for existing v2 drafts.
		// New nodes persist Fields and receive their configurable input pins at
		// validation/runtime time; see definitionForNode in the pipeline package.
		blueprintNode("data:build_object", "Data", "Build Object", "Build an object from configurable typed input pins.", "braces", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("key", "Key", domain.PinInput, domain.DataText), dataPin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("object", "Object", domain.PinOutput, domain.DataObject)}, []domain.ConfigField{objectFieldsField("fields", "Fields")}, map[string]any{"fields": defaultObjectFields()}),
		blueprintNode("data:break_object", "Data", "Break Object", "Split configured object key paths into typed output pins.", "unfold-vertical", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("source", "Source", domain.PinInput, domain.DataObject)}, nil, []domain.ConfigField{fieldOutputsField("outputs", "Outputs")}, map[string]any{"outputs": defaultFieldOutputs()}),
		blueprintNode("data:cast", "Data", "Cast", "Explicitly cast a value to text, number, or Boolean.", "arrow-right-left", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{selectField("target", "Target type", []string{"text", "number", "boolean"})}, map[string]any{"target": "text"}),
		blueprintNode("data:json_query", "Data", "Query JSON", "Read a dotted path from JSON data.", "scan-search", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("source", "Source", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{field("path", "JSON path", "string", "", false)}, map[string]any{}),
		blueprintNode("data:equals", "Data", "Equals", "Compare two values.", "equal", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("left", "Left", domain.PinInput, domain.DataAny), dataPin("right", "Right", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("value", "Equal", domain.PinOutput, domain.DataBoolean)}, nil, map[string]any{}),
		blueprintNode("data:greater_than", "Data", "Greater Than", "Compare two numeric values.", "chevron-right", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("left", "Left", domain.PinInput, domain.DataNumber), dataPin("right", "Right", domain.PinInput, domain.DataNumber)}, []domain.NodePort{dataPin("value", "True", domain.PinOutput, domain.DataBoolean)}, nil, map[string]any{}),
		blueprintNode("data:json_parse", "Data", "Parse JSON", "Parse JSON text into an object or list.", "file-json", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("text", "Text", domain.PinInput, domain.DataText)}, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, nil, map[string]any{}),
		blueprintNode("data:get_variable", "Data", "Get Variable", "Read a value stored during this execution.", "bookmark-check", "#22c55e", domain.NodePure,
			nil, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{field("name", "Variable name", "string", "Result", true)}, map[string]any{}),
		blueprintNode("data:chat_history", "Chat", "Read Chat History", "Read earlier messages in this local chat conversation.", "history", "#a78bfa", domain.NodePure,
			[]domain.NodePort{dataPin("chatId", "Chat ID", domain.PinInput, domain.DataText), dataPin("limit", "Limit", domain.PinInput, domain.DataNumber)}, []domain.NodePort{dataPin("messages", "Messages", domain.PinOutput, domain.DataList)}, nil, map[string]any{"limit": 50}),
		blueprintNode("data:reroute", "Data", "Reroute", "Reorganise a data wire without changing its value.", "waypoints", "#22c55e", domain.NodePure,
			[]domain.NodePort{dataPin("value", "Value", domain.PinInput, domain.DataAny)}, []domain.NodePort{dataPin("value", "Value", domain.PinOutput, domain.DataAny)}, nil, map[string]any{}),

		blueprintNode("math:add", "Math", "Add", "Add two numeric values.", "plus", "#86efac", domain.NodePure,
			mathInputs(), mathOutput(), mathFields(), mathDefaults()),
		blueprintNode("math:subtract", "Math", "Subtract", "Subtract B from A.", "minus", "#86efac", domain.NodePure,
			mathInputs(), mathOutput(), mathFields(), mathDefaults()),
		blueprintNode("math:multiply", "Math", "Multiply", "Multiply two numeric values.", "x", "#86efac", domain.NodePure,
			mathInputs(), mathOutput(), mathFields(), mathDefaults()),
		blueprintNode("math:divide", "Math", "Divide", "Divide A by B.", "divide", "#86efac", domain.NodePure,
			mathInputs(), mathOutput(), mathFields(), mathDefaults()),

		blueprintNode("date:now", "Date", "Now", "Get the current date and time.", "calendar", "#f97316", domain.NodePure,
			nil, []domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
				dataPin("local", "Local String", domain.PinOutput, domain.DataText),
			}, []domain.ConfigField{
				selectField("timezone", "Timezone", []string{"local", "utc"}),
			}, map[string]any{"timezone": "local"}),
		blueprintNode("date:create", "Date", "Create Date", "Create a date from year, month, day and time components.", "calendar-plus", "#f97316", domain.NodePure,
			dateCreateInputs(), []domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
			}, dateCreateFields(), dateCreateDefaults()),
		blueprintNode("date:extract", "Date", "Extract Components", "Extract year, month, day, time and other components from a timestamp.", "calendar-search", "#f97316", domain.NodePure,
			[]domain.NodePort{dataPin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{
				dataPin("year", "Year", domain.PinOutput, domain.DataNumber),
				dataPin("month", "Month", domain.PinOutput, domain.DataNumber),
				dataPin("day", "Day", domain.PinOutput, domain.DataNumber),
				dataPin("hour", "Hour", domain.PinOutput, domain.DataNumber),
				dataPin("minute", "Minute", domain.PinOutput, domain.DataNumber),
				dataPin("second", "Second", domain.PinOutput, domain.DataNumber),
				dataPin("millisecond", "Millisecond", domain.PinOutput, domain.DataNumber),
				dataPin("weekday", "Weekday (0=Sun)", domain.PinOutput, domain.DataNumber),
				dataPin("dayOfYear", "Day of Year", domain.PinOutput, domain.DataNumber),
				dataPin("weekOfYear", "Week of Year", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
				dataPin("unix", "Unix Seconds", domain.PinOutput, domain.DataNumber),
				dataPin("unixMs", "Unix Milliseconds", domain.PinOutput, domain.DataNumber),
			}, []domain.ConfigField{
				selectField("timezone", "Timezone", []string{"local", "utc"}),
			}, map[string]any{"timezone": "local"}),
		blueprintNode("date:format", "Date", "Format Date", "Format a timestamp as text using a Go time format string (reference: 2006-01-02 15:04:05).", "calendar-days", "#f97316", domain.NodePure,
			[]domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber),
				dataPin("format", "Format", domain.PinInput, domain.DataText),
			}, []domain.NodePort{dataPin("text", "Text", domain.PinOutput, domain.DataText)}, []domain.ConfigField{
				field("format", "Format", "string", "2006-01-02 15:04:05", true),
				selectField("timezone", "Timezone", []string{"local", "utc"}),
			}, map[string]any{"format": "2006-01-02 15:04:05", "timezone": "local"}),
		blueprintNode("date:parse", "Date", "Parse Date", "Parse a date string into a timestamp. If format is empty, tries common formats (RFC3339, ISO8601, etc.).", "calendar-search", "#f97316", domain.NodePure,
			[]domain.NodePort{
				dataPin("text", "Text", domain.PinInput, domain.DataText),
				dataPin("format", "Format", domain.PinInput, domain.DataText),
			}, []domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
			}, []domain.ConfigField{
				field("format", "Format", "string", "", false),
				selectField("timezone", "Timezone", []string{"local", "utc"}),
			}, map[string]any{"format": "", "timezone": "local"}),
		blueprintNode("date:compare", "Date", "Compare Dates", "Compare two timestamps and return boolean results and differences.", "git-compare", "#f97316", domain.NodePure,
			[]domain.NodePort{
				dataPin("left", "Left (ms)", domain.PinInput, domain.DataNumber),
				dataPin("right", "Right (ms)", domain.PinInput, domain.DataNumber),
			}, []domain.NodePort{
				dataPin("before", "Before", domain.PinOutput, domain.DataBoolean),
				dataPin("after", "After", domain.PinOutput, domain.DataBoolean),
				dataPin("equal", "Equal", domain.PinOutput, domain.DataBoolean),
				dataPin("diffMs", "Difference (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("diffSeconds", "Difference (s)", domain.PinOutput, domain.DataNumber),
				dataPin("diffMinutes", "Difference (min)", domain.PinOutput, domain.DataNumber),
				dataPin("diffHours", "Difference (h)", domain.PinOutput, domain.DataNumber),
				dataPin("diffDays", "Difference (days)", domain.PinOutput, domain.DataNumber),
			}, nil, map[string]any{}),
		blueprintNode("date:add", "Date", "Add Duration", "Add years, months, days, hours, minutes, seconds, milliseconds to a timestamp.", "calendar-plus-2", "#f97316", domain.NodePure,
			dateMathInputs(), []domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
			}, dateMathFields(), dateMathDefaults()),
		blueprintNode("date:subtract", "Date", "Subtract Duration", "Subtract years, months, days, hours, minutes, seconds, milliseconds from a timestamp.", "calendar-minus-2", "#f97316", domain.NodePure,
			dateMathInputs(), []domain.NodePort{
				dataPin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber),
				dataPin("iso", "ISO 8601", domain.PinOutput, domain.DataText),
			}, dateMathFields(), dateMathDefaults()),
		blueprintNode("date:to_unix", "Date", "To Unix Seconds", "Convert milliseconds timestamp to Unix seconds.", "hash", "#f97316", domain.NodePure,
			[]domain.NodePort{dataPin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{
				dataPin("value", "Unix Seconds", domain.PinOutput, domain.DataNumber),
			}, nil, map[string]any{}),
		blueprintNode("date:to_unix_ms", "Date", "To Unix Milliseconds", "Pass through milliseconds timestamp as Unix milliseconds.", "hash", "#f97316", domain.NodePure,
			[]domain.NodePort{dataPin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{
				dataPin("value", "Unix Milliseconds", domain.PinOutput, domain.DataNumber),
			}, nil, map[string]any{}),

		blueprintNode("flow:branch", "Flow", "Branch", "Route execution from a Boolean data value.", "git-branch", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("condition", "Condition", domain.PinInput, domain.DataBoolean)), []domain.NodePort{execPin("true", "True", domain.PinOutput), execPin("false", "False", domain.PinOutput)}, nil, map[string]any{}),
		blueprintNode("flow:sequence", "Flow", "Sequence", "Execute each output in order.", "list-ordered", "#fbbf24", domain.NodeImpure,
			execIn(), []domain.NodePort{execPin("then_0", "Then 0", domain.PinOutput), execPin("then_1", "Then 1", domain.PinOutput)}, nil, map[string]any{}),
		blueprintNode("flow:for_each", "Flow", "For Each Loop", "Run Loop Body for every item, then Completed.", "repeat-2", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("items", "Array", domain.PinInput, domain.DataList)), []domain.NodePort{execPin("loop", "Loop Body", domain.PinOutput), execPin("completed", "Completed", domain.PinOutput), dataPin("item", "Array Element", domain.PinOutput, domain.DataAny), dataPin("index", "Array Index", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}),
		blueprintNode("flow:for_loop", "Flow", "For Loop", "Run Loop Body between inclusive numeric bounds.", "repeat", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("first", "First Index", domain.PinInput, domain.DataNumber), dataPin("last", "Last Index", domain.PinInput, domain.DataNumber)), []domain.NodePort{execPin("loop", "Loop Body", domain.PinOutput), execPin("completed", "Completed", domain.PinOutput), dataPin("index", "Index", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}),
		blueprintNode("flow:while", "Flow", "While", "Evaluate Condition before each bounded body iteration.", "rotate-cw", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("condition", "Condition", domain.PinInput, domain.DataBoolean)), []domain.NodePort{execPin("loop", "Loop Body", domain.PinOutput), execPin("completed", "Completed", domain.PinOutput)}, nil, map[string]any{}),
		blueprintNode("flow:switch", "Flow", "Switch", "Route execution to a matching named output.", "split", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("selection", "Value", domain.PinInput, domain.DataAny)), []domain.NodePort{execPin("default", "Default", domain.PinOutput)}, []domain.ConfigField{switchCasesField("switch", "Cases")}, map[string]any{"switch": switchCasesConfig()}),
		blueprintNode("flow:do_once", "Flow", "Do Once", "Pass execution only the first time until reset.", "badge-check", "#fbbf24", domain.NodeImpure,
			[]domain.NodePort{execPin("in", "Exec", domain.PinInput), execPin("reset", "Reset", domain.PinInput)}, execOut(), nil, map[string]any{}),
		blueprintNode("flow:gate", "Flow", "Gate", "Route execution only while the gate is open.", "door-open", "#fbbf24", domain.NodeImpure,
			[]domain.NodePort{execPin("in", "Enter", domain.PinInput), execPin("open", "Open", domain.PinInput), execPin("close", "Close", domain.PinInput), execPin("toggle", "Toggle", domain.PinInput)}, execOut(), []domain.ConfigField{field("startOpen", "Start open", "boolean", "true", false)}, map[string]any{"startOpen": true}),
		blueprintNode("flow:flip_flop", "Flow", "FlipFlop", "Alternate between A and B on every pulse.", "repeat-1", "#fbbf24", domain.NodeImpure,
			execIn(), []domain.NodePort{execPin("a", "A", domain.PinOutput), execPin("b", "B", domain.PinOutput)}, nil, map[string]any{}),
		blueprintNode("flow:multi_gate", "Flow", "MultiGate", "Cycle through A, B, and C outputs.", "git-fork", "#fbbf24", domain.NodeImpure,
			[]domain.NodePort{execPin("in", "Exec", domain.PinInput), execPin("reset", "Reset", domain.PinInput)}, []domain.NodePort{execPin("a", "A", domain.PinOutput), execPin("b", "B", domain.PinOutput), execPin("c", "C", domain.PinOutput)}, []domain.ConfigField{field("loop", "Loop", "boolean", "false", false)}, map[string]any{"loop": false}),
		blueprintNode("flow:reroute", "Flow", "Reroute", "Reorganise an execution wire without changing control flow.", "waypoints", "#fbbf24", domain.NodeImpure,
			execIn(), execOut(), nil, map[string]any{}),
		blueprintNode("flow:break", "Flow", "Break", "Stop the innermost active loop.", "circle-stop", "#fbbf24", domain.NodeImpure,
			execIn(), nil, nil, map[string]any{}),
		blueprintNode("flow:set_variable", "Data", "Set Variable", "Store a data value for this execution.", "bookmark-plus", "#fbbf24", domain.NodeImpure,
			append(execIn(), dataPin("value", "Value", domain.PinInput, domain.DataAny)), append(execOut(), dataPin("result", "Value", domain.PinOutput, domain.DataAny)), []domain.ConfigField{field("name", "Variable name", "string", "Result", true)}, map[string]any{}),
		blueprintNode("action:chat_reply", "Chat", "Reply to Chat", "Send an ordered Markdown reply to the active chat run.", "reply", "#a78bfa", domain.NodeImpure,
			append(execIn(), dataPin("text", "Text", domain.PinInput, domain.DataText), dataPin("chatRunId", "Chat Run ID", domain.PinInput, domain.DataText)), execOut(), nil, map[string]any{}),
		blueprintNode("action:chat_status", "Chat", "Update Chat Status", "Update the spinner text for the active chat run.", "loader-circle", "#a78bfa", domain.NodeImpure,
			append(execIn(), dataPin("status", "Status", domain.PinInput, domain.DataText), dataPin("chatRunId", "Chat Run ID", domain.PinInput, domain.DataText)), execOut(), nil, map[string]any{}),
		blueprintNode("flow:return", "Flow", "Return", "Finish the current impure function or pipeline.", "corner-down-left", "#fbbf24", domain.NodeImpure, execIn(), nil, nil, map[string]any{}),

		blueprintNode("function:entry", "Functions", "Function Entry", "The single entry point for an impure custom function.", "log-in", "#a78bfa", domain.NodeEvent, nil, execOut(), nil, map[string]any{}),
		blueprintNode("function:return", "Functions", "Function Return", "The single return point for an impure custom function.", "log-out", "#a78bfa", domain.NodeImpure, execIn(), nil, nil, map[string]any{}),
		blueprintNode("function:input", "Functions", "Function Inputs", "Typed inputs supplied by a pure custom function call.", "log-in", "#a78bfa", domain.NodePure, nil, nil, nil, map[string]any{}),
		blueprintNode("function:output", "Functions", "Function Outputs", "Typed outputs returned from a pure custom function call.", "log-out", "#a78bfa", domain.NodePure, nil, nil, nil, map[string]any{}),
	}
}

func mathInputs() []domain.NodePort {
	inputs := []domain.NodePort{
		dataPin("a", "A", domain.PinInput, domain.DataNumber),
		dataPin("b", "B", domain.PinInput, domain.DataNumber),
	}
	for index := range inputs {
		// A wire always wins, but these defaults make a Math node useful on
		// its own and keep old manually-created nodes executable.
		inputs[index].Default = 0.0
	}
	return inputs
}

func mathOutput() []domain.NodePort {
	return []domain.NodePort{dataPin("result", "Result", domain.PinOutput, domain.DataNumber)}
}

func mathFields() []domain.ConfigField {
	return []domain.ConfigField{
		field("a", "A", "number", "0", false),
		field("b", "B", "number", "0", false),
	}
}

func mathDefaults() map[string]any { return map[string]any{"a": 0.0, "b": 0.0} }

func blueprintNode(nodeType, category, label, description, icon, color string, mode domain.NodeExecutionMode, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	return normalizeDefinition(domain.NodeDefinition{Type: nodeType, Category: category, Label: label, Description: description, Icon: icon, Color: color, Mode: mode, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"})
}

func execPin(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

func dataPin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Color: dataColor(dataType), MaxConnections: 1}
}

func dataColor(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataObject:
		return "#60a5fa"
	case domain.DataList:
		return "#facc15"
	default:
		return "#a1a1aa"
	}
}

func fieldOutputsField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "field-outputs", Required: true}
}

func objectFieldsField(name, label string) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: "object-fields", Required: true}
}

func defaultFieldOutputs() []any {
	return []any{map[string]any{
		"id":       "value",
		"label":    "Value",
		"path":     "value",
		"dataType": string(domain.DataAny),
	}}
}

func defaultObjectFields() []any {
	return []any{map[string]any{
		"id":       "value",
		"label":    "Value",
		"key":      "value",
		"dataType": string(domain.DataAny),
	}}
}

func dateCreateInputs() []domain.NodePort {
	inputs := []domain.NodePort{
		dataPin("year", "Year", domain.PinInput, domain.DataNumber),
		dataPin("month", "Month (1-12)", domain.PinInput, domain.DataNumber),
		dataPin("day", "Day", domain.PinInput, domain.DataNumber),
		dataPin("hour", "Hour (0-23)", domain.PinInput, domain.DataNumber),
		dataPin("minute", "Minute (0-59)", domain.PinInput, domain.DataNumber),
		dataPin("second", "Second (0-59)", domain.PinInput, domain.DataNumber),
		dataPin("millisecond", "Millisecond (0-999)", domain.PinInput, domain.DataNumber),
	}
	for index := range inputs {
		inputs[index].Default = 0.0
	}
	return inputs
}

func dateCreateFields() []domain.ConfigField {
	now := time.Now()
	return []domain.ConfigField{
		field("year", "Year", "number", strconv.Itoa(now.Year()), false),
		field("month", "Month (1-12)", "number", "1", false),
		field("day", "Day", "number", "1", false),
		field("hour", "Hour (0-23)", "number", "0", false),
		field("minute", "Minute (0-59)", "number", "0", false),
		field("second", "Second (0-59)", "number", "0", false),
		field("millisecond", "Millisecond (0-999)", "number", "0", false),
		selectField("timezone", "Timezone", []string{"local", "utc"}),
	}
}

func dateCreateDefaults() map[string]any {
	now := time.Now()
	return map[string]any{
		"year":        float64(now.Year()),
		"month":       1.0,
		"day":         1.0,
		"hour":        0.0,
		"minute":      0.0,
		"second":      0.0,
		"millisecond": 0.0,
		"timezone":    "local",
	}
}

func dateMathInputs() []domain.NodePort {
	inputs := []domain.NodePort{
		dataPin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber),
		dataPin("years", "Years", domain.PinInput, domain.DataNumber),
		dataPin("months", "Months", domain.PinInput, domain.DataNumber),
		dataPin("days", "Days", domain.PinInput, domain.DataNumber),
		dataPin("hours", "Hours", domain.PinInput, domain.DataNumber),
		dataPin("minutes", "Minutes", domain.PinInput, domain.DataNumber),
		dataPin("seconds", "Seconds", domain.PinInput, domain.DataNumber),
		dataPin("milliseconds", "Milliseconds", domain.PinInput, domain.DataNumber),
	}
	for index := range inputs {
		inputs[index].Default = 0.0
	}
	return inputs
}

func dateMathFields() []domain.ConfigField {
	return []domain.ConfigField{
		field("years", "Years", "number", "0", false),
		field("months", "Months", "number", "0", false),
		field("days", "Days", "number", "0", false),
		field("hours", "Hours", "number", "0", false),
		field("minutes", "Minutes", "number", "0", false),
		field("seconds", "Seconds", "number", "0", false),
		field("milliseconds", "Milliseconds", "number", "0", false),
		selectField("timezone", "Timezone", []string{"local", "utc"}),
	}
}

func dateMathDefaults() map[string]any {
	return map[string]any{
		"years":        0.0,
		"months":       0.0,
		"days":         0.0,
		"hours":        0.0,
		"minutes":      0.0,
		"seconds":      0.0,
		"milliseconds": 0.0,
		"timezone":     "local",
	}
}
