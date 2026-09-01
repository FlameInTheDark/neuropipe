// Package javascript registers Neuropipe's explicitly capability-gated
// JavaScript Blueprint node.
package javascript

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
	"github.com/dop251/goja"
	"github.com/google/uuid"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

const (
	codeConfigKey         = "code"
	inputsConfigKey       = "inputs"
	outputsConfigKey      = "outputs"
	capabilitiesConfigKey = "capabilities"
	codeInputPinID        = "code"
	maxScriptBytes        = 64 * 1024
	maxScriptDuration     = 5 * time.Second
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

var reservedIdentifiers = map[string]struct{}{
	"arguments": {}, "await": {}, "break": {}, "case": {}, "catch": {}, "class": {}, "const": {}, "continue": {}, "debugger": {}, "default": {}, "delete": {}, "do": {}, "else": {}, "enum": {}, "eval": {}, "export": {}, "extends": {}, "false": {}, "finally": {}, "for": {}, "function": {}, "if": {}, "implements": {}, "import": {}, "in": {}, "instanceof": {}, "interface": {}, "let": {}, "new": {}, "null": {}, "package": {}, "private": {}, "protected": {}, "public": {}, "return": {}, "static": {}, "super": {}, "switch": {}, "this": {}, "throw": {}, "true": {}, "try": {}, "typeof": {}, "undefined": {}, "var": {}, "void": {}, "while": {}, "with": {}, "yield": {},
	"inputs": {}, "np": {}, "code": {},
}

// New creates the complete JavaScript node module.
func New() Node {
	definition := definition()
	return Node{Metadata: definition, Resolver: resolve, Executor: execute}
}

// Register contributes the JavaScript module to the deterministic built-in registry.
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	stringType := domain.TypeSpec{Kind: domain.TypeString}
	return domain.NodeDefinition{
		Type:        "action:javascript",
		Category:    "Code",
		Label:       "JavaScript",
		Description: "Run typed JavaScript with an explicit, capability-gated Neuropipe API.",
		Icon:        "braces",
		Color:       "#facc15",
		Mode:        domain.NodeImpure,
		Inputs: []domain.NodePort{
			{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1},
			{ID: codeInputPinID, Label: "Code", Kind: domain.PinData, Direction: domain.PinInput, DataType: domain.DataText, Type: &stringType, Color: "#e879f9", MaxConnections: 1, IgnoreConfigFallback: true},
		},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{{Name: codeConfigKey, Label: "Code", Kind: "javascript-editor", Required: true}},
		DefaultConfig: map[string]any{
			codeConfigKey:         "// Return an object with one property for each configured output.\nreturn {};",
			inputsConfigKey:       []any{},
			outputsConfigKey:      []any{},
			capabilitiesConfigKey: []any{},
		},
		Source: "builtin",
	}
}

type configuredPin struct {
	ID       string          `json:"id"`
	Label    string          `json:"label"`
	Type     domain.TypeSpec `json:"type"`
	Required bool            `json:"required,omitempty"`
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	definition := definition()
	config := config(node)
	inputs, err := configuredPins(config, definition.DefaultConfig, inputsConfigKey)
	if err != nil {
		return definition, err
	}
	outputs, err := configuredPins(config, definition.DefaultConfig, outputsConfigKey)
	if err != nil {
		return definition, err
	}
	capabilities, err := configuredCapabilities(config, definition.DefaultConfig)
	if err != nil {
		return definition, err
	}
	definition.Inputs = append(definition.Inputs[:2:2], ports(inputs, domain.PinInput)...)
	definition.Outputs = append(definition.Outputs[:1:1], ports(outputs, domain.PinOutput)...)
	definition.Capabilities = capabilities
	if code, ok := config[codeConfigKey].(string); ok && strings.TrimSpace(code) != "" {
		if err := Validate(code); err != nil {
			return definition, err
		}
	}
	return definition, nil
}

func ports(pins []configuredPin, direction domain.PinDirection) []domain.NodePort {
	result := make([]domain.NodePort, 0, len(pins))
	for _, pin := range pins {
		typeSpec := pin.Type
		result = append(result, domain.NodePort{ID: pin.ID, Label: pin.Label, Kind: domain.PinData, Direction: direction, DataType: dataTypeFor(typeSpec), Type: &typeSpec, Color: colorFor(typeSpec), Required: direction == domain.PinInput && pin.Required, MaxConnections: 1})
	}
	return result
}

