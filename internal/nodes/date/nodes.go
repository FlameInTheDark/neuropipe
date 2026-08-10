package date

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Node is the common first-party node implementation used by each Date node.
// Its executor is supplied by the corresponding operation below.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Register contributes every Date node through the same registry contract as
// all other built-ins. The graph engine has no date-specific dispatch.
func Register(registrar nodes.Registrar) error {
	for _, node := range All() {
		if err := registrar.Register(node); err != nil {
			return err
		}
	}
	return nil
}

// All returns the complete Date node set in deterministic source order.
func All() []Node {
	return []Node{
		newNode(definition("date:now", "Now", "Get the current date and time.", "calendar", nil, []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber), pin("iso", "ISO 8601", domain.PinOutput, domain.DataText), pin("local", "Local String", domain.PinOutput, domain.DataText)}, []domain.ConfigField{selectField("timezone", "Timezone", "local", "utc")}, map[string]any{"timezone": "local"}), now),
		newNode(definition("date:create", "Create Date", "Create a date from year, month, day and time components.", "calendar-plus", createInputs(), []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber), pin("iso", "ISO 8601", domain.PinOutput, domain.DataText)}, createFields(), createDefaults()), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return create(invocation.Inputs, invocation.Config, location(invocation.Config))
		}),
		newNode(definition("date:extract", "Extract Components", "Extract year, month, day, time and other components from a timestamp.", "calendar-search", []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{pin("year", "Year", domain.PinOutput, domain.DataNumber), pin("month", "Month", domain.PinOutput, domain.DataNumber), pin("day", "Day", domain.PinOutput, domain.DataNumber), pin("hour", "Hour", domain.PinOutput, domain.DataNumber), pin("minute", "Minute", domain.PinOutput, domain.DataNumber), pin("second", "Second", domain.PinOutput, domain.DataNumber), pin("millisecond", "Millisecond", domain.PinOutput, domain.DataNumber), pin("weekday", "Weekday (0=Sun)", domain.PinOutput, domain.DataNumber), pin("dayOfYear", "Day of Year", domain.PinOutput, domain.DataNumber), pin("weekOfYear", "Week of Year", domain.PinOutput, domain.DataNumber), pin("iso", "ISO 8601", domain.PinOutput, domain.DataText), pin("unix", "Unix Seconds", domain.PinOutput, domain.DataNumber), pin("unixMs", "Unix Milliseconds", domain.PinOutput, domain.DataNumber)}, []domain.ConfigField{selectField("timezone", "Timezone", "local", "utc")}, map[string]any{"timezone": "local"}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return extract(invocation.Inputs, location(invocation.Config))
		}),
		newNode(definition("date:format", "Format Date", "Format a timestamp as text using a Go time format string (reference: 2006-01-02 15:04:05).", "calendar-days", []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber), pin("format", "Format", domain.PinInput, domain.DataText)}, []domain.NodePort{pin("text", "Text", domain.PinOutput, domain.DataText)}, []domain.ConfigField{field("format", "Format", "string", "2006-01-02 15:04:05", true), selectField("timezone", "Timezone", "local", "utc")}, map[string]any{"format": "2006-01-02 15:04:05", "timezone": "local"}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return format(invocation.Inputs, invocation.Config, location(invocation.Config))
		}),
		newNode(definition("date:parse", "Parse Date", "Parse a date string into a timestamp. If format is empty, tries common formats (RFC3339, ISO8601, etc.).", "calendar-search", []domain.NodePort{pin("text", "Text", domain.PinInput, domain.DataText), pin("format", "Format", domain.PinInput, domain.DataText)}, []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber), pin("iso", "ISO 8601", domain.PinOutput, domain.DataText)}, []domain.ConfigField{field("format", "Format", "string", "", false), selectField("timezone", "Timezone", "local", "utc")}, map[string]any{"format": "", "timezone": "local"}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return parse(invocation.Inputs, invocation.Config, location(invocation.Config))
		}),
		newNode(definition("date:compare", "Compare Dates", "Compare two timestamps and return boolean results and differences.", "git-compare", []domain.NodePort{pin("left", "Left (ms)", domain.PinInput, domain.DataNumber), pin("right", "Right (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{pin("before", "Before", domain.PinOutput, domain.DataBoolean), pin("after", "After", domain.PinOutput, domain.DataBoolean), pin("equal", "Equal", domain.PinOutput, domain.DataBoolean), pin("diffMs", "Difference (ms)", domain.PinOutput, domain.DataNumber), pin("diffSeconds", "Difference (s)", domain.PinOutput, domain.DataNumber), pin("diffMinutes", "Difference (min)", domain.PinOutput, domain.DataNumber), pin("diffHours", "Difference (h)", domain.PinOutput, domain.DataNumber), pin("diffDays", "Difference (days)", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return compare(invocation.Inputs)
		}),
		newNode(definition("date:add", "Add Duration", "Add years, months, days, hours, minutes, seconds, milliseconds to a timestamp.", "calendar-plus-2", mathInputs(), timestampOutputs(), mathFields(), mathDefaults()), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return add(invocation.Inputs, invocation.Config, location(invocation.Config), 1)
		}),
		newNode(definition("date:subtract", "Subtract Duration", "Subtract years, months, days, hours, minutes, seconds, milliseconds from a timestamp.", "calendar-minus-2", mathInputs(), timestampOutputs(), mathFields(), mathDefaults()), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return add(invocation.Inputs, invocation.Config, location(invocation.Config), -1)
		}),
		newNode(definition("date:to_unix", "To Unix Seconds", "Convert milliseconds timestamp to Unix seconds.", "hash", []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{pin("value", "Unix Seconds", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return toUnix(invocation.Inputs, false)
		}),
		newNode(definition("date:to_unix_ms", "To Unix Milliseconds", "Pass through milliseconds timestamp as Unix milliseconds.", "hash", []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber)}, []domain.NodePort{pin("value", "Unix Milliseconds", domain.PinOutput, domain.DataNumber)}, nil, map[string]any{}), func(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
			return toUnix(invocation.Inputs, true)
		}),
	}
}

