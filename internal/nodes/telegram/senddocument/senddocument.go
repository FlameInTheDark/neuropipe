// Package senddocument registers the Send Telegram Document action node.
package senddocument

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

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	def := definition()
	resolved := def
	mode := attachments.SourceMode(configValue(node, "documentSource"))
	resolved.Inputs = filterDocumentPins(def.Inputs, mode)
	return resolved, nil
}

// filterDocumentPins keeps only the document pins the selected source uses.
// Auto keeps every pin so graphs saved before the selector keep their wires.
func filterDocumentPins(inputs []domain.NodePort, mode string) []domain.NodePort {
	if mode == "" {
		return inputs
	}
	kept := make([]domain.NodePort, 0, len(inputs))
	for _, port := range inputs {
		if !documentPinVisible(port.ID, mode) {
			continue
		}
		kept = append(kept, port)
	}
	return kept
}

func documentPinVisible(pinID, mode string) bool {
	switch pinID {
	case "documentUrl":
		return attachments.SourceIncludes(mode, attachments.SourceURL)
	case "documentPath":
		return attachments.SourceIncludes(mode, attachments.SourceFile)
	case "documentBase64":
		return attachments.SourceIncludes(mode, attachments.SourceBase64)
	case "documentData":
		return attachments.SourceIncludes(mode, attachments.SourceBytes)
	case "fileName":
		return attachments.NameIncludes(mode)
	default:
		return true
	}
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

// Telegram caps bot uploads at 50 MB; one document per message.
const maxDocumentBytes = 50 * 1024 * 1024

var parseModeOptions = []domain.Option{
	{Value: "", Label: "Plain text"},
	{Value: "HTML", Label: "HTML"},
	{Value: "MarkdownV2", Label: "MarkdownV2"},
}

func definition() domain.NodeDefinition {
	return tg.Definition("action:telegram_send_document", "Send Telegram Document", "Send one file by URL, local path, base64, or bytes from another node with a caption through the selected bot identity.", "send",
		[]domain.NodePort{
			tg.Exec("in", "Send", domain.PinInput),
			tg.Text("documentUrl", "Document URL", domain.PinInput, false),
			tg.Text("documentPath", "Document path", domain.PinInput, false),
			tg.Text("documentBase64", "Document base64", domain.PinInput, false),
			documentDataPort(),
			tg.Text("fileName", "File name", domain.PinInput, false),
			tg.Text("caption", "Caption", domain.PinInput, false),
			tg.Text("chatId", "Chat ID", domain.PinInput, true),
			tg.Text("parseMode", "Parse mode", domain.PinInput, false),
			tg.Text("replyToMessageId", "Reply to message ID", domain.PinInput, false),
			tg.Bool("disableNotification", "Silent", domain.PinInput, false),
			tg.Text("identityId", "Identity", domain.PinInput, false),
		},
		[]domain.NodePort{
			tg.Exec("sent", "Sent", domain.PinOutput),
			tg.Exec("rejected", "Rejected", domain.PinOutput),
			tg.Text("messageId", "Message ID", domain.PinOutput, false),
			tg.Text("reason", "Reason", domain.PinOutput, false),
		},
		[]domain.ConfigField{
			{Name: "documentSource", Label: "Document source", Kind: "select", Options: attachments.SourceOptions()},
			sourceField("documentUrl", "Document URL", "string", "https://example.com/report.pdf — Telegram fetches this server-side", attachments.SourceURL),
			sourceField("documentPath", "Document path", "string", `C:\Reports\report.pdf`, attachments.SourceFile),
			sourceField("documentBase64", "Document base64", "textarea", "aGVsbG8gd29ybGQ= or a data: URL", attachments.SourceBase64),
			{Name: "chatId", Label: "Chat ID", Kind: "string", Placeholder: "123456 or @mychannel", Required: true},
			{Name: "fileName", Label: "File name", Kind: "string", Placeholder: "report.pdf — names the Base64 and Bytes pins", VisibleWhen: "documentSource=base64|documentSource=bytes|documentSource="},
			{Name: "caption", Label: "Caption", Kind: "textarea"},
			{Name: "parseMode", Label: "Parse mode", Kind: "select", Options: parseModeOptions},
			{Name: "replyToMessageId", Label: "Reply to message ID", Kind: "string"},
			{Name: "disableNotification", Label: "Silent (no notification)", Kind: "boolean"},
		},
		map[string]any{
			"documentSource": "", "documentUrl": "", "documentPath": "", "documentBase64": "", "fileName": "",
			"caption": "", "parseMode": "", "replyToMessageId": "", "disableNotification": false,
		},
	)
}

// sourceField builds one document-source input gated to its source mode (and
// to Auto, which keeps every input reachable for legacy graphs).
func sourceField(name, label, kind, placeholder, source string) domain.ConfigField {
	return domain.ConfigField{
		Name: name, Label: label, Kind: kind, Placeholder: placeholder,
		VisibleWhen: "documentSource=" + source + "|documentSource=",
	}
}

// documentDataPort is the raw bytes pin: it matches the Draw Image node's
// image output type so a rendered picture wires straight into a document.
func documentDataPort() domain.NodePort {
	bytesType := domain.TypeSpec{Kind: domain.TypeBytes}
	return domain.NodePort{
		ID: "documentData", Label: "Document data", Kind: domain.PinData, Direction: domain.PinInput,
		DataType: domain.DataBytes, Type: &bytesType, Color: "#fbbf24", MaxConnections: 1,
	}
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

	request, reason, resolveErr := resolveDocument(ctx, invocation, chatID, caption, parseMode)
	if resolveErr != nil {
		return nodes.ExecutionResult{}, resolveErr
	}
	if reason != "" {
		return nodes.ExecutionResult{Outputs: emptyOutputs(reason), Ports: []string{"rejected"}}, nil
	}

	result, err := sender.SendTelegramDocument(ctx, request)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	outputs := map[string]any{"messageId": result.MessageID, "reason": result.Reason}
	if result.Sent {
		return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"sent"}}, nil
	}
	return nodes.ExecutionResult{Outputs: outputs, Ports: []string{"rejected"}}, nil
}

