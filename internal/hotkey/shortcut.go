// Package hotkey owns global shortcut registration through Wails v3's
// GlobalShortcutManager. The shortcut parser in this file produces the
// canonical accelerator string Wails v3 uses as the registration key, so
// conflict detection and persistence remain stable and independent of the
// Wails version.
package hotkey

import (
	"fmt"
	"strconv"
	"strings"
)

// chord is the parsed form of one configured hotkey. The canonical string is
// the form Wails v3's GlobalShortcutManager.Register accepts and reports in
// GetAll(); it must match the canonical form used by the Wails v3 accelerator
// parser so duplicate detection across persistence and runtime agree.
type chord struct {
	modifiers []string
	key       string
	canonical string
}

// modifierOrder defines the deterministic display order of modifiers in the
// canonical form. Wails v3 sorts modifiers alphabetically when stringifying an
// accelerator, so this slice mirrors that order.
var modifierOrder = []string{"Alt", "Cmd", "Ctrl", "Option", "Shift", "Super", "Win"}

// namedKeys maps the lowercase, user-facing key names to the canonical form
// expected by Wails v3's parseKey. Wails uses lowercase key names internally
// and stringifies them through strings.ToUpper, so a single-letter key like
// "n" canonicalises to "N" and a named key like "f5" canonicalises to "F5".
var namedKeys = map[string]string{
	"space":      "space",
	"enter":      "enter",
	"return":     "return",
	"tab":        "tab",
	"backspace":  "backspace",
	"delete":     "delete",
	"insert":     "insert",
	"home":       "home",
	"end":        "end",
	"pageup":     "page up",
	"pagedown":   "page down",
	"left":       "left",
	"right":      "right",
	"up":         "up",
	"down":       "down",
	"arrowleft":  "left",
	"arrowright": "right",
	"arrowup":    "up",
	"arrowdown":  "down",
	"escape":     "escape",
	"esc":        "escape",
	"numlock":    "numlock",
}

func parseShortcut(value string) (chord, error) {
	parts := strings.Split(value, "+")
	if len(parts) < 2 {
		return chord{}, fmt.Errorf("shortcut %q must include a modifier and another key", value)
	}

	seenModifiers := make(map[string]struct{})
	var modifiers []string
	var keyName string
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return chord{}, fmt.Errorf("shortcut %q contains an empty key", value)
		}
		switch strings.ToLower(part) {
		case "ctrl", "control":
			if _, exists := seenModifiers["Ctrl"]; exists {
				return chord{}, fmt.Errorf("shortcut %q repeats Ctrl", value)
			}
			seenModifiers["Ctrl"] = struct{}{}
			modifiers = append(modifiers, "Ctrl")
			continue
		case "alt", "option", "optionoralt":
			if _, exists := seenModifiers["Alt"]; exists {
				return chord{}, fmt.Errorf("shortcut %q repeats Alt", value)
			}
			seenModifiers["Alt"] = struct{}{}
			modifiers = append(modifiers, "Alt")
			continue
		case "shift":
			if _, exists := seenModifiers["Shift"]; exists {
				return chord{}, fmt.Errorf("shortcut %q repeats Shift", value)
			}
			seenModifiers["Shift"] = struct{}{}
			modifiers = append(modifiers, "Shift")
			continue
		case "meta", "win", "windows", "super":
			if _, exists := seenModifiers["Win"]; exists {
				return chord{}, fmt.Errorf("shortcut %q repeats Win", value)
			}
			seenModifiers["Win"] = struct{}{}
			modifiers = append(modifiers, "Win")
			continue
		case "cmd", "command", "cmdorctrl":
			if _, exists := seenModifiers["Ctrl"]; exists {
				return chord{}, fmt.Errorf("shortcut %q repeats CmdOrCtrl", value)
			}
			seenModifiers["Ctrl"] = struct{}{}
			modifiers = append(modifiers, "Ctrl")
			continue
		}

		if keyName != "" {
			return chord{}, fmt.Errorf("shortcut %q contains more than one non-modifier key", value)
		}
		key, err := shortcutKey(part)
		if err != nil {
			return chord{}, fmt.Errorf("shortcut %q: %w", value, err)
		}
		keyName = key
	}
	if len(modifiers) == 0 || keyName == "" {
		return chord{}, fmt.Errorf("shortcut %q must include a modifier and another key", value)
	}
	return chord{modifiers: modifiers, key: keyName, canonical: canonicalAccelerator(modifiers, keyName)}, nil
}

func shortcutKey(value string) (string, error) {
	if len(value) == 1 {
		letter := value[0]
		if letter >= 'a' && letter <= 'z' {
			letter -= 'a' - 'A'
		}
		if (letter >= 'A' && letter <= 'Z') || (letter >= '0' && letter <= '9') {
			return strings.ToLower(string(letter)), nil
		}
	}
	lower := strings.ToLower(value)
	if key, ok := namedKeys[lower]; ok {
		return key, nil
	}
	if strings.HasPrefix(lower, "f") {
		number, err := strconv.Atoi(strings.TrimPrefix(lower, "f"))
		if err == nil && number >= 1 && number <= 35 {
			return lower, nil
		}
	}
	return "", fmt.Errorf("%q is not a supported key", value)
}

// canonicalAccelerator mirrors Wails v3's accelerator.String() output: the
// modifiers sorted alphabetically, followed by the upper-cased key, joined by
// '+'. This keeps persistence stable across Wails upgrades because the same
// canonical form is used as the persistence key, the Wails v3 registration
// key, and the user-facing display string.
func canonicalAccelerator(modifiers []string, key string) string {
	sorted := append([]string(nil), modifiers...)
	// Alphabetical sort matches Wails v3's slices.Sort on the modifier strings.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	sorted = append(sorted, strings.ToUpper(key))
	return strings.Join(sorted, "+")
}
