package kv

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// maxNormalizedStrings caps how many collection elements a normalised reply
// keeps. It mirrors the SQL node's 500-row cap so node-run records and the
// packet stay bounded.
const maxNormalizedStrings = 500

// maxNormalizedStringBytes caps a single string value so a multi-megabyte
// blob cannot blow up JSON serialization of node runs or UI rendering.
const maxNormalizedStringBytes = 64 * 1024

var commandPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// dangerousCommands are administratively destructive or security-relevant
// operations. They are rejected on the node path unless the node explicitly
// opts in, and on the console path only after an explicit confirmation.
var dangerousCommands = map[string]struct{}{
	"FLUSHALL": {}, "FLUSHDB": {}, "CONFIG": {}, "SHUTDOWN": {}, "DEBUG": {},
	"SCRIPT": {}, "ACL": {}, "CLIENT": {}, "CLUSTER": {}, "SLAVEOF": {},
	"REPLICAOF": {}, "MODULE": {}, "SAVE": {}, "BGSAVE": {}, "BGREWRITEAOF": {},
	"FAILOVER": {}, "RESET": {}, "SWAPDB": {}, "MIGRATE": {}, "RESTORE": {},
}

// normalizeCommand trims and upper-cases a command word.
func normalizeCommand(command string) string {
	return strings.ToUpper(strings.TrimSpace(command))
}

// validateCommand checks one Redis command word and reports whether it is
// allowed on this path. Errors deliberately name only the command, never the
// arguments, so logs cannot leak payload data.
func validateCommand(command string, allowDangerous bool) error {
	if !commandPattern.MatchString(command) {
		return fmt.Errorf("invalid redis command %q", command)
	}
	if _, blocked := dangerousCommands[command]; blocked && !allowDangerous {
		return fmt.Errorf("redis command %s is disabled for this node (enable \"Allow dangerous commands\" to run it)", command)
	}
	return nil
}

// dangerousCommand reports whether a command word is on the denylist. The
// console uses it to demand an explicit confirmation before executing.
func dangerousCommand(command string) bool {
	_, blocked := dangerousCommands[command]
	return blocked
}

// DangerousCommands returns the denylist in stable order for display.
func DangerousCommands() []string {
	result := make([]string, 0, len(dangerousCommands))
	for command := range dangerousCommands {
		result = append(result, command)
	}
	sort.Strings(result)
	return result
}

// normalizeReply converts a go-redis reply into JSON-safe values: strings,
// booleans, int64 (float64 beyond the JavaScript safe integer range), and
// nested lists/maps of those. The second return reports truncation.
func normalizeReply(value any) (any, bool) {
	return normalizeValue(value, newBudget())
}

type budget struct {
	elements int
	strings  int
}

func newBudget() *budget {
	return &budget{elements: maxNormalizedStrings, strings: maxNormalizedStrings}
}

func normalizeValue(value any, remaining *budget) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		return truncateString(typed)
	case bool:
		return typed, false
	case []byte:
		return truncateString(string(typed))
	case int:
		return jsSafeInt(int64(typed)), false
	case int32:
		return jsSafeInt(int64(typed)), false
	case int64:
		return jsSafeInt(typed), false
	case float32, float64:
		return value, false
	case []any:
		truncated := false
		if len(typed) > remaining.elements {
			typed = typed[:remaining.elements]
			truncated = true
		}
		remaining.elements -= len(typed)
		result := make([]any, len(typed))
		for index, item := range typed {
			normalized, itemTruncated := normalizeValue(item, remaining)
			result[index] = normalized
			truncated = truncated || itemTruncated
		}
		return result, truncated
	case map[string]any:
		truncated := false
		if len(typed) > remaining.elements {
			remaining.elements = 0
			truncated = true
		} else {
			remaining.elements -= len(typed)
		}
		result := make(map[string]any, len(typed))
		count := 0
		for key, item := range typed {
			if count >= maxNormalizedStrings {
				break
			}
			normalized, itemTruncated := normalizeValue(item, remaining)
			result[key] = normalized
			truncated = truncated || itemTruncated
			count++
		}
		return result, truncated
	case map[any]any:
		// RESP3 map replies from the generic Do path arrive with any-typed keys.
		truncated := false
		if len(typed) > remaining.elements {
			remaining.elements = 0
			truncated = true
		} else {
			remaining.elements -= len(typed)
		}
		result := make(map[string]any, len(typed))
		count := 0
		for key, item := range typed {
			if count >= maxNormalizedStrings {
				break
			}
			normalized, itemTruncated := normalizeValue(item, remaining)
			result[fmt.Sprintf("%v", key)] = normalized
			truncated = truncated || itemTruncated
			count++
		}
		return result, truncated
	case map[string]string:
		truncated := false
		if len(typed) > remaining.elements {
			remaining.elements = 0
			truncated = true
		} else {
			remaining.elements -= len(typed)
		}
		result := make(map[string]any, len(typed))
		count := 0
		for key, item := range typed {
			if count >= maxNormalizedStrings {
				break
			}
			normalized, itemTruncated := truncateString(item)
			result[key] = normalized
			truncated = truncated || itemTruncated
			count++
		}
		return result, truncated
	case fmt.Stringer:
		return truncateString(typed.String())
	default:
		return truncateString(fmt.Sprintf("%v", typed))
	}
}

func truncateString(value string) (any, bool) {
	if len(value) <= maxNormalizedStringBytes {
		return value, false
	}
	return value[:maxNormalizedStringBytes], true
}

// jsSafeInt mirrors the SQL row conversion: integers beyond JavaScript's
// safe range are widened to float64 instead of silently losing precision.
func jsSafeInt(value int64) any {
	if value > 1<<53 || value < -(1<<53) {
		return float64(value)
	}
	return value
}
