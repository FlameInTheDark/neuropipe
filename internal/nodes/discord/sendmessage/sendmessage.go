// Package sendmessage registers the Send Discord Message action node.
package sendmessage

import (
	"context"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/attachments"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/discord/dc"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

// Discord caps each bot upload at 25 MiB with 10 attachments per message.
const (
	maxAttachmentBytes = 25 * 1024 * 1024
	maxAttachments     = 10
)

func definition() domain.NodeDefinition {
	return dc.Definition("action:discord_send_message", "Send Discord Message", "Send one message with text, embeds, and file attachments through the selected bot identity.", "send",
		[]domain.NodePort{
			dc.Exec("in", "Send", domain.PinInput),
			dc.Text("message", "Message", domain.PinInput, false),
			dc.Text("channel", "Channel ID", domain.PinInput, true),
			dc.Text("replyToMessageId", "Reply to message ID", domain.PinInput, false),
			dc.Text("embedsJson", "Embeds JSON", domain.PinInput, false),
			dc.Text("fileUrl", "File URL", domain.PinInput, false),
			dc.Text("filePath", "File path", domain.PinInput, false),
			dc.Text("fileBase64", "File base64", domain.PinInput, false),
			fileDataPort(),
			dc.Text("fileName", "File name", domain.PinInput, false),
			dc.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			dc.Exec("sent", "Sent", domain.PinOutput),
			dc.Exec("rejected", "Rejected", domain.PinOutput),
			dc.Text("messageId", "Message ID", domain.PinOutput, false),
			dc.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "channel", Label: "Channel ID", Kind: "string", Placeholder: "123456789012345678", Required: true},
			{Name: "message", Label: "Message", Kind: "textarea", Required: false},
			{Name: "embeds", Label: "Embeds", Kind: "embed-editor"},
			{Name: "embedsJson", Label: "Embeds JSON", Kind: "textarea", Placeholder: `[{"title":"Hello"}] — overrides the embed editor`},
			{Name: "imageSource", Label: "Image source", Kind: "select", Options: attachments.SourceOptions()},
			sourceField("fileUrl", "File URL", "string", "https://example.com/report.pdf", attachments.SourceURL),
			sourceField("filePath", "File path", "string", `C:\Reports\report.pdf`, attachments.SourceFile),
			sourceField("fileBase64", "File base64", "textarea", "aGVsbG8gd29ybGQ= or a data: URL", attachments.SourceBase64),
			{Name: "fileName", Label: "File name", Kind: "string", Placeholder: "report.pdf — names the File data pin", VisibleWhen: "imageSource=base64|imageSource=bytes|imageSource="},
		},
		map[string]any{
			"channel": "", "message": "", "embeds": EmptyEmbedDocument(),
			"embedsJson": "", "imageSource": "", "fileUrl": "", "filePath": "", "fileBase64": "", "fileName": "",
		},
	)
}

// sourceField builds one image-source input gated to its source mode (and
// to Auto, which keeps every input reachable for legacy graphs).
func sourceField(name, label, kind, placeholder, source string) domain.ConfigField {
	return domain.ConfigField{
		Name: name, Label: label, Kind: kind, Placeholder: placeholder,
		VisibleWhen: "imageSource=" + source + "|imageSource=",
	}
}

// fileDataPort is the raw bytes pin: it matches the Draw Image node's image
// output type so a rendered picture wires straight into an attachment.
func fileDataPort() domain.NodePort {
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	return domain.NodePort{
		ID: "fileData", Label: "File data", Kind: domain.PinData, Direction: domain.PinInput,
		DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", MaxConnections: 1,
	}
}

/* ------------------------------------------------------------------ */
/* dynamic pins                                                        */
/* ------------------------------------------------------------------ */

// embedPinPortSpec maps one declared embed document pin onto a typed node
// port, mirroring the Draw Image resolver so the canvas shows compatible
// connections per variable type.
func embedPinPortSpec(pin EmbedPin) domain.NodePort {
	var dataType domain.DataType
	var typeSpec domain.TypeSpec
	switch pin.Type {
	case PinNumber:
		dataType, typeSpec = domain.DataNumber, typespec.Float()
	case PinBoolean:
		dataType, typeSpec = domain.DataBoolean, typespec.Bool()
	default:
		dataType, typeSpec = domain.DataText, typespec.String()
	}
	port := domain.NodePort{
		ID: pin.Name, Label: pin.Name, Kind: domain.PinData, Direction: domain.PinInput,
		DataType: dataType, Type: &typeSpec, Color: pinColor(dataType), MaxConnections: 1,
	}
	if pin.Default != "" && pin.Type == PinText {
		port.Default = pin.Default
	}
	return port
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

// resolve adapts the input pin contract to the embed document's declared
// variables and to the selected image source, so pipelines wire typed data
// into {{templates}} and the canvas only offers the pins the source uses.
func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	resolved := def
	document := ParseEmbedDocument(configValue(node, "embeds"))
	inputsWithDocument := append(append([]domain.NodePort{}, def.Inputs...), documentPorts(document)...)
	mode := attachments.SourceMode(configValue(node, "imageSource"))
	resolved.Inputs = filterImagePins(inputsWithDocument, mode)
	return resolved, nil
}

// filterImagePins keeps only the attachment pins the selected image source
// uses. Auto keeps every pin, so graphs saved before the selector existed
// (and graphs that never pick one) keep their wired connections.
func filterImagePins(inputs []domain.NodePort, mode string) []domain.NodePort {
	if mode == "" {
		return inputs
	}
	kept := make([]domain.NodePort, 0, len(inputs))
	for _, port := range inputs {
		if !imagePinVisible(port.ID, mode) {
			continue
		}
		kept = append(kept, port)
	}
	return kept
}