func emptyOutputs(reason string) map[string]any {
	return map[string]any{"messageId": "", "reason": reason}
}

// resolveDocument builds the document request from the source the Document
// source dropdown selected. Auto prefers an upload source over the URL
// pass-through (the pre-selector behaviour). A missing input is a hard
// validation error; a file that fails to load is a soft rejection.
func resolveDocument(ctx context.Context, invocation nodes.Invocation, chatID, caption, parseMode string) (domain.TelegramDocumentRequest, string, error) {
	mode := attachments.SourceMode(invocation.Config["documentSource"])
	base := domain.TelegramDocumentRequest{
		IdentityID:          tg.String(invocation, "identityId"),
		ChatID:              chatID,
		FileName:            tg.String(invocation, "fileName"),
		Caption:             caption,
		ParseMode:           parseMode,
		ReplyToMessageID:    tg.String(invocation, "replyToMessageId"),
		DisableNotification: tg.BoolValue(invocation, "disableNotification"),
	}
	uploadData := func(data any, label string) (domain.TelegramDocumentRequest, string, error) {
		value := attachments.DataValue(data)
		if value == nil {
			return domain.TelegramDocumentRequest{}, "", validationError(label + " is required when the document source is " + attachments.SourceLabel(mode))
		}
		loaded, err := attachments.Load(ctx, attachments.Sources{Data: value, DataName: base.FileName}, attachments.Limits{MaxBytes: maxDocumentBytes, MaxCount: 1})
		if err != nil {
			return domain.TelegramDocumentRequest{}, err.Error(), nil
		}
		request := base
		request.FileName = loaded[0].Name
		request.ContentType = loaded[0].ContentType
		request.Data = loaded[0].Data
		return request, "", nil
	}
	uploadPath := func(path, label string) (domain.TelegramDocumentRequest, string, error) {
		if attachments.IsBlank(path) {
			return domain.TelegramDocumentRequest{}, "", validationError(label + " is required when the document source is " + attachments.SourceLabel(mode))
		}
		loaded, err := attachments.Load(ctx, attachments.Sources{Paths: path}, attachments.Limits{MaxBytes: maxDocumentBytes, MaxCount: 1})
		if err != nil {
			return domain.TelegramDocumentRequest{}, err.Error(), nil
		}
		request := base
		request.FileName = loaded[0].Name
		request.ContentType = loaded[0].ContentType
		request.Data = loaded[0].Data
		return request, "", nil
	}

	switch mode {
	case attachments.SourceURL:
		if url := tg.String(invocation, "documentUrl"); url != "" {
			base.DocumentURL = url
			return base, "", nil
		}
		return domain.TelegramDocumentRequest{}, "", validationError("document URL is required when the document source is URL")
	case attachments.SourceFile:
		return uploadPath(tg.String(invocation, "documentPath"), "document path")
	case attachments.SourceBase64:
		return uploadData(tg.String(invocation, "documentBase64"), "document base64")
	case attachments.SourceBytes:
		return uploadData(invocation.Inputs["documentData"], "document data")
	default: // Auto: path and data load as uploads, the URL passes through
		request := base
		request.DocumentURL = tg.String(invocation, "documentUrl")
		if path := tg.String(invocation, "documentPath"); path != "" || invocation.Inputs["documentData"] != nil || tg.String(invocation, "documentBase64") != "" {
			data := attachments.DataValue(invocation.Inputs["documentData"])
			if data == nil {
				data = attachments.DataValue(tg.String(invocation, "documentBase64"))
			}
			loaded, err := attachments.Load(ctx, attachments.Sources{
				Paths: path, Data: data, DataName: request.FileName,
			}, attachments.Limits{MaxBytes: maxDocumentBytes, MaxCount: 1})
			if err != nil {
				return domain.TelegramDocumentRequest{}, err.Error(), nil
			}
			if len(loaded) > 0 {
				request.FileName = loaded[0].Name
				request.ContentType = loaded[0].ContentType
				request.Data = loaded[0].Data
			}
		}
		if request.DocumentURL == "" && len(request.Data) == 0 {
			return domain.TelegramDocumentRequest{}, "", errDocumentRequired
		}
		return request, "", nil
	}
}

type validationError string

func (e validationError) Error() string { return string(e) }

const (
	errDocumentRequired validationError = "a document URL, path, base64, or data is required"
	errParseMode        validationError = "parse mode must be empty, HTML, or MarkdownV2"
)