func configuredPins(config, defaults map[string]any, key string) ([]configuredPin, error) {
	raw, exists := config[key]
	if !exists {
		raw = defaults[key]
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list", key)
	}
	pins := make([]configuredPin, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("%s %d: encode contract: %w", key, index+1, err)
		}
		var pin configuredPin
		if err := json.Unmarshal(encoded, &pin); err != nil {
			return nil, fmt.Errorf("%s %d: decode contract: %w", key, index+1, err)
		}
		pin.ID = strings.TrimSpace(pin.ID)
		pin.Label = strings.TrimSpace(pin.Label)
		if !identifierPattern.MatchString(pin.ID) {
			return nil, fmt.Errorf("%s pin %d needs a valid JavaScript identifier", key, index+1)
		}
		if _, reserved := reservedIdentifiers[pin.ID]; reserved {
			return nil, fmt.Errorf("%s pin %q uses a reserved JavaScript name", key, pin.ID)
		}
		if _, duplicate := seen[pin.ID]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate pin %q", key, pin.ID)
		}
		seen[pin.ID] = struct{}{}
		if pin.Label == "" {
			pin.Label = pin.ID
		}
		if err := typespec.ValidateSpec(pin.Type); err != nil {
			return nil, fmt.Errorf("%s pin %q type: %w", key, pin.ID, err)
		}
		if pin.Type.Kind == domain.TypeMap && (pin.Type.Key == nil || pin.Type.Key.Kind != domain.TypeString) {
			return nil, fmt.Errorf("%s pin %q maps must use text keys in JavaScript", key, pin.ID)
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

func configuredCapabilities(config, defaults map[string]any) ([]domain.Capability, error) {
	raw, exists := config[capabilitiesConfigKey]
	if !exists {
		raw = defaults[capabilitiesConfigKey]
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("allowed system access must be a list")
	}
	allowed := map[domain.Capability]struct{}{
		domain.CapabilityFileRead: {}, domain.CapabilityFileWrite: {}, domain.CapabilityNetwork: {},
	}
	seen := make(map[domain.Capability]struct{}, len(items))
	capabilities := make([]domain.Capability, 0, len(items))
	for _, item := range items {
		capability, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("allowed system access contains an invalid capability")
		}
		value := domain.Capability(strings.TrimSpace(capability))
		if _, supported := allowed[value]; !supported {
			return nil, fmt.Errorf("JavaScript cannot request capability %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		capabilities = append(capabilities, value)
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	return capabilities, nil
}

// Validate checks source with the same Goja parser used by execution. Errors
// intentionally contain a location and message but never echo the source.
func Validate(code string) error {
	if len(code) > maxScriptBytes {
		return fmt.Errorf("JavaScript source exceeds the %d KB limit", maxScriptBytes/1024)
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("JavaScript code is required")
	}
	_, err := goja.Compile("Blueprint JavaScript", wrapped(code), true)
	if err != nil {
		return fmt.Errorf("JavaScript syntax: %w", err)
	}
	return nil
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("JavaScript cancelled: %w", err)
	}
	definition := invocation.Definition
	inputs, err := configuredPins(invocation.Config, definition.DefaultConfig, inputsConfigKey)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs, err := configuredPins(invocation.Config, definition.DefaultConfig, outputsConfigKey)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	capabilities, err := configuredCapabilities(invocation.Config, definition.DefaultConfig)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	code, _ := invocation.Config[codeConfigKey].(string)
	// The Code input pin overrides the editor value when connected. Falling
	// back to the editor's configured source keeps the dialog the source of
	// truth for an unconnected node.
	if invocation.ConnectedInputs[codeInputPinID] {
		if wired, ok := invocation.Inputs[codeInputPinID].(string); ok {
			code = wired
		}
	}
	if strings.TrimSpace(code) == "" {
		code, _ = definition.DefaultConfig[codeConfigKey].(string)
	}
	if err := Validate(code); err != nil {
		return nodes.ExecutionResult{}, err
	}

	vm := goja.New()
	vm.SetMaxCallStackSize(512)
	timer := time.AfterFunc(maxScriptDuration, func() { vm.Interrupt("JavaScript execution timed out") })
	defer timer.Stop()
	program, err := goja.Compile("Blueprint JavaScript", wrapped(code), true)
	if err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("JavaScript syntax: %w", err)
	}
	inputObject := vm.NewObject()
	for _, pin := range inputs {
		value, found := invocation.Inputs[pin.ID]
		if !found {
			if pin.Required {
				return nodes.ExecutionResult{}, fmt.Errorf("JavaScript input %q is required", pin.Label)
			}
			value = nil
		}
		copy, err := copyForJavaScript(value, pin.Type)
		if err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("JavaScript input %q: %w", pin.Label, err)
		}
		if err := inputObject.Set(pin.ID, copy); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("set JavaScript input %q: %w", pin.ID, err)
		}
		if err := vm.GlobalObject().DefineDataProperty(pin.ID, vm.ToValue(copy), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			return nodes.ExecutionResult{}, fmt.Errorf("define JavaScript input %q: %w", pin.ID, err)
		}
	}
	if err := vm.GlobalObject().DefineDataProperty("inputs", inputObject, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("define JavaScript inputs: %w", err)
	}

	var hostAPI nodes.JavaScriptHost
	if provider, ok := runtime.(nodes.JavaScriptHostProvider); ok {
		hostAPI = provider.JavaScriptHost()
	}
	variables, _ := runtime.(nodes.VariableStore)
	np := buildNP(vm, javascriptEnvironment{ctx: ctx, invocation: invocation, host: hostAPI, variables: variables, capabilities: capabilities})
	if err := vm.GlobalObject().DefineDataProperty("np", np, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
		return nodes.ExecutionResult{}, fmt.Errorf("define JavaScript API: %w", err)
	}

	value, err := vm.RunProgram(program)
	if err != nil {
		return nodes.ExecutionResult{}, executionError(err)
	}
	callable, ok := goja.AssertFunction(value)
	if !ok {
		return nodes.ExecutionResult{}, fmt.Errorf("JavaScript did not compile to a callable script")
	}
	returned, err := callable(goja.Undefined())
	if err != nil {
		return nodes.ExecutionResult{}, executionError(err)
	}
	result, err := normalizeOutputs(returned.Export(), outputs)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	return nodes.ExecutionResult{Outputs: result, Ports: []string{"out"}}, nil
}