// imagePinVisible maps one attachment pin onto the source modes that use it.
// Pins that are not attachment pins are always visible.
func imagePinVisible(pinID, mode string) bool {
	switch pinID {
	case "fileUrl":
		return attachments.SourceIncludes(mode, attachments.SourceURL)
	case "filePath":
		return attachments.SourceIncludes(mode, attachments.SourceFile)
	case "fileBase64":
		return attachments.SourceIncludes(mode, attachments.SourceBase64)
	case "fileData":
		return attachments.SourceIncludes(mode, attachments.SourceBytes)
	case "fileName":
		return attachments.NameIncludes(mode)
	default:
		return true
	}
}

func documentPorts(document EmbedDocument) []domain.NodePort {
	ports := make([]domain.NodePort, 0, len(document.Pins))
	for _, pin := range document.Pins {
		ports = append(ports, embedPinPortSpec(pin))
	}
	return ports
}

func configValue(node domain.FlowNode, key string) any {
	config := map[string]any{}
	if value, ok := node.Data["config"].(map[string]any); ok {
		config = value
	} else if len(node.Data) > 0 {
		config = node.Data
	}
	return config[key]
}

/* ------------------------------------------------------------------ */
/* execution                                                           */
/* ------------------------------------------------------------------ */

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := dc.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	channel, err := dc.RequiredString(invocation, "channel", "channel ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	message := dc.String(invocation, "message")
	if message != "" && utf8.RuneCountInString(message) > 2000 {
		return nodes.ExecutionResult{Outputs: emptyOutputs("message exceeds Discord's 2,000-character limit"), Ports: []string{"rejected"}}, nil
	}

	embeds, reason := resolveEmbeds(invocation)
	if reason != "" {
		return nodes.ExecutionResult{Outputs: emptyOutputs(reason), Ports: []string{"rejected"}}, nil
	}

	files, reason := resolveAttachments(ctx, invocation)
	if reason != "" {
		return nodes.ExecutionResult{Outputs: emptyOutputs(reason), Ports: []string{"rejected"}}, nil
	}

	if message == "" && len(embeds) == 0 && len(files) == 0 {
		return nodes.ExecutionResult{}, errEmptyMessage
	}

	result, err := sender.SendDiscordMessage(ctx, domain.DiscordMessageRequest{
		IdentityID: dc.String(invocation, "identityId"), ChannelID: channel,
		Message: message, ReplyToID: dc.String(invocation, "replyToMessageId"),
		Embeds: embeds, Attachments: files,
	})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"messageId": result.MessageID, "reason": result.Reason}
	if result.Sent {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"sent"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

// resolveEmbeds picks the embed source: a non-empty embedsJson pin carries
// live data computed by the pipeline and overrides the editor document;
// otherwise the document's {{templates}} resolve from input pin values.
func resolveEmbeds(invocation nodes.Invocation) ([]*domain.DiscordEmbed, string) {
	if raw := dc.String(invocation, "embedsJson"); raw != "" {
		embeds, err := ParseEmbedsJSON(raw)
		if err != nil {
			return nil, err.Error()
		}
		if reason := ValidateEmbeds(embeds); reason != "" {
			return nil, reason
		}
		return embeds, ""
	}
	document := ParseEmbedDocument(invocation.Config["embeds"])
	if len(document.Embeds) == 0 {
		return nil, ""
	}
	return document.BuildEmbeds(PinValues(document, invocation.Inputs))
}

// resolveAttachments loads files from the source the Image source dropdown
// selected — URL, local path, base64 text, or raw bytes — with Discord's
// upload caps enforced before any request. Auto uses every source that is
// set, exactly like graphs saved before the dropdown existed.
func resolveAttachments(ctx context.Context, invocation nodes.Invocation) ([]domain.DiscordAttachment, string) {
	mode := attachments.SourceMode(invocation.Config["imageSource"])
	sources := attachments.Sources{DataName: dc.String(invocation, "fileName")}
	switch mode {
	case attachments.SourceURL:
		sources.URLs = dc.String(invocation, "fileUrl")
	case attachments.SourceFile:
		sources.Paths = dc.String(invocation, "filePath")
	case attachments.SourceBase64:
		sources.Data = attachments.DataValue(dc.String(invocation, "fileBase64"))
	case attachments.SourceBytes:
		sources.Data = attachments.DataValue(invocation.Inputs["fileData"])
	default:
		sources.URLs = dc.String(invocation, "fileUrl")
		sources.Paths = dc.String(invocation, "filePath")
		if data := attachments.DataValue(invocation.Inputs["fileData"]); data != nil {
			sources.Data = data
		} else {
			sources.Data = attachments.DataValue(dc.String(invocation, "fileBase64"))
		}
	}
	if sources.URLs == "" && sources.Paths == "" && sources.Data == nil {
		return nil, ""
	}
	loaded, err := attachments.Load(ctx, sources, attachments.Limits{MaxBytes: maxAttachmentBytes, MaxCount: maxAttachments})
	if err != nil {
		return nil, err.Error()
	}
	files := make([]domain.DiscordAttachment, 0, len(loaded))
	for _, attachment := range loaded {
		files = append(files, domain.DiscordAttachment{Name: attachment.Name, ContentType: attachment.ContentType, Data: attachment.Data})
	}
	return files, ""
}

func emptyOutputs(reason string) map[string]any {
	return map[string]any{"messageId": "", "reason": reason}
}

type validationError string

func (e validationError) Error() string { return string(e) }

const errEmptyMessage validationError = "message is empty — provide message text, embeds, or attachments"
