// Package typespec implements Blueprint V3's strict, Go-inspired wire types.
package typespec

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func Any() domain.TypeSpec    { return domain.TypeSpec{Kind: domain.TypeAny} }
func Bool() domain.TypeSpec   { return domain.TypeSpec{Kind: domain.TypeBool} }
func String() domain.TypeSpec { return domain.TypeSpec{Kind: domain.TypeString} }
func Int() domain.TypeSpec    { return domain.TypeSpec{Kind: domain.TypeInt} }
func Float() domain.TypeSpec  { return domain.TypeSpec{Kind: domain.TypeFloat} }

// ValidateSpec rejects malformed contracts before they become a dynamic pin's
// public type. This keeps persisted graph configuration JSON-safe and makes
// Type Assert fail at validation time instead of weakening a wire to any.
func ValidateSpec(spec domain.TypeSpec) error {
	return validateSpec(spec, "type")
}

func validateSpec(spec domain.TypeSpec, path string) error {
	switch spec.Kind {
	case domain.TypeAny, domain.TypeBool, domain.TypeString, domain.TypeInt, domain.TypeFloat, domain.TypeBytes:
		return nil
	case domain.TypeList:
		if spec.Element == nil {
			return fmt.Errorf("%s list needs an element type", path)
		}
		return validateSpec(*spec.Element, path+".element")
	case domain.TypeMap:
		if spec.Key == nil || spec.Value == nil {
			return fmt.Errorf("%s map needs key and value types", path)
		}
		if !comparableMapKey(spec.Key.Kind) {
			return fmt.Errorf("%s map key type %q is not supported", path, spec.Key.Kind)
		}
		if err := validateSpec(*spec.Key, path+".key"); err != nil {
			return err
		}
		return validateSpec(*spec.Value, path+".value")
	case domain.TypeRecord:
		seen := make(map[string]struct{}, len(spec.Fields))
		for index, field := range spec.Fields {
			name := field.Name
			if name == "" {
				name = field.ID
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("%s record field %d needs an ID or name", path, index+1)
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("%s record has duplicate field %q", path, name)
			}
			seen[name] = struct{}{}
			if err := validateSpec(field.Type, path+"."+name); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s has unsupported kind %q", path, spec.Kind)
	}
}

func comparableMapKey(kind domain.TypeKind) bool {
	return kind == domain.TypeBool || kind == domain.TypeString || kind == domain.TypeInt || kind == domain.TypeFloat
}

// FromDataType is the deterministic V2-to-V3 mapping used while migrating
// catalog metadata. Number becomes float because V2 JSON numbers are decoded
// as float64; V3 modules may declare int explicitly where required.
func FromDataType(dataType domain.DataType) domain.TypeSpec {
	switch dataType {
	case domain.DataText:
		return String()
	case domain.DataNumber:
		return Float()
	case domain.DataBoolean:
		return Bool()
	case domain.DataObject:
		return domain.TypeSpec{Kind: domain.TypeRecord}
	case domain.DataList:
		element := Any()
		return domain.TypeSpec{Kind: domain.TypeList, Element: &element}
	default:
		return Any()
	}
}

// Assignable reports whether a source pin may connect to a target pin. It
// deliberately mirrors Go assignment rather than Go conversion: numeric and
// textual values are never silently widened or parsed.
func Assignable(source, target domain.TypeSpec) bool {
	if target.Kind == domain.TypeAny {
		return true
	}
	if source.Kind == domain.TypeAny || source.Kind != target.Kind {
		return false
	}
	switch target.Kind {
	case domain.TypeList:
		return source.Element != nil && target.Element != nil && Assignable(*source.Element, *target.Element)
	case domain.TypeMap:
		return source.Key != nil && source.Value != nil && target.Key != nil && target.Value != nil &&
			Assignable(*source.Key, *target.Key) && Assignable(*target.Key, *source.Key) &&
			Assignable(*source.Value, *target.Value) && Assignable(*target.Value, *source.Value)
	case domain.TypeRecord:
		if target.Name != "" {
			return source.Name == target.Name
		}
		for _, wanted := range target.Fields {
			actual, found := recordField(source.Fields, wanted.Name)
			if !found {
				if wanted.Optional {
					continue
				}
				return false
			}
			if !Assignable(actual.Type, wanted.Type) {
				return false
			}
		}
	}
	return true
}

func recordField(fields []domain.TypeFieldSpec, name string) (domain.TypeFieldSpec, bool) {
	for _, field := range fields {
		if field.Name == name || field.ID == name {
			return field, true
		}
	}
	return domain.TypeFieldSpec{}, false
}

// ValidateValue guards every resolved data input. Reflection lets native Go
// producers use named structs, maps, slices, and pointers without weakening
// the declared wire contract.
func ValidateValue(value any, target domain.TypeSpec) error {
	if target.Kind == "" || target.Kind == domain.TypeAny {
		return nil
	}
	return validate(reflect.ValueOf(value), target, "value")
}

func validate(value reflect.Value, target domain.TypeSpec, path string) error {
	// `any` accepts every value, including nil. Short-circuit before the
	// nil-unwrapping loop below, which would otherwise reject nil values
	// nested inside structured contracts (e.g. a SQL row's NULL column
	// flowing into a list<map<string, any>> pin).
	if target.Kind == domain.TypeAny {
		return nil
	}
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return fmt.Errorf("%s is nil, need %s", path, target.Kind)
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return fmt.Errorf("%s is nil, need %s", path, target.Kind)
	}
	switch target.Kind {
	case domain.TypeBool:
		if value.Kind() != reflect.Bool {
			return typeError(path, target, value)
		}
	case domain.TypeString:
		if value.Kind() != reflect.String {
			return typeError(path, target, value)
		}
	case domain.TypeInt:
		if value.Kind() < reflect.Int || value.Kind() > reflect.Int64 {
			return typeError(path, target, value)
		}
	case domain.TypeFloat:
		// JSON has a single number domain: runtime values may be held in any Go
		// int, uint, or float kind, and all satisfy a float contract. The strict
		// int-pin → float-pin connection rule lives in Assignable, not here —
		// this check validates a value against a declared number contract.
		isInt := value.Kind() >= reflect.Int && value.Kind() <= reflect.Int64
		isUint := value.Kind() >= reflect.Uint && value.Kind() <= reflect.Uint64
		isFloat := value.Kind() == reflect.Float32 || value.Kind() == reflect.Float64
		if !isInt && !isUint && !isFloat {
			return typeError(path, target, value)
		}
		if isFloat && (math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0)) {
			return fmt.Errorf("%s must be finite", path)
		}
	case domain.TypeBytes:
		if value.Kind() != reflect.Slice || value.Type().Elem().Kind() != reflect.Uint8 {
			return typeError(path, target, value)
		}
	case domain.TypeList:
		if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
			return typeError(path, target, value)
		}
		if target.Element == nil {
			return fmt.Errorf("%s list contract has no element type", path)
		}
		for index := 0; index < value.Len(); index++ {
			if err := validate(value.Index(index), *target.Element, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case domain.TypeMap:
		if value.Kind() != reflect.Map || target.Key == nil || target.Value == nil {
			return typeError(path, target, value)
		}
		for _, key := range value.MapKeys() {
			if err := validate(key, *target.Key, path+" key"); err != nil {
				return err
			}
			if err := validate(value.MapIndex(key), *target.Value, path+" value"); err != nil {
				return err
			}
		}
	case domain.TypeRecord:
		if value.Kind() != reflect.Struct && (value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String) {
			return typeError(path, target, value)
		}
		if target.Name != "" && value.Type().Name() != target.Name {
			return fmt.Errorf("%s has named Go type %q, need record %q", path, value.Type().Name(), target.Name)
		}
		for _, field := range target.Fields {
			item, found := valueField(value, field.Name)
			if !found {
				if field.Optional {
					continue
				}
				return fmt.Errorf("%s is missing required field %q", path, field.Name)
			}
			if err := validate(item, field.Type, path+"."+field.Name); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s has unsupported type kind %q", path, target.Kind)
	}
	return nil
}

func valueField(value reflect.Value, name string) (reflect.Value, bool) {
	if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
		key := reflect.New(value.Type().Key()).Elem()
		key.SetString(name)
		item := value.MapIndex(key)
		return item, item.IsValid()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	for index := 0; index < value.NumField(); index++ {
		field := value.Type().Field(index)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.PkgPath == "" && (field.Name == name || jsonName == name) {
			return value.Field(index), true
		}
	}
	return reflect.Value{}, false
}

func typeError(path string, target domain.TypeSpec, value reflect.Value) error {
	return fmt.Errorf("%s has Go type %s, need %s", path, value.Type(), target.Kind)
}
