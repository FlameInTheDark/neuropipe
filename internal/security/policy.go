// Package security contains the execution policy and secret-handling boundaries.
package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/catalog"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/persistence"
)

// ApprovalRequiredError prevents unattended work outside an explicitly trusted scope.
type ApprovalRequiredError struct {
	Capability domain.Capability
}

func (e ApprovalRequiredError) Error() string {
	return fmt.Sprintf("approval required for %s", e.Capability)
}

// RevisionGate evaluates permission grants for exactly one immutable pipeline revision.
type RevisionGate struct {
	store      *persistence.Store
	pipelineID string
	revision   int
	unattended bool
}

// NewRevisionGate creates a scope-aware gate for one execution.
func NewRevisionGate(store *persistence.Store, pipelineID string, revision int, unattended bool) *RevisionGate {
	return &RevisionGate{store: store, pipelineID: pipelineID, revision: revision, unattended: unattended}
}

// Allow considers an explicit button press approval for manual runs, while unattended
// triggers must have persisted revision-scoped trust for every sensitive capability.
func (g *RevisionGate) Allow(ctx context.Context, _ domain.FlowNode, capabilities []domain.Capability) error {
	if !g.unattended || len(capabilities) == 0 {
		return nil
	}
	for _, capability := range capabilities {
		granted, err := g.store.HasGrant(ctx, g.pipelineID, g.revision, capability)
		if err != nil {
			return err
		}
		if !granted {
			return ApprovalRequiredError{Capability: capability}
		}
	}
	return nil
}

// RequiredCapabilities lists unique capability requests in presentation order.
func RequiredCapabilities(definition domain.FlowDefinition, registry *catalog.Registry) []domain.Capability {
	seen := make(map[domain.Capability]struct{})
	result := make([]domain.Capability, 0)
	for _, node := range definition.Nodes {
		metadata, exists := registry.Get(node.Type)
		if !exists {
			continue
		}
		// First-party modules may derive their sensitive capabilities from a
		// validated configuration (for example JavaScript's explicit np access
		// switches). Keep trust prompts and runtime authorization aligned with
		// the exact node definition the Blueprint engine will execute.
		if module, moduleExists := registry.Node(node.Type); moduleExists {
			if resolved, err := module.Resolve(node); err == nil {
				metadata = resolved
			}
		}
		for _, capability := range metadata.Capabilities {
			if _, exists := seen[capability]; exists {
				continue
			}
			seen[capability] = struct{}{}
			result = append(result, capability)
		}
	}
	return result
}

// Redact removes likely secret values from execution logs before persistence.
func Redact(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, nested := range typed {
			if looksSecret(key) {
				result[key] = "[redacted]"
				continue
			}
			result[key] = Redact(nested)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, nested := range typed {
			result[index] = Redact(nested)
		}
		return result
	default:
		return value
	}
}

func looksSecret(key string) bool {
	key = strings.ToLower(key)
	for _, marker := range []string{"secret", "token", "password", "api_key", "apikey", "authorization"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