func wrapped(code string) string { return "(function() { 'use strict';\n" + code + "\n})" }

func executionError(err error) error {
	if interrupt, ok := err.(*goja.InterruptedError); ok {
		return fmt.Errorf("JavaScript interrupted: %v", interrupt.Value())
	}
	return fmt.Errorf("JavaScript execution failed: %w", err)
}

func normalizeOutputs(value any, pins []configuredPin) (map[string]any, error) {
	if len(pins) == 0 {
		if value == nil {
			return map[string]any{}, nil
		}
		if object, ok := value.(map[string]any); ok && len(object) == 0 {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("JavaScript has no configured outputs; return an empty object")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JavaScript must return an object keyed by configured output IDs")
	}
	configured := make(map[string]configuredPin, len(pins))
	for _, pin := range pins {
		configured[pin.ID] = pin
	}
	for id := range object {
		if _, exists := configured[id]; !exists {
			return nil, fmt.Errorf("JavaScript returned unknown output %q", id)
		}
	}
	result := make(map[string]any, len(pins))
	for _, pin := range pins {
		raw, exists := object[pin.ID]
		if !exists || raw == nil {
			return nil, fmt.Errorf("JavaScript did not return output %q", pin.Label)
		}
		normalized, err := normalizeValue(raw, pin.Type)
		if err != nil {
			return nil, fmt.Errorf("JavaScript output %q: %w", pin.Label, err)
		}
		if err := typespec.ValidateValue(normalized, pin.Type); err != nil {
			return nil, fmt.Errorf("JavaScript output %q: %w", pin.Label, err)
		}
		result[pin.ID] = normalized
	}
	return result, nil
}

func normalizeValue(value any, spec domain.TypeSpec) (any, error) {
	switch spec.Kind {
	case domain.TypeAny:
		return cloneAny(value), nil
	case domain.TypeBool:
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("must be true or false")
		}
		return value, nil
	case domain.TypeString:
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("must be text")
		}
		return value, nil
	case domain.TypeInt:
		integer, ok := integerValue(value)
		if !ok {
			return nil, fmt.Errorf("must be a safe integer")
		}
		return integer, nil
	case domain.TypeFloat:
		float, ok := floatValue(value)
		if !ok || math.IsNaN(float) || math.IsInf(float, 0) {
			return nil, fmt.Errorf("must be a finite number")
		}
		return float, nil
	case domain.TypeBytes:
		return byteValue(value)
	case domain.TypeList:
		items, ok := sliceValue(value)
		if !ok || spec.Element == nil {
			return nil, fmt.Errorf("must be a list")
		}
		result := make([]any, len(items))
		for index, item := range items {
			normalized, err := normalizeValue(item, *spec.Element)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", index, err)
			}
			result[index] = normalized
		}
		return result, nil
	case domain.TypeMap:
		if spec.Key == nil || spec.Key.Kind != domain.TypeString || spec.Value == nil {
			return nil, fmt.Errorf("maps need text keys and a value type")
		}
		object, ok := objectValue(value)
		if !ok {
			return nil, fmt.Errorf("must be an object map")
		}
		result := make(map[string]any, len(object))
		for key, item := range object {
			normalized, err := normalizeValue(item, *spec.Value)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", key, err)
			}
			result[key] = normalized
		}
		return result, nil
	case domain.TypeRecord:
		object, ok := objectValue(value)
		if !ok {
			return nil, fmt.Errorf("must be an object record")
		}
		result := make(map[string]any, len(object))
		for _, field := range spec.Fields {
			item, exists := object[field.Name]
			if !exists {
				item, exists = object[field.ID]
			}
			if !exists {
				if field.Optional {
					continue
				}
				return nil, fmt.Errorf("is missing required field %q", field.Name)
			}
			normalized, err := normalizeValue(item, field.Type)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", field.Name, err)
			}
			result[field.Name] = normalized
		}
		return result, nil
	default:
		return nil, fmt.Errorf("uses unsupported type %q", spec.Kind)
	}
}

