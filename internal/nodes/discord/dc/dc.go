// Package dc provides shared pin, field, and runtime helpers for first-party
// Discord node modules. It contains no execution logic of its own so every
// node package keeps owning its semantics.
package dc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

var (
	textType = domain.TypeSpec{Kind: domain.TypeString}
	boolType = domain.TypeSpec{Kind: domain.TypeBool}
)

// Exec builds the control-flow pin used by every impure Discord node.
func Exec(id, label string, direction domain.PinDirection) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinExec, Direction: direction, Color: "#fafafa", MaxConnections: 1}
}

// Text builds a string data pin.
func Text(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataText, Type: &textType, Color: "#5865f2", Required: required, MaxConnections: 1}
}

// Bool builds a boolean data pin.
func Bool(id, label string, direction domain.PinDirection, required bool) domain.NodePort {
	return domain.NodePort{ID: id, Label: label, Kind: domain.PinData, Direction: direction, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", Required: required, MaxConnections: 1}
}

// IdentityField is the bot-identity picker shared by every Discord node.
func IdentityField() domain.ConfigField {
	return domain.ConfigField{Name: "identityId", Label: "Bot identity", Kind: "discord-identity"}
}

// Definition assembles the common NodeDefinition skeleton for Discord nodes.
func Definition(nodeType, label, description, icon string, inputs []domain.NodePort, outputs []domain.NodePort, fields []domain.ConfigField, defaults map[string]any) domain.NodeDefinition {
	allFields := append([]domain.ConfigField{IdentityField()}, fields...)
	if defaults == nil {
		defaults = map[string]any{}
	}
	defaults["identityId"] = ""
	return domain.NodeDefinition{
		Type: nodeType, Category: "Discord", Label: label, Description: description,
		Icon: icon, Color: "#5865f2", Mode: domain.NodeImpure, PortContractOwned: true,
		Capabilities: []domain.Capability{domain.CapabilityNetwork},
		Inputs:       inputs, Outputs: outputs, Fields: allFields,
		DefaultConfig: defaults, Source: "builtin",
	}
}

// Sender resolves the Discord capability from the runtime. Nodes never import
// the gateway service or discordgo; they only pass typed requests.
func Sender(runtime nodes.Runtime) (nodes.DiscordSender, error) {
	provider, ok := runtime.(nodes.DiscordSenderProvider)
	if !ok || provider.DiscordSender() == nil {
		return nil, fmt.Errorf("discord delivery is unavailable")
	}
	return provider.DiscordSender(), nil
}

// String reads one string pin value (wired or config fallback). Numeric
// values are coerced because entity IDs (messages, channels, users) are
// frequently wired from number-producing nodes or extracted from JSON
// payloads; formatting avoids scientific notation so the result stays a
// plausible ID string instead of being silently dropped.
func String(invocation nodes.Invocation, pinID string) string {
	switch value := invocation.Inputs[pinID].(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

// BoolValue reads one boolean pin value (wired or config fallback).
func BoolValue(invocation nodes.Invocation, pinID string) bool {
	value, _ := invocation.Inputs[pinID].(bool)
	return value
}

// RequiredString reads one string pin and errors when it is empty.
func RequiredString(invocation nodes.Invocation, pinID, label string) (string, error) {
	value := String(invocation, pinID)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}
