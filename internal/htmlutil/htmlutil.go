// Package htmlutil provides shared HTML document cleaning used by the HTTP
// Request node and HTML extraction modules.
package htmlutil

import (
	"strings"

	"golang.org/x/net/html"
)

// Clean removes script and style scaffolding from an HTML document. Scripts
// cover `script` and `noscript` elements; styles cover `style` elements and
// `link rel="stylesheet"` references. The source is returned unchanged when
// neither strip flag is set, so callers can apply the flags unconditionally.
func Clean(source string, stripScripts, stripStyles bool) (string, error) {
	if !stripScripts && !stripStyles {
		return source, nil
	}
	document, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	remove := func(tag string, stylesheetOnly bool) {
		var walk func(node *html.Node)
		walk = func(node *html.Node) {
			var next *html.Node
			for child := node.FirstChild; child != nil; child = next {
				next = child.NextSibling
				walk(child)
			}
			if node.Type != html.ElementNode || node.Data != tag {
				return
			}
			if stylesheetOnly && !hasStylesheetRel(node) {
				return
			}
			if node.Parent != nil {
				node.Parent.RemoveChild(node)
			}
		}
		walk(document)
	}
	if stripScripts {
		remove("script", false)
		remove("noscript", false)
	}
	if stripStyles {
		remove("style", false)
		remove("link", true)
	}
	var builder strings.Builder
	if err := html.Render(&builder, document); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func hasStylesheetRel(node *html.Node) bool {
	for _, attribute := range node.Attr {
		if !strings.EqualFold(attribute.Key, "rel") {
			continue
		}
		for _, relation := range strings.Fields(attribute.Val) {
			if strings.EqualFold(relation, "stylesheet") {
				return true
			}
		}
	}
	return false
}