func copyForJavaScript(value any, spec domain.TypeSpec) (any, error) {
	if value == nil {
		return nil, nil
	}
	if spec.Kind == domain.TypeAny {
		return cloneAny(value), nil
	}
	if err := typespec.ValidateValue(value, spec); err != nil {
		return nil, err
	}
	return normalizeValue(cloneAny(value), spec)
}

func cloneAny(value any) any {
	if data, ok := value.([]byte); ok {
		return append([]byte(nil), data...)
	}
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() {
		return nil
	}
	switch reflectValue.Kind() {
	case reflect.Interface, reflect.Pointer:
		if reflectValue.IsNil() {
			return nil
		}
		return cloneAny(reflectValue.Elem().Interface())
	case reflect.Slice, reflect.Array:
		result := make([]any, reflectValue.Len())
		for index := range result {
			result[index] = cloneAny(reflectValue.Index(index).Interface())
		}
		return result
	case reflect.Map:
		if reflectValue.Type().Key().Kind() != reflect.String {
			return value
		}
		result := make(map[string]any, reflectValue.Len())
		iterator := reflectValue.MapRange()
		for iterator.Next() {
			result[iterator.Key().String()] = cloneAny(iterator.Value().Interface())
		}
		return result
	case reflect.Struct:
		encoded, err := json.Marshal(value)
		if err != nil {
			return value
		}
		var result any
		if json.Unmarshal(encoded, &result) == nil {
			return result
		}
	}
	return value
}

func objectValue(value any) (map[string]any, bool) {
	if object, ok := value.(map[string]any); ok {
		return object, true
	}
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() || reflectValue.Kind() != reflect.Map || reflectValue.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	result := make(map[string]any, reflectValue.Len())
	iterator := reflectValue.MapRange()
	for iterator.Next() {
		result[iterator.Key().String()] = iterator.Value().Interface()
	}
	return result, true
}

func sliceValue(value any) ([]any, bool) {
	if items, ok := value.([]any); ok {
		return items, true
	}
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() || (reflectValue.Kind() != reflect.Array && reflectValue.Kind() != reflect.Slice) {
		return nil, false
	}
	result := make([]any, reflectValue.Len())
	for index := range result {
		result[index] = reflectValue.Index(index).Interface()
	}
	return result, true
}

func integerValue(value any) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	case float64:
		if math.Trunc(number) == number && math.Abs(number) <= 9007199254740991 {
			return int64(number), true
		}
	case float32:
		float := float64(number)
		if math.Trunc(float) == float && math.Abs(float) <= 9007199254740991 {
			return int64(float), true
		}
	}
	return 0, false
}

func floatValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int64:
		return float64(number), true
	case int:
		return float64(number), true
	}
	return 0, false
}

func byteValue(value any) ([]byte, error) {
	switch bytes := value.(type) {
	case []byte:
		return append([]byte(nil), bytes...), nil
	case goja.ArrayBuffer:
		return append([]byte(nil), bytes.Bytes()...), nil
	}
	items, ok := sliceValue(value)
	if !ok {
		return nil, fmt.Errorf("must be Uint8Array or a byte array")
	}
	result := make([]byte, len(items))
	for index, item := range items {
		integer, ok := integerValue(item)
		if !ok || integer < 0 || integer > 255 {
			return nil, fmt.Errorf("byte %d must be an integer from 0 to 255", index)
		}
		result[index] = byte(integer)
	}
	return result, nil
}

func dataTypeFor(spec domain.TypeSpec) domain.DataType {
	switch spec.Kind {
	case domain.TypeString:
		return domain.DataText
	case domain.TypeInt, domain.TypeFloat:
		return domain.DataNumber
	case domain.TypeBool:
		return domain.DataBoolean
	case domain.TypeList:
		return domain.DataList
	case domain.TypeMap, domain.TypeRecord:
		return domain.DataObject
	case domain.TypeBytes:
		return domain.DataBytes
	default:
		return domain.DataAny
	}
}

func colorFor(spec domain.TypeSpec) string {
	switch dataTypeFor(spec) {
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
	case domain.DataBytes:
		return "#fbbf24"
	default:
		return "#a1a1aa"
	}
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}

type javascriptEnvironment struct {
	ctx          context.Context
	invocation   nodes.Invocation
	host         nodes.JavaScriptHost
	variables    nodes.VariableStore
	capabilities []domain.Capability
}

func (environment javascriptEnvironment) allows(capability domain.Capability) bool {
	for _, allowed := range environment.capabilities {
		if allowed == capability {
			return true
		}
	}
	return false
}