// Find returns one self-contained Date node by type.
func Find(nodeType string) (Node, bool) {
	for _, node := range All() {
		if node.Definition().Type == nodeType {
			return node, true
		}
	}
	return Node{}, false
}

type operation func(context.Context, nodes.Invocation, nodes.Runtime) (map[string]any, error)

func newNode(definition domain.NodeDefinition, execute operation) Node {
	return Node{Metadata: definition, Executor: nodes.Outputs(execute)}
}

func now(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	value := time.Now().In(location(invocation.Config))
	return map[string]any{"timestamp": float64(value.UnixMilli()), "iso": value.Format(time.RFC3339Nano), "local": value.Format(time.DateTime)}, nil
}

func definition(nodeType, label, description, icon string, inputs, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	return domain.NodeDefinition{Type: nodeType, Category: "Date", Label: label, Description: description, Icon: icon, Color: "#f97316", Mode: domain.NodePure, Inputs: inputs, Outputs: outputs, Fields: fields, DefaultConfig: defaults, Source: "builtin"}
}

func pin(id, label string, direction domain.PinDirection, dataType domain.DataType) domain.NodePort {
	typeSpec := typespec.FromDataType(dataType)
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: dataType, Type: &typeSpec, Color: color(dataType), MaxConnections: 1}
}

func color(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	default:
		return "#a1a1aa"
	}
}

func field(name, label, kind, placeholder string, required bool) domain.ConfigField {
	return domain.ConfigField{Name: name, Label: label, Kind: kind, Placeholder: placeholder, Required: required}
}

