// Package hotkey owns global shortcut registration independently of Wails.
package hotkey

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	modifierAlt     uint32 = 0x0001
	modifierControl uint32 = 0x0002
	modifierShift   uint32 = 0x0004
	modifierMeta    uint32 = 0x0008
)

type chord struct {
	modifiers uint32
	key       uint32
	canonical string
}

var namedKeys = map[string]struct {
	canonical string
	key       uint32
}{
	"space":      {canonical: "Space", key: 0x20},
	"enter":      {canonical: "Enter", key: 0x0D},
	"tab":        {canonical: "Tab", key: 0x09},
	"backspace":  {canonical: "Backspace", key: 0x08},
	"delete":     {canonical: "Delete", key: 0x2E},
	"insert":     {canonical: "Insert", key: 0x2D},
	"home":       {canonical: "Home", key: 0x24},
	"end":        {canonical: "End", key: 0x23},
	"pageup":     {canonical: "PageUp", key: 0x21},
	"pagedown":   {canonical: "PageDown", key: 0x22},
	"arrowleft":  {canonical: "ArrowLeft", key: 0x25},
	"arrowup":    {canonical: "ArrowUp", key: 0x26},
	"arrowright": {canonical: "ArrowRight", key: 0x27},
	"arrowdown":  {canonical: "ArrowDown", key: 0x28},
	"escape":     {canonical: "Escape", key: 0x1B},
}

func parseShortcut(value string) (chord, error) {
	parts := strings.Split(value, "+")
	if len(parts) < 2 {
		return chord{}, fmt.Errorf("shortcut %q must include a modifier and another key", value)
	}

	var modifiers uint32
	var modifierNames []string
	var keyName string
	var virtualKey uint32
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return chord{}, fmt.Errorf("shortcut %q contains an empty key", value)
		}
		switch strings.ToLower(part) {
		case "ctrl", "control":
			if modifiers&modifierControl != 0 {
				return chord{}, fmt.Errorf("shortcut %q repeats Ctrl", value)
			}
			modifiers |= modifierControl
			modifierNames = append(modifierNames, "Ctrl")
			continue
		case "alt":
			if modifiers&modifierAlt != 0 {
				return chord{}, fmt.Errorf("shortcut %q repeats Alt", value)
			}
			modifiers |= modifierAlt
			modifierNames = append(modifierNames, "Alt")
			continue
		case "shift":
			if modifiers&modifierShift != 0 {
				return chord{}, fmt.Errorf("shortcut %q repeats Shift", value)
			}
			modifiers |= modifierShift
			modifierNames = append(modifierNames, "Shift")
			continue
		case "meta", "win", "windows":
			if modifiers&modifierMeta != 0 {
				return chord{}, fmt.Errorf("shortcut %q repeats Meta", value)
			}
			modifiers |= modifierMeta
			modifierNames = append(modifierNames, "Meta")
			continue
		}

		if keyName != "" {
			return chord{}, fmt.Errorf("shortcut %q contains more than one non-modifier key", value)
		}
		name, key, err := shortcutKey(part)
		if err != nil {
			return chord{}, fmt.Errorf("shortcut %q: %w", value, err)
		}
		keyName, virtualKey = name, key
	}
	if modifiers == 0 || keyName == "" {
		return chord{}, fmt.Errorf("shortcut %q must include a modifier and another key", value)
	}
	return chord{modifiers: modifiers, key: virtualKey, canonical: strings.Join(append(modifierNames, keyName), "+")}, nil
}

func shortcutKey(value string) (string, uint32, error) {
	if len(value) == 1 {
		letter := value[0]
		if letter >= 'a' && letter <= 'z' {
			letter -= 'a' - 'A'
		}
		if (letter >= 'A' && letter <= 'Z') || (letter >= '0' && letter <= '9') {
			return string(letter), uint32(letter), nil
		}
	}
	if key, ok := namedKeys[strings.ToLower(value)]; ok {
		return key.canonical, key.key, nil
	}
	upper := strings.ToUpper(value)
	if strings.HasPrefix(upper, "F") {
		function, err := strconv.Atoi(strings.TrimPrefix(upper, "F"))
		if err == nil && function >= 1 && function <= 24 {
			return upper, 0x70 + uint32(function-1), nil
		}
	}
	return "", 0, fmt.Errorf("%q is not a supported key", value)
}
