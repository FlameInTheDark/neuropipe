// Package sendphoto registers the Send Telegram Photo action node.
package sendphoto

import (
	"context"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/attachments"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/tg"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

func New() Node                                { return Node{Metadata: definition(), Resolver: resolve, Executor: execute} }
func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

// Telegram re-encodes and caps photos sent by bots at 10 MB.
const maxPhotoBytes = 10 * 1024 * 1024

func definition() domain.NodeDefinition {
	return tg.Definition("action:telegram_send_photo", "Send Telegram Photo", "Send one photo by URL, local file, base64, or bytes from another node through the selected bot identity.", "camera",
		[]domain.NodePort{
			tg.Exec("in", "Send", domain.PinInput),
			tg.Text("photoUrl", "Photo URL", domain.PinInput, false),
			tg.Text("photoPath", "Photo path", domain.PinInput, false),
			tg.Text("photoBase64", "Photo base64", domain.PinInput, false),
			photoDataPort(),
			tg.Text("photoName", "Photo name", domain.PinInput, false),
			tg.Text("caption", "Caption", domain.PinInput, false),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("parseMode", "Parse mode", domain.PinInput, false),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("sent", "Sent", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("messageId", "Message ID", domain.PinOutput, false),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "photoSource", Label: "Photo source", Kind: "select", Options: attachments.SourceOptions()},
			sourceField("photoUrl", "Photo URL", "string", "https://example.com/photo.jpg — Telegram fetches this server-side", attachments.SourceURL),
			sourceField("photoPath", "Photo path", "string", `C:\Pictures\photo.jpg`, attachments.SourceFile),
			sourceField("photoBase64", "Photo base64", "textarea", "aGVsbG8gd29ybGQ= or a data: URL", attachments.SourceBase64),
			{Name: "chatId", Label: "Chat ID", Kind: "string", Placeholder: "123456 or @mychannel", Required: true},
			{Name: "photoName", Label: "Photo name", Kind: "string", Placeholder: "photo.jpg — names the Base64 and Bytes pins", VisibleWhen: "photoSource=base64|photoSource=bytes|photoSource="},
			{Name: "caption", Label: "Caption", Kind: "textarea"},
			{Name: "parseMode", Label: "Parse mode", Kind: "select", Options: []domain.Option{{Value: "", Label: "Plain text"}, {Value: "HTML", Label: "HTML"}, {Value: "MarkdownV2", Label: "MarkdownV2"}}},
		},
		map[string]any{
			"photoSource": "", "photoUrl": "", "photoPath": "", "photoBase64": "",
			"photoName": "", "chatId": "", "caption": "", "parseMode": "",
		},
	)
}

// sourceField builds one photo-source input gated to its source mode (and to
// Auto, which keeps every input reachable for unconfigured nodes).
func sourceField(name, label, kind, placeholder, source string) domain.ConfigField {
	return domain.ConfigField{
		Name: name, Label: label, Kind: kind, Placeholder: placeholder,
		VisibleWhen: "photoSource=" + source + "|photoSource=",
	}
}

// photoDataPort is the raw bytes pin: it matches the Draw Image node's image
// output type so a rendered picture wires straight into the photo.
func photoDataPort() domain.NodePort {
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	return domain.NodePort{
		ID: "photoData", Label: "Photo data", Kind: domain.PinData, Direction: domain.PinInput,
		DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", MaxConnections: 1,
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	resolved := def
	mode := attachments.SourceMode(configValue(node, "photoSource"))
	resolved.Inputs = filterPhotoPins(def.Inputs, mode)
	return resolved, nil
}

// filterPhotoPins keeps only the photo pins the selected source uses. Auto
// keeps every pin so graphs saved before the selector keep their wires.
func filterPhotoPins(inputs []domain.NodePort, mode string) []domain.NodePort {
	if mode == "" {
		return inputs
	}
	kept := make([]domain.NodePort, 0, len(inputs))
	for _, port := range inputs {
		if !photoPinVisible(port.ID, mode) {
			continue
		}
		kept = append(kept, port)
	}
	return kept
}

func photoPinVisible(pinID, mode string) bool {
	switch pinID {
	case "photoUrl":
		return attachments.SourceIncludes(mode, attachments.SourceURL)
	case "photoPath":
		return attachments.SourceIncludes(mode, attachments.SourceFile)
	case "photoBase64":
		return attachments.SourceIncludes(mode, attachments.SourceBase64)
	case "photoData":
		return attachments.SourceIncludes(mode, attachments.SourceBytes)
	case "photoName":
		return attachments.NameIncludes(mode)
	default:
		return true
	}
}

func configValue(node domain.FlowNode, key string) any {
	config := map[string]any{}
	config, _ = node.Data["config"].(map[string]any)
	return config[key]
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	sender, err := tg.Sender(runtime)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	chatID, err := tg.RequiredString(invocation, "chatId", "chat ID")
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	caption := tg.String(invocation, "caption")
	if utf8.RuneCountInString(caption) > 1024 {
		return nodes.ExecutionResult{Outputs: emptyOutputs("caption exceeds Telegram's 1,024-character limit"), Ports: []string{"rejected"}}, nil
	}
	parseMode := tg.String(invocation, "parseMode")
	switch parseMode {
	case "", "HTML", "MarkdownV2":
	default:
		return nodes.ExecutionResult{}, errParseMode
	}

	request, reason, err := resolvePhoto(ctx, invocation, chatID, caption, parseMode)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	if reason != "" {
		return nodes.ExecutionResult{Outputs: emptyOutputs(reason), Ports: []string{"rejected"}}, nil
	}

	result, err := sender.SendTelegramPhoto(ctx, request)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"messageId": result.MessageID, "reason": result.Reason}
	if result.Sent {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"sent"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

// resolvePhoto builds the photo request from the source the Photo source
// dropdown selected. Auto prefers an upload source over the URL pass-through,
// mirroring the Send Telegram Document node. A missing input is a hard
// validation error; a file that fails to load is a soft rejection.
func resolvePhoto(ctx context.Context, invocation nodes.Invocation, chatID, caption, parseMode string) (domain.TelegramPhotoRequest, string, error) {
	mode := attachments.SourceMode(invocation.Config["photoSource"])
	name := tg.String(invocation, "photoName")
	base := domain.TelegramPhotoRequest{
		IdentityID: tg.String(invocation, "identityId"), ChatID: chatID,
		Caption: caption, ParseMode: parseMode,
	}
	uploadData := func(data any, label string) (domain.TelegramPhotoRequest, string, error) {
		value := attachments.DataValue(data)
		if value == nil {
			return domain.TelegramPhotoRequest{}, "", validationError(label + " is required when the photo source is " + attachments.SourceLabel(mode))
		}
		loaded, err := attachments.Load(ctx, attachments.Sources{Data: value, DataName: name}, attachments.Limits{MaxBytes: maxPhotoBytes, MaxCount: 1})
		if err != nil {
			return domain.TelegramPhotoRequest{}, err.Error(), nil
		}
		request := base
		request.FileName = loaded[0].Name
		request.ContentType = loaded[0].ContentType
		request.Data = loaded[0].Data
		return request, "", nil
	}
	uploadPath := func(path, label string) (domain.TelegramPhotoRequest, string, error) {
		if attachments.IsBlank(path) {
			return domain.TelegramPhotoRequest{}, "", validationError(label + " is required when the photo source is " + attachments.SourceLabel(mode))
		}
		loaded, err := attachments.Load(ctx, attachments.Sources{Paths: path}, attachments.Limits{MaxBytes: maxPhotoBytes, MaxCount: 1})
		if err != nil {
			return domain.TelegramPhotoRequest{}, err.Error(), nil
		}
		request := base
		request.FileName = loaded[0].Name
		request.ContentType = loaded[0].ContentType
		request.Data = loaded[0].Data
		return request, "", nil
	}

	switch mode {
	case attachments.SourceURL:
		if url := tg.String(invocation, "photoUrl"); url != "" {
			base.PhotoURL = url
			return base, "", nil
		}
		return domain.TelegramPhotoRequest{}, "", validationError("photo URL is required when the photo source is URL")
	case attachments.SourceFile:
		return uploadPath(tg.String(invocation, "photoPath"), "photo path")
	case attachments.SourceBase64:
		return uploadData(tg.String(invocation, "photoBase64"), "photo base64")
	case attachments.SourceBytes:
		return uploadData(invocation.Inputs["photoData"], "photo data")
	default: // Auto: upload sources win over the URL pass-through
		if path := tg.String(invocation, "photoPath"); path != "" {
			return uploadPath(path, "photo path")
		}
		if encoded := tg.String(invocation, "photoBase64"); encoded != "" {
			return uploadData(encoded, "photo base64")
		}
		if data := attachments.DataValue(invocation.Inputs["photoData"]); data != nil {
			return uploadData(data, "photo data")
		}
		if url := tg.String(invocation, "photoUrl"); url != "" {
			base.PhotoURL = url
			return base, "", nil
		}
		return domain.TelegramPhotoRequest{}, "", errPhotoRequired
	}
}

func emptyOutputs(reason string) map[string]any {
	return map[string]any{"messageId": "", "reason": reason}
}

type validationError string

func (e validationError) Error() string { return string(e) }

const (
	errPhotoRequired validationError = "a photo URL, file, base64, or data is required"
	errParseMode     validationError = "parse mode must be empty, HTML, or MarkdownV2"
)
