package jsonquery

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	return registrar.Register(Node{Metadata: datanodes.Node("data:json_query", "Data", "Query JSON", "Read a value from JSON data with a JSONPath expression such as $.items[0].name; plain dotted paths still work.", "scan-search", []domain.NodePort{datanodes.Pin("source", "Source", domain.PinInput, domain.DataAny)}, []domain.NodePort{datanodes.Pin("value", "Value", domain.PinOutput, domain.DataAny)}, []domain.ConfigField{datanodes.Field("path", "JSON path", "string", "$.geonames[0].lng", false)}, map[string]any{}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate resolves the configured path against the connected source.
func Evaluate(_ context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	path, _ := invocation.Config["path"].(string)
	return map[string]any{"value": Query(invocation.Inputs["source"], path)}, nil
}

// ValueAt resolves a plain dotted path through graph-safe structured
// values: each dot-separated part is an object key or, failing that, a
// non-negative list index. JSONPath expressions live in path.go; Query
// routes between the two by shape, so paths like `$.items[0]` never reach
// this fallback.
func ValueAt(value any, path string) any {
	if strings.TrimSpace(path) == "" {
		return value
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		next, found := objectValueAt(current, part)
		if !found {
			next, found = listValueAt(current, part)
		}
		if !found {
			return nil
		}
		current = next
	}
	return current
}

func objectValueAt(value any, key string) (any, bool) {
	resolved := dereference(reflect.ValueOf(value))
	if !resolved.IsValid() {
		return nil, false
	}
	switch resolved.Kind() {
	case reflect.Map:
		if resolved.Type().Key().Kind() != reflect.String {
			return nil, false
		}
		mapKey := reflect.New(resolved.Type().Key()).Elem()
		mapKey.SetString(key)
		item := resolved.MapIndex(mapKey)
		if !item.IsValid() || !item.CanInterface() {
			return nil, false
		}
		return item.Interface(), true
	case reflect.Struct:
		for index := 0; index < resolved.NumField(); index++ {
			field := resolved.Type().Field(index)
			if field.PkgPath != "" || field.Name == "" {
				continue
			}
			jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
			if key != field.Name && key != strings.ToLower(field.Name) && (jsonName == "" || jsonName == "-" || key != jsonName) {
				continue
			}
			item := resolved.Field(index)
			if item.CanInterface() {
				return item.Interface(), true
			}
		}
	}
	return nil, false
}

func listValueAt(value any, key string) (any, bool) {
	index, err := strconv.Atoi(key)
	if err != nil || index < 0 {
		return nil, false
	}
	resolved := dereference(reflect.ValueOf(value))
	if !resolved.IsValid() || (resolved.Kind() != reflect.Slice && resolved.Kind() != reflect.Array) || index >= resolved.Len() {
		return nil, false
	}
	item := resolved.Index(index)
	if !item.CanInterface() {
		return nil, false
	}
	return item.Interface(), true
}

func dereference(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}