func selectField(name, label string, options ...string) domain.ConfigField {
	values := make([]domain.Option, 0, len(options))
	for _, option := range options {
		values = append(values, domain.Option{Value: option, Label: option})
	}
	return domain.ConfigField{Name: name, Label: label, Kind: "select", Options: values, Required: true}
}

func createInputs() []domain.NodePort {
	inputs := []domain.NodePort{pin("year", "Year", domain.PinInput, domain.DataNumber), pin("month", "Month (1-12)", domain.PinInput, domain.DataNumber), pin("day", "Day", domain.PinInput, domain.DataNumber), pin("hour", "Hour (0-23)", domain.PinInput, domain.DataNumber), pin("minute", "Minute (0-59)", domain.PinInput, domain.DataNumber), pin("second", "Second (0-59)", domain.PinInput, domain.DataNumber), pin("millisecond", "Millisecond (0-999)", domain.PinInput, domain.DataNumber)}
	for index := range inputs {
		inputs[index].Default = 0.0
	}
	return inputs
}

func createFields() []domain.ConfigField {
	return []domain.ConfigField{field("year", "Year", "number", strconv.Itoa(time.Now().Year()), false), field("month", "Month (1-12)", "number", "1", false), field("day", "Day", "number", "1", false), field("hour", "Hour (0-23)", "number", "0", false), field("minute", "Minute (0-59)", "number", "0", false), field("second", "Second (0-59)", "number", "0", false), field("millisecond", "Millisecond (0-999)", "number", "0", false), selectField("timezone", "Timezone", "local", "utc")}
}

func createDefaults() map[string]any {
	return map[string]any{"year": float64(time.Now().Year()), "month": 1.0, "day": 1.0, "hour": 0.0, "minute": 0.0, "second": 0.0, "millisecond": 0.0, "timezone": "local"}
}

func mathInputs() []domain.NodePort {
	inputs := []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinInput, domain.DataNumber), pin("years", "Years", domain.PinInput, domain.DataNumber), pin("months", "Months", domain.PinInput, domain.DataNumber), pin("days", "Days", domain.PinInput, domain.DataNumber), pin("hours", "Hours", domain.PinInput, domain.DataNumber), pin("minutes", "Minutes", domain.PinInput, domain.DataNumber), pin("seconds", "Seconds", domain.PinInput, domain.DataNumber), pin("milliseconds", "Milliseconds", domain.PinInput, domain.DataNumber)}
	for index := range inputs {
		inputs[index].Default = 0.0
	}
	return inputs
}

func timestampOutputs() []domain.NodePort {
	return []domain.NodePort{pin("timestamp", "Timestamp (ms)", domain.PinOutput, domain.DataNumber), pin("iso", "ISO 8601", domain.PinOutput, domain.DataText)}
}

func mathFields() []domain.ConfigField {
	return []domain.ConfigField{field("years", "Years", "number", "0", false), field("months", "Months", "number", "0", false), field("days", "Days", "number", "0", false), field("hours", "Hours", "number", "0", false), field("minutes", "Minutes", "number", "0", false), field("seconds", "Seconds", "number", "0", false), field("milliseconds", "Milliseconds", "number", "0", false), selectField("timezone", "Timezone", "local", "utc")}
}

func mathDefaults() map[string]any {
	return map[string]any{"years": 0.0, "months": 0.0, "days": 0.0, "hours": 0.0, "minutes": 0.0, "seconds": 0.0, "milliseconds": 0.0, "timezone": "local"}
}

// Verify date registrations during package initialization without retaining a
// separate runtime switch. It catches duplicate or incomplete definitions in
// local development before the application registry is built.
func init() {
	seen := make(map[string]struct{})
	for _, node := range All() {
		definition := node.Definition()
		if definition.Type == "" {
			panic("date node has no type")
		}
		if _, duplicate := seen[definition.Type]; duplicate {
			panic(fmt.Sprintf("duplicate date node %q", definition.Type))
		}
		seen[definition.Type] = struct{}{}
	}
}