func (environment javascriptEnvironment) requireHost(vm *goja.Runtime, capability domain.Capability) nodes.JavaScriptHost {
	if !environment.allows(capability) {
		panic(vm.NewTypeError("Enable %s access on this JavaScript node before using this API", capability))
	}
	if environment.host == nil {
		panic(vm.NewTypeError("JavaScript system access is unavailable for this execution"))
	}
	return environment.host
}

func (environment javascriptEnvironment) system(vm *goja.Runtime) nodes.JavaScriptHost {
	if environment.host == nil {
		panic(vm.NewTypeError("JavaScript system access is unavailable for this execution"))
	}
	return environment.host
}

func buildNP(vm *goja.Runtime, environment javascriptEnvironment) *goja.Object {
	np := vm.NewObject()
	mustSet(vm, np, "context", func(goja.FunctionCall) goja.Value {
		context := map[string]any{"nodeId": environment.invocation.Node.ID}
		if environment.host != nil {
			value := environment.host.ExecutionContext()
			context["pipelineId"], context["executionId"] = value.PipelineID, value.ExecutionID
		}
		return vm.ToValue(context)
	})
	mustSet(vm, np, "uuid", func(goja.FunctionCall) goja.Value { return vm.ToValue(uuid.NewString()) })
	mustSet(vm, np, "assert", func(call goja.FunctionCall) goja.Value {
		if !call.Argument(0).ToBoolean() {
			panic(vm.NewTypeError("%s", optionalString(call.Argument(1), "assertion failed")))
		}
		return goja.Undefined()
	})
	mustSet(vm, np, "fail", func(call goja.FunctionCall) goja.Value {
		panic(vm.NewTypeError("%s", optionalString(call.Argument(0), "JavaScript failed")))
	})

	variables := vm.NewObject()
	mustSet(vm, variables, "get", func(call goja.FunctionCall) goja.Value {
		if environment.variables == nil {
			panic(vm.NewTypeError("run variables are unavailable"))
		}
		value, exists := environment.variables.LookupVariable(requiredString(vm, call.Argument(0), "variable name"))
		if !exists {
			return goja.Undefined()
		}
		return vm.ToValue(cloneAny(value))
	})
	mustSet(vm, variables, "has", func(call goja.FunctionCall) goja.Value {
		if environment.variables == nil {
			return vm.ToValue(false)
		}
		_, exists := environment.variables.LookupVariable(requiredString(vm, call.Argument(0), "variable name"))
		return vm.ToValue(exists)
	})
	mustSet(vm, variables, "set", func(call goja.FunctionCall) goja.Value {
		if environment.variables == nil {
			panic(vm.NewTypeError("run variables are unavailable"))
		}
		environment.variables.StoreVariable(requiredString(vm, call.Argument(0), "variable name"), cloneAny(call.Argument(1).Export()))
		return goja.Undefined()
	})
	mustSet(vm, variables, "delete", func(call goja.FunctionCall) goja.Value {
		if environment.variables == nil {
			panic(vm.NewTypeError("run variables are unavailable"))
		}
		environment.variables.DeleteVariable(requiredString(vm, call.Argument(0), "variable name"))
		return goja.Undefined()
	})
	mustSet(vm, np, "variables", variables)

	base64API := vm.NewObject()
	mustSet(vm, base64API, "encodeText", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(requiredString(vm, call.Argument(0), "text"))))
	})
	mustSet(vm, base64API, "decodeText", func(call goja.FunctionCall) goja.Value {
		decoded, err := base64.StdEncoding.DecodeString(requiredString(vm, call.Argument(0), "Base64 text"))
		if err != nil {
			panic(vm.NewTypeError("invalid Base64 text"))
		}
		return vm.ToValue(string(decoded))
	})
	mustSet(vm, base64API, "encodeBytes", func(call goja.FunctionCall) goja.Value {
		bytes, err := byteValue(call.Argument(0).Export())
		if err != nil {
			panic(vm.NewTypeError("Base64 bytes: %v", err))
		}
		return vm.ToValue(base64.StdEncoding.EncodeToString(bytes))
	})
	mustSet(vm, base64API, "decodeBytes", func(call goja.FunctionCall) goja.Value {
		decoded, err := base64.StdEncoding.DecodeString(requiredString(vm, call.Argument(0), "Base64 text"))
		if err != nil {
			panic(vm.NewTypeError("invalid Base64 text"))
		}
		return vm.ToValue(decoded)
	})
	mustSet(vm, np, "base64", base64API)

	hashAPI := vm.NewObject()
	mustSet(vm, hashAPI, "sha256", func(call goja.FunctionCall) goja.Value {
		value := call.Argument(0).Export()
		var data []byte
		if text, ok := value.(string); ok {
			data = []byte(text)
		} else {
			bytes, err := byteValue(value)
			if err != nil {
				panic(vm.NewTypeError("sha256 value must be text or bytes"))
			}
			data = bytes
		}
		digest := sha256.Sum256(data)
		return vm.ToValue(hex.EncodeToString(digest[:]))
	})
	mustSet(vm, np, "hash", hashAPI)

	workspaceNP(vm, np, environment)
	reportsNP(vm, np, environment)
	chatNP(vm, np, environment)
	filesNP(vm, np, environment)
	httpNP(vm, np, environment)
	notifyNP(vm, np, environment)
	return np
}

func workspaceNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	pipelines := vm.NewObject()
	listPipelines := func(goja.FunctionCall) goja.Value {
		values, err := environment.system(vm).ListPipelines(environment.ctx)
		if err != nil {
			panic(vm.NewTypeError("list pipelines: %v", err))
		}
		return vm.ToValue(pipelineSummaries(values))
	}
	mustSet(vm, pipelines, "list", listPipelines)
	mustSet(vm, np, "getPipelines", listPipelines)
	mustSet(vm, pipelines, "get", func(call goja.FunctionCall) goja.Value {
		value, err := environment.system(vm).GetPipeline(environment.ctx, requiredString(vm, call.Argument(0), "pipeline ID"))
		if err != nil {
			panic(vm.NewTypeError("get pipeline: %v", err))
		}
		return vm.ToValue(pipelineSummary(value))
	})
	mustSet(vm, np, "pipelines", pipelines)
	functions := vm.NewObject()
	mustSet(vm, functions, "list", func(goja.FunctionCall) goja.Value {
		values, err := environment.system(vm).ListFunctions(environment.ctx)
		if err != nil {
			panic(vm.NewTypeError("list functions: %v", err))
		}
		return vm.ToValue(functionSummaries(values))
	})
	mustSet(vm, np, "functions", functions)
	triggers := vm.NewObject()
	mustSet(vm, triggers, "list", func(goja.FunctionCall) goja.Value {
		values, err := environment.system(vm).ListTriggers(environment.ctx)
		if err != nil {
			panic(vm.NewTypeError("list triggers: %v", err))
		}
		return vm.ToValue(triggerSummaries(values))
	})
	mustSet(vm, np, "triggers", triggers)
	executions := vm.NewObject()
	mustSet(vm, executions, "list", func(call goja.FunctionCall) goja.Value {
		limit := optionalLimit(vm, call.Argument(0), 20, 100)
		values, err := environment.system(vm).ListExecutions(environment.ctx, limit)
		if err != nil {
			panic(vm.NewTypeError("list executions: %v", err))
		}
		return vm.ToValue(executionSummaries(values))
	})
	mustSet(vm, np, "executions", executions)
}

func reportsNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	reports := vm.NewObject()
	mustSet(vm, reports, "list", func(call goja.FunctionCall) goja.Value {
		limit := optionalLimit(vm, call.Argument(0), 20, 250)
		values, err := environment.system(vm).ListReports(environment.ctx, limit)
		if err != nil {
			panic(vm.NewTypeError("list reports: %v", err))
		}
		return vm.ToValue(reportSummaries(values, false))
	})
	mustSet(vm, reports, "get", func(call goja.FunctionCall) goja.Value {
		value, err := environment.system(vm).GetReport(environment.ctx, requiredString(vm, call.Argument(0), "report ID"))
		if err != nil {
			panic(vm.NewTypeError("get report: %v", err))
		}
		return vm.ToValue(reportSummary(value, true))
	})
	mustSet(vm, reports, "create", func(call goja.FunctionCall) goja.Value {
		object := requiredObject(vm, call.Argument(0), "report")
		title := stringField(vm, object, "title", true)
		markdown := stringField(vm, object, "markdown", true)
		tags := stringListField(vm, object, "tags")
		value, err := environment.system(vm).CreateReport(environment.ctx, environment.invocation.Node.ID, title, markdown, tags)
		if err != nil {
			panic(vm.NewTypeError("create report: %v", err))
		}
		return vm.ToValue(reportSummary(value, true))
	})
	mustSet(vm, np, "reports", reports)
}

func chatNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	chat := vm.NewObject()
	mustSet(vm, chat, "history", func(call goja.FunctionCall) goja.Value {
		values, err := environment.system(vm).ReadChatHistory(environment.ctx, requiredString(vm, call.Argument(0), "chat ID"), optionalLimit(vm, call.Argument(1), 20, 100))
		if err != nil {
			panic(vm.NewTypeError("read chat history: %v", err))
		}
		return vm.ToValue(chatMessages(values))
	})
	mustSet(vm, chat, "reply", func(call goja.FunctionCall) goja.Value {
		value, err := environment.system(vm).AppendChatReply(environment.ctx, requiredString(vm, call.Argument(0), "chat run ID"), requiredString(vm, call.Argument(1), "message"))
		if err != nil {
			panic(vm.NewTypeError("reply to chat: %v", err))
		}
		return vm.ToValue(chatMessage(value))
	})
	mustSet(vm, chat, "setStatus", func(call goja.FunctionCall) goja.Value {
		if err := environment.system(vm).UpdateChatStatus(environment.ctx, requiredString(vm, call.Argument(0), "chat run ID"), requiredString(vm, call.Argument(1), "status")); err != nil {
			panic(vm.NewTypeError("update chat status: %v", err))
		}
		return goja.Undefined()
	})
	mustSet(vm, np, "chat", chat)
}

func filesNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	files := vm.NewObject()
	mustSet(vm, files, "list", func(call goja.FunctionCall) goja.Value {
		values, err := environment.requireHost(vm, domain.CapabilityFileRead).ListDirectory(environment.ctx, requiredString(vm, call.Argument(0), "directory path"))
		if err != nil {
			panic(vm.NewTypeError("list directory: %v", err))
		}
		return vm.ToValue(values)
	})
	mustSet(vm, files, "readBytes", func(call goja.FunctionCall) goja.Value {
		value, err := environment.requireHost(vm, domain.CapabilityFileRead).ReadFile(environment.ctx, requiredString(vm, call.Argument(0), "file path"))
		if err != nil {
			panic(vm.NewTypeError("read file: %v", err))
		}
		return vm.ToValue(value)
	})
	mustSet(vm, files, "readText", func(call goja.FunctionCall) goja.Value {
		value, err := environment.requireHost(vm, domain.CapabilityFileRead).ReadFile(environment.ctx, requiredString(vm, call.Argument(0), "file path"))
		if err != nil {
			panic(vm.NewTypeError("read file: %v", err))
		}
		return vm.ToValue(string(value))
	})
	mustSet(vm, files, "writeBytes", func(call goja.FunctionCall) goja.Value {
		bytes, err := byteValue(call.Argument(1).Export())
		if err != nil {
			panic(vm.NewTypeError("file bytes: %v", err))
		}
		path, err := environment.requireHost(vm, domain.CapabilityFileWrite).WriteFile(environment.ctx, requiredString(vm, call.Argument(0), "file path"), bytes)
		if err != nil {
			panic(vm.NewTypeError("write file: %v", err))
		}
		return vm.ToValue(map[string]any{"path": path, "written": true})
	})
	mustSet(vm, files, "writeText", func(call goja.FunctionCall) goja.Value {
		path, err := environment.requireHost(vm, domain.CapabilityFileWrite).WriteFile(environment.ctx, requiredString(vm, call.Argument(0), "file path"), []byte(requiredString(vm, call.Argument(1), "text")))
		if err != nil {
			panic(vm.NewTypeError("write file: %v", err))
		}
		return vm.ToValue(map[string]any{"path": path, "written": true})
	})
	mustSet(vm, np, "files", files)
}

func httpNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	httpAPI := vm.NewObject()
	mustSet(vm, httpAPI, "request", func(call goja.FunctionCall) goja.Value {
		object := requiredObject(vm, call.Argument(0), "HTTP request")
		request := nodes.JavaScriptHTTPRequest{URL: stringField(vm, object, "url", true), Method: strings.ToUpper(defaultString(stringField(vm, object, "method", false), "GET")), Headers: headersField(vm, object)}
		if body := object.Get("body"); body != nil && !goja.IsUndefined(body) && !goja.IsNull(body) {
			if text, ok := body.Export().(string); ok {
				request.Body = []byte(text)
			} else {
				bytes, err := byteValue(body.Export())
				if err != nil {
					panic(vm.NewTypeError("HTTP body must be text or bytes"))
				}
				request.Body = bytes
			}
		}
		response, err := environment.requireHost(vm, domain.CapabilityNetwork).HTTPRequest(environment.ctx, request)
		if err != nil {
			panic(vm.NewTypeError("HTTP request: %v", err))
		}
		result := map[string]any{"status": response.Status, "headers": response.Headers, "body": string(response.Body)}
		return vm.ToValue(result)
	})
	mustSet(vm, np, "http", httpAPI)
}

func notifyNP(vm *goja.Runtime, np *goja.Object, environment javascriptEnvironment) {
	mustSet(vm, np, "notify", func(call goja.FunctionCall) goja.Value {
		if err := environment.system(vm).Notify(environment.ctx, requiredString(vm, call.Argument(0), "title"), requiredString(vm, call.Argument(1), "message")); err != nil {
			panic(vm.NewTypeError("send notification: %v", err))
		}
		return goja.Undefined()
	})
}

func mustSet(vm *goja.Runtime, object *goja.Object, name string, value any) {
	if err := object.Set(name, value); err != nil {
		panic(fmt.Errorf("define JavaScript API %s: %w", name, err))
	}
}

