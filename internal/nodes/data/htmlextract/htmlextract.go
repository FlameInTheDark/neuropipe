// Package htmlextract registers the CSS selector HTML extraction node.
package htmlextract

import (
	"context"
	"fmt"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

// Return modes describing what each matched element contributes.
const (
	ReturnText      = "text"
	ReturnHTML      = "html"
	ReturnAttribute = "attribute"
)

// Extraction is one configured CSS selector query and its output pin.
type Extraction struct {
	ID        string
	Label     string
	Selector  string
	Mode      string
	Attribute string
	ReturnAll bool
}

// DataType reports the default pin type for the extraction. Every extraction
// returns plain text or a list of texts so results compose with every other
// node without a custom wire type.
func (e Extraction) DataType() domain.DataType {
	if e.ReturnAll {
		return domain.DataList
	}
	return domain.DataText
}

func Register(registrar nodes.Registrar) error {
	definition := definition()
	return registrar.Register(Node{
		Metadata: definition,
		Resolver: func(node domain.FlowNode) (domain.NodeDefinition, error) {
			return ResolveDefinition(definition, node)
		},
		Executor: nodes.Outputs(Evaluate),
	})
}

func definition() domain.NodeDefinition {
	defaults := []any{map[string]any{"id": "value", "label": "Value", "selector": "", "mode": ReturnText, "attribute": "", "returnAll": false}}
	return datanodes.Node(
		"data:html_extract",
		"Data",
		"HTML Extract",
		"Extract values from an HTML document with CSS selectors. Every configured query creates its own output pin.",
		"file-code",
		[]domain.NodePort{datanodes.Pin("html", "HTML", domain.PinInput, domain.DataText)},
		nil,
		[]domain.ConfigField{{Name: "extractions", Label: "Extractions", Kind: "html-extractions", Required: true}},
		map[string]any{"extractions": defaults},
	)
}

// ResolveDefinition expands one output pin per configured extraction.
func ResolveDefinition(definition domain.NodeDefinition, node domain.FlowNode) (domain.NodeDefinition, error) {
	extractions, err := ExtractionsFor(config(node), definition.DefaultConfig)
	if err != nil {
		return definition, err
	}
	resolved := definition
	resolved.Outputs = make([]domain.NodePort, 0, len(extractions))
	for _, extraction := range extractions {
		resolved.Outputs = append(resolved.Outputs, datanodes.Pin(extraction.ID, extraction.Label, domain.PinOutput, extraction.DataType()))
	}
	return resolved, nil
}

// Evaluate parses the document once and resolves every configured query.
func Evaluate(ctx context.Context, invocation nodes.Invocation, _ nodes.Runtime) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("html extract cancelled: %w", err)
	}
	document, err := htmlInput(invocation.Inputs)
	if err != nil {
		return nil, err
	}
	extractions, err := ExtractionsFor(invocation.Config, invocation.Definition.DefaultConfig)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(extractions))
	for _, extraction := range extractions {
		selector, err := cascadia.Compile(extraction.Selector)
		if err != nil {
			return nil, fmt.Errorf("html extract output %q has invalid CSS selector: %w", extraction.Label, err)
		}
		matches := selector.MatchAll(document)
		if extraction.ReturnAll {
			values := make([]any, 0, len(matches))
			for _, node := range matches {
				values = append(values, extractValue(node, extraction))
			}
			result[extraction.ID] = values
			continue
		}
		value := ""
		if len(matches) > 0 {
			value = extractValue(matches[0], extraction)
		}
		result[extraction.ID] = value
	}
	return result, nil
}

func extractValue(node *html.Node, extraction Extraction) string {
	switch extraction.Mode {
	case ReturnHTML:
		var builder strings.Builder
		if err := html.Render(&builder, node); err != nil {
			return ""
		}
		return builder.String()
	case ReturnAttribute:
		for _, attribute := range node.Attr {
			if attribute.Key == extraction.Attribute {
				return attribute.Val
			}
		}
		return ""
	default:
		var builder strings.Builder
		writeTextContent(&builder, node)
		return builder.String()
	}
}

func writeTextContent(builder *strings.Builder, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
			continue
		}
		if child.Type == html.ElementNode {
			writeTextContent(builder, child)
		}
	}
}

func htmlInput(inputs map[string]any) (*html.Node, error) {
	source, ok := inputs["html"].(string)
	if !ok {
		return nil, fmt.Errorf("html must be text")
	}
	document, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return document, nil
}

// ExtractionsFor parses and validates the node's extraction configuration.
func ExtractionsFor(config, defaults map[string]any) ([]Extraction, error) {
	configured, exists := config["extractions"]
	if !exists {
		configured = defaults["extractions"]
	}
	items, ok := configured.([]any)
	if !ok {
		return nil, fmt.Errorf("extractions must be a list")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("add at least one extraction")
	}
	extractions := make([]Extraction, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("extraction %d must be an object", index+1)
		}
		id := fieldText(item, "id")
		if id == "" {
			return nil, fmt.Errorf("extraction %d needs an ID", index+1)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("extractions contains duplicate ID %q", id)
		}
		seen[id] = struct{}{}
		selector := fieldText(item, "selector")
		if selector == "" {
			return nil, fmt.Errorf("extraction %q needs a CSS selector", id)
		}
		mode := fieldText(item, "mode")
		switch mode {
		case "", ReturnText:
			mode = ReturnText
		case ReturnHTML, ReturnAttribute:
		default:
			return nil, fmt.Errorf("extraction %q has unsupported return mode %q", id, mode)
		}
		attribute := fieldText(item, "attribute")
		if mode == ReturnAttribute && attribute == "" {
			return nil, fmt.Errorf("extraction %q needs an attribute name to return", id)
		}
		label := fieldText(item, "label")
		if label == "" {
			label = id
		}
		returnAll, _ := item["returnAll"].(bool)
		extractions = append(extractions, Extraction{ID: id, Label: label, Selector: selector, Mode: mode, Attribute: attribute, ReturnAll: returnAll})
	}
	return extractions, nil
}

// fieldText reads a string field, treating missing values as empty instead of
// the "<nil>" produced by printing an absent map entry.
func fieldText(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}

// config returns the node's persisted V3 configuration.
func config(node domain.FlowNode) map[string]any {
	value, _ := node.Data["config"].(map[string]any)
	return value
}
