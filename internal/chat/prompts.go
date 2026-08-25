package chat

import (
	_ "embed"
	"fmt"
	"strings"
)

// Model-facing authoring documentation. These guides are embedded in the
// binary and deliberately live outside the user-facing Documentation service:
// they describe internal graph structure for the assistant only.

//go:embed prompts/authoring.md
var authoringGuide string

//go:embed prompts/functions.md
var functionsGuide string

// guideSections lists the selectable sections of the embedded authoring docs.
func guideSections() []string {
	return []string{"authoring", "functions"}
}

// guide returns one embedded documentation section for the assistant.
func guide(section string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(section)) {
	case "", "authoring", "pipelines":
		return authoringGuide, nil
	case "functions":
		return functionsGuide, nil
	default:
		return "", fmt.Errorf("unknown guide section %q (available: %s)", section, strings.Join(guideSections(), ", "))
	}
}