func requiredString(vm *goja.Runtime, value goja.Value, name string) string {
	text, ok := value.Export().(string)
	if !ok || strings.TrimSpace(text) == "" {
		panic(vm.NewTypeError("%s must be non-empty text", name))
	}
	return strings.TrimSpace(text)
}
func optionalString(value goja.Value, fallback string) string {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return fallback
	}
	text, ok := value.Export().(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fallback
	}
	return strings.TrimSpace(text)
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func optionalLimit(vm *goja.Runtime, value goja.Value, fallback, maximum int) int {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return fallback
	}
	number, ok := integerValue(value.Export())
	if !ok || number < 1 || number > int64(maximum) {
		panic(vm.NewTypeError("limit must be an integer from 1 to %d", maximum))
	}
	return int(number)
}
func requiredObject(vm *goja.Runtime, value goja.Value, name string) *goja.Object {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		panic(vm.NewTypeError("%s must be an object", name))
	}
	object := value.ToObject(vm)
	if object.ClassName() == "Array" {
		panic(vm.NewTypeError("%s must be an object", name))
	}
	return object
}
func stringField(vm *goja.Runtime, object *goja.Object, name string, required bool) string {
	value := object.Get(name)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		if required {
			panic(vm.NewTypeError("%s is required", name))
		}
		return ""
	}
	return requiredString(vm, value, name)
}
func stringListField(vm *goja.Runtime, object *goja.Object, name string) []string {
	value := object.Get(name)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return []string{}
	}
	items, ok := sliceValue(value.Export())
	if !ok {
		panic(vm.NewTypeError("%s must be a list of text", name))
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			panic(vm.NewTypeError("%s must be a list of text", name))
		}
		result = append(result, text)
	}
	return result
}
func headersField(vm *goja.Runtime, object *goja.Object) map[string][]string {
	value := object.Get("headers")
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return map[string][]string{}
	}
	values, ok := objectValue(value.Export())
	if !ok {
		panic(vm.NewTypeError("headers must be an object"))
	}
	result := make(map[string][]string, len(values))
	for name, raw := range values {
		if text, ok := raw.(string); ok {
			result[name] = []string{text}
			continue
		}
		items, ok := sliceValue(raw)
		if !ok {
			panic(vm.NewTypeError("header %s must be text or a list of text", name))
		}
		list := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				panic(vm.NewTypeError("header %s must be text or a list of text", name))
			}
			list = append(list, text)
		}
		result[name] = list
	}
	return result
}

func pipelineSummaries(values []domain.PipelineSummary) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = pipelineSummaryFromSummary(value)
	}
	return result
}
func pipelineSummary(value domain.Pipeline) map[string]any {
	return map[string]any{"id": value.ID, "name": value.Name, "description": value.Description, "status": string(value.Status), "publishedRevision": value.PublishedRevision, "updatedAt": value.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
func pipelineSummaryFromSummary(value domain.PipelineSummary) map[string]any {
	return map[string]any{"id": value.ID, "name": value.Name, "description": value.Description, "status": string(value.Status), "publishedRevision": value.PublishedRevision, "triggerCount": value.TriggerCount, "updatedAt": value.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
func functionSummaries(values []domain.FunctionSummary) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = map[string]any{"id": value.ID, "name": value.Name, "description": value.Description, "category": value.Category, "kind": string(value.Kind), "mode": string(value.Mode), "publishedRevision": value.PublishedRevision, "updatedAt": value.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	}
	return result
}
func triggerSummaries(values []domain.TriggerBinding) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = map[string]any{"id": value.ID, "pipelineId": value.PipelineID, "nodeId": value.NodeID, "kind": string(value.Kind), "label": value.Label, "enabled": value.Enabled, "trusted": value.Trusted}
	}
	return result
}
func executionSummaries(values []domain.Execution) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = map[string]any{"id": value.ID, "pipelineId": value.PipelineID, "triggerId": value.TriggerID, "status": string(value.Status), "startedAt": value.StartedAt.UTC().Format(time.RFC3339Nano), "finishedAt": formatTime(value.FinishedAt)}
	}
	return result
}
func reportSummaries(values []domain.Report, content bool) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = reportSummary(value, content)
	}
	return result
}
func reportSummary(value domain.Report, content bool) map[string]any {
	result := map[string]any{"id": value.ID, "pipelineId": value.PipelineID, "pipelineName": value.PipelineName, "executionId": value.ExecutionID, "nodeId": value.NodeID, "title": value.Title, "tags": value.Tags, "createdAt": value.CreatedAt.UTC().Format(time.RFC3339Nano)}
	if content {
		result["markdown"] = value.Markdown
	}
	return result
}
func chatMessages(values []domain.ChatMessage) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = chatMessage(value)
	}
	return result
}
func chatMessage(value domain.ChatMessage) map[string]any {
	return map[string]any{"id": value.ID, "conversationId": value.ConversationID, "role": string(value.Role), "content": value.Content, "createdAt": value.CreatedAt.UTC().Format(time.RFC3339Nano)}
}
func formatTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}
