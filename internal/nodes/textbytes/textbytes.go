// Package textbytes provides strict, reusable string-or-byte-slice contracts
// for node modules that need an explicit wire representation.
package textbytes

import (
	"fmt"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

// Representation is the selected wire representation.
type Representation string

const (
	// Text represents a valid UTF-8 Go string.
	Text Representation = "text"
	// Bytes represents a byte slice without text interpretation.
	Bytes Representation = "bytes"
)

// Resolve validates the representation stored at key and applies the default
// when the configuration does not provide one. It never coerces a non-string
// setting, keeping persisted node contracts unambiguous.
func Resolve(config, defaults map[string]any, key string) (Representation, error) {
	value, exists := config[key]
	if !exists {
		value = defaults[key]
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be text", key)
	}
	representation := Representation(text)
	if representation != Text && representation != Bytes {
		return "", fmt.Errorf("%s %q is unsupported", key, text)
	}
	return representation, nil
}

// Pin creates a data pin with the exact TypeSpec for representation. Bytes is
// distinct from text even though the legacy presentation data type is any.
func Pin(id, label string, direction domain.PinDirection, representation Representation, required bool) domain.NodePort {
	dataType, typeSpec, color := pinType(representation)
	return domain.NodePort{
		ID:             id,
		Label:          label,
		Kind:           domain.PinData,
		Direction:      direction,
		DataType:       dataType,
		Type:           &typeSpec,
		Color:          color,
		Required:       required,
		MaxConnections: 1,
	}
}

// InputBytes validates value against representation and returns the byte
// sequence used by the caller's encoding or filesystem operation.
func InputBytes(value any, representation Representation) ([]byte, error) {
	switch representation {
	case Text:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("value must be text")
		}
		if !utf8.ValidString(text) {
			return nil, fmt.Errorf("text value must be valid UTF-8")
		}
		return []byte(text), nil
	case Bytes:
		data, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("value must be bytes")
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported representation %q", representation)
	}
}

// OutputValue validates and returns data in representation. Requesting text
// for binary data fails rather than passing invalid UTF-8 through a string pin.
func OutputValue(data []byte, representation Representation) (any, error) {
	switch representation {
	case Text:
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("output is not valid UTF-8; select Bytes")
		}
		return string(data), nil
	case Bytes:
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported representation %q", representation)
	}
}

// Options lists the representations for the shared styled select primitive.
func Options() []domain.Option {
	return []domain.Option{{Value: string(Text), Label: "Text"}, {Value: string(Bytes), Label: "Bytes"}}
}

func pinType(representation Representation) (domain.DataType, domain.TypeSpec, string) {
	if representation == Text {
		return domain.DataText, typespec.String(), "#e879f9"
	}
	return domain.DataAny, domain.TypeSpec{Kind: domain.TypeBytes}, "#a1a1aa"
}
