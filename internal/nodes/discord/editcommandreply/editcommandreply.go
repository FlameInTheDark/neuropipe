// Package editcommandreply registers the Edit Command Reply action node:
// it edits the original reply to an application command (or any followup
// message by id) while the interaction token is valid.
package editcommandreply

import (
	"context"

	"github.com/FlameInTheDark/neuropipe/internal/discordspec"
	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/dc"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/sendmessage"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	return dc.Definition("action:discord_edit_command_reply", "Edit Command Reply", "Change the replied message of an application command: edit the original reply, or any followup message by its ID.", "pencil",
		[]domain.NodePort{
			dc.Exec("in", "Edit", domain.PinInput),
			interactionPort(),
			dc.Text("messageId", "Message ID", domain.PinInput, false),
			dc.Text("message", "Message", domain.PinInput, false),
			dc.Text("embedsJson", "Embeds JSON", domain.PinInput, false),
		},
		[]domain.NodePort{
			dc.Exec("done", "Done", domain.PinOutput),
			dc.Exec("rejected", "Rejected", domain.PinOutput),
			dc.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "messageId", Label: "Message ID", Kind: "string", Placeholder: "empty — edit the original reply", Required: false},
			{Name: "message", Label: "Message", Kind: "textarea", Required: false},
			{Name: "embeds", Label: "Embeds", Kind: "embed-editor"},
			{Name: "embedsJson", Label: "Embeds JSON", Kind: "textarea", Placeholder: `[{"title":"Hello"}] — overrides the embed editor`},
		},
		map[string]any{
			"messageId": "", "message": "", "embeds": sendmessage.EmptyEmbedDocument(), "embedsJson": "",
		},
	)
}

func interactionPort() domain.NodePort {
	spec := discordspec.InteractionRefType()
	return domain.NodePort{
		ID: "interaction", Label: "Interaction", Kind: domain.PinData, Direction: domain.PinInput,
		DataType: domain.DataObject, Type: &spec, Color: "#60a5fa", Required: true, MaxConnections: 1,
	}
}

// resolve adds the embed document's template pins to the input contract.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	resolved := def
	document := sendmessage.ParseEmbedDocument(configValue(node, "embeds"))
	resolved.Inputs = append(append([]domain.NodePort{}, def.Inputs...), documentPorts(document)...)
	return resolved, nil
}

func documentPorts(document sendmessage.EmbedDocument) []domain.NodePort {
	ports := make([]domain.NodePort, 0, len(document.Pins))
	for _, pin := range document.Pins {
		var dataType domain.DataType
		var typeSpec domain.TypeSpec
		switch pin.Type {
		case sendmessage.PinNumber:
			dataType, typeSpec = domain.DataNumber, typespec.Float()
		case sendmessage.PinBoolean:
			dataType, typeSpec = domain.DataBoolean, typespec.Bool()
		default:
			dataType, typeSpec = domain.DataText, typespec.String()
		}
		port := domain.NodePort{
			ID: pin.Name, Label: pin.Name, Kind: domain.PinData, Direction: domain.PinInput,
			DataType: dataType, Type: &typeSpec, Color: pinColor(dataType), MaxConnections: 1,
		}
		if pin.Default != "" && pin.Type == sendmessage.PinText {
			port.Default = pin.Default
		}
		ports = append(ports, port)
	}
	return ports
}

func pinColor(dataType domain.DataType) string {
	switch dataType {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	default:
		return "#a1a1aa"
	}
}

func configValue(node domain.FlowNode, key string) any {
	config := map[string]any{}
	config, _ = node.Data["config"].(map[string]any)
	return config[key]
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := dc.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	reference, ok := dc.InteractionRef(invocation)
	if !ok {
		return nodes.ExecutionResult{}, errMissingInteraction
	}
	message := dc.String(invocation, "message")
	embeds, reason := resolveEmbeds(invocation)
	if reason != "" {
		return nodes.ExecutionResult{Outputs: map[string]any{"reason": reason}, Ports: []string{"rejected"}}, nil
	}
	if message == "" && len(embeds) == 0 {
		return nodes.ExecutionResult{Outputs: map[string]any{"reason": "a message or at least one embed is required"}, Ports: []string{"rejected"}}, nil
	}
	result, err := sender.EditDiscordInteractionMessage(ctx, domain.DiscordCommandEditRequest{
		IdentityID:  dc.String(invocation, "identityId"),
		Interaction: reference,
		MessageID:   dc.String(invocation, "messageId"),
		Message:     message,
		Embeds:      embeds,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"reason": result.Reason}
	if result.Done {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"done"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

func resolveEmbeds(invocation nodes.Invocation) ([]*domain.DiscordEmbed, string) {
	if raw := dc.String(invocation, "embedsJson"); raw != "" {
		embeds, err := sendmessage.ParseEmbedsJSON(raw)
		if err != nil {
			return nil, err.Error()
		}
		if reason := sendmessage.ValidateEmbeds(embeds); reason != "" {
			return nil, reason
		}
		return embeds, ""
	}
	document := sendmessage.ParseEmbedDocument(invocation.Config["embeds"])
	if len(document.Embeds) == 0 {
		return nil, ""
	}
	return document.BuildEmbeds(sendmessage.PinValues(document, invocation.Inputs))
}

type validationError string

func (e validationError) Error() string { return string(e) }

const errMissingInteraction validationError = "wire the Command Trigger's Interaction output into the Interaction pin"
