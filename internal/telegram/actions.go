package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
)

// apiError is a soft Bot API rejection: the transport worked, Telegram
// answered ok=false. Action methods translate it into a rejected result with
// the API's own description instead of a node error.
type apiError struct {
	Code        int
	Description string
}

func (e *apiError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("telegram api error %d", e.Code)
	}
	return e.Description
}

type apiEnvelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

type getMeResult struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type sendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

// call performs one Bot API method with a JSON body. The 30-second cap covers
// every action method; long polling uses its own longer context.
func (s *Service) call(ctx context.Context, token, method string, body any, destination any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	s.mu.RLock()
	baseURL := s.baseURL
	s.mu.RUnlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/bot"+token+"/"+method, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := s.http.Do(request.WithContext(callCtx))
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	var envelope apiEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: %s", method, response.Status)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("telegram bot token was rejected")
	}
	if !envelope.OK {
		return &apiError{Code: response.StatusCode, Description: envelope.Description}
	}
	if destination == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, destination)
}

func (s *Service) getMe(ctx context.Context, token string) (getMeResult, error) {
	var result getMeResult
	if err := s.call(ctx, token, "getMe", map[string]any{}, &result); err != nil {
		return getMeResult{}, err
	}
	return result, nil
}

// SendTelegramMessage sends one text message. Transport failures are hard
// errors; Telegram rejections are soft results carrying the API description.
func (s *Service) SendTelegramMessage(ctx context.Context, request domain.TelegramMessageRequest) (domain.TelegramMessageResult, error) {
	if utf8.RuneCountInString(request.Message) > maxMessageRunes {
		return domain.TelegramMessageResult{Reason: "message exceeds Telegram's 4,096-character limit"}, nil
	}
	replyID := int64(0)
	if request.ReplyToMessageID != "" {
		// The Bot API types reply_to_message_id as Integer, so the
		// string pin value is converted; a non-numeric ID would
		// otherwise surface as an opaque Bad Request.
		parsed, err := strconv.ParseInt(strings.TrimSpace(request.ReplyToMessageID), 10, 64)
		if err != nil {
			return domain.TelegramMessageResult{Reason: fmt.Sprintf("reply to message ID %q is not numeric", request.ReplyToMessageID)}, nil
		}
		replyID = parsed
	}
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramMessageResult{}, err
	}
	body := map[string]any{"chat_id": request.ChatID, "text": request.Message}
	if request.ParseMode != "" {
		body["parse_mode"] = request.ParseMode
	}
	if replyID != 0 {
		body["reply_to_message_id"] = replyID
	}
	if request.DisableNotification {
		body["disable_notification"] = true
	}
	var result sendMessageResult
	if err := s.call(ctx, token, "sendMessage", body, &result); err != nil {
		return softMessage(err), nil
	}
	return domain.TelegramMessageResult{MessageID: formatInt(result.MessageID), Sent: true}, nil
}

// SendTelegramPhoto sends one photo. A photo given only by URL is posted as
// JSON and Telegram fetches it server-side; a photo carrying bytes is
// uploaded as multipart form data.
func (s *Service) SendTelegramPhoto(ctx context.Context, request domain.TelegramPhotoRequest) (domain.TelegramMessageResult, error) {
	if utf8.RuneCountInString(request.Caption) > maxCaptionRunes {
		return domain.TelegramMessageResult{Reason: "caption exceeds Telegram's 1,024-character limit"}, nil
	}
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramMessageResult{}, err
	}
	var result sendMessageResult
	if len(request.Data) == 0 {
		body := map[string]any{"chat_id": request.ChatID, "photo": request.PhotoURL}
		if request.Caption != "" {
			body["caption"] = request.Caption
		}
		if request.ParseMode != "" {
			body["parse_mode"] = request.ParseMode
		}
		if err := s.call(ctx, token, "sendPhoto", body, &result); err != nil {
			return softMessage(err), nil
		}
		return domain.TelegramMessageResult{MessageID: formatInt(result.MessageID), Sent: true}, nil
	}
	name := request.FileName
	if strings.TrimSpace(name) == "" {
		name = "photo.jpg"
	}
	contentType := request.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if err := s.callMultipart(ctx, token, "sendPhoto", photoFormFields(request), formFile{
		field: "photo", name: name, contentType: contentType, data: request.Data,
	}, &result); err != nil {
		return softMessage(err), nil
	}
	return domain.TelegramMessageResult{MessageID: formatInt(result.MessageID), Sent: true}, nil
}

func photoFormFields(request domain.TelegramPhotoRequest) map[string]string {
	fields := map[string]string{"chat_id": request.ChatID}
	if request.Caption != "" {
		fields["caption"] = request.Caption
	}
	if request.ParseMode != "" {
		fields["parse_mode"] = request.ParseMode
	}
	return fields
}

// SendTelegramDocument sends one general file. A document given only by URL is
// posted as JSON and Telegram fetches it server-side; a document carrying
// bytes is uploaded as multipart form data. Captions double as the message
// text because Telegram cannot send text and a file in one API call.
func (s *Service) SendTelegramDocument(ctx context.Context, request domain.TelegramDocumentRequest) (domain.TelegramMessageResult, error) {
	if utf8.RuneCountInString(request.Caption) > maxCaptionRunes {
		return domain.TelegramMessageResult{Reason: "caption exceeds Telegram's 1,024-character limit"}, nil
	}
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramMessageResult{}, err
	}
	var result sendMessageResult
	if len(request.Data) == 0 {
		body := map[string]any{"chat_id": request.ChatID, "document": request.DocumentURL}
		if request.Caption != "" {
			body["caption"] = request.Caption
		}
		if request.ParseMode != "" {
			body["parse_mode"] = request.ParseMode
		}
		if request.DisableNotification {
			body["disable_notification"] = true
		}
		if request.ReplyToMessageID != "" {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(request.ReplyToMessageID), 10, 64); err == nil {
				body["reply_to_message_id"] = parsed
			}
		}
		if err := s.call(ctx, token, "sendDocument", body, &result); err != nil {
			return softMessage(err), nil
		}
		return domain.TelegramMessageResult{MessageID: formatInt(result.MessageID), Sent: true}, nil
	}
	name := request.FileName
	if strings.TrimSpace(name) == "" {
		name = "document.bin"
	}
	if err := s.callMultipart(ctx, token, "sendDocument", documentFormFields(request), formFile{
		field: "document", name: name, contentType: request.ContentType, data: request.Data,
	}, &result); err != nil {
		return softMessage(err), nil
	}
	return domain.TelegramMessageResult{MessageID: formatInt(result.MessageID), Sent: true}, nil
}

func documentFormFields(request domain.TelegramDocumentRequest) map[string]string {
	fields := map[string]string{"chat_id": request.ChatID}
	if request.Caption != "" {
		fields["caption"] = request.Caption
	}
	if request.ParseMode != "" {
		fields["parse_mode"] = request.ParseMode
	}
	if request.ReplyToMessageID != "" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(request.ReplyToMessageID), 10, 64); err == nil {
			fields["reply_to_message_id"] = strconv.FormatInt(parsed, 10)
		}
	}
	if request.DisableNotification {
		fields["disable_notification"] = "true"
	}
	return fields
}

type formFile struct {
	field       string
	name        string
	contentType string
	data        []byte
}

// callMultipart performs one Bot API method as multipart/form-data: text
// fields plus a single file part. The file streams from the request's byte
// slice; Telegram caps uploads at 50 MB, far below any memory concern.
func (s *Service) callMultipart(ctx context.Context, token, method string, fields map[string]string, file formFile, destination any) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	var part io.Writer
	var err error
	if file.contentType != "" {
		part, err = writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {fmt.Sprintf("form-data; name=%q; filename=%q", file.field, file.name)},
			"Content-Type":        {file.contentType},
		})
	} else {
		part, err = writer.CreateFormFile(file.field, file.name)
	}
	if err != nil {
		return err
	}
	if _, err := part.Write(file.data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	s.mu.RLock()
	baseURL := s.baseURL
	s.mu.RUnlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/bot"+token+"/"+method, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	response, err := s.http.Do(request.WithContext(callCtx))
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	var envelope apiEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: %s", method, response.Status)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("telegram bot token was rejected")
	}
	if !envelope.OK {
		return &apiError{Code: response.StatusCode, Description: envelope.Description}
	}
	if destination == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, destination)
}

func (s *Service) EditTelegramMessage(ctx context.Context, request domain.TelegramEditRequest) (domain.TelegramActionResult, error) {
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramActionResult{}, err
	}
	body := map[string]any{"chat_id": request.ChatID, "message_id": request.MessageID, "text": request.Message}
	if request.ParseMode != "" {
		body["parse_mode"] = request.ParseMode
	}
	if err := s.call(ctx, token, "editMessageText", body, nil); err != nil {
		return softAction(err), nil
	}
	return domain.TelegramActionResult{Done: true}, nil
}

func (s *Service) DeleteTelegramMessage(ctx context.Context, request domain.TelegramDeleteRequest) (domain.TelegramActionResult, error) {
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramActionResult{}, err
	}
	body := map[string]any{"chat_id": request.ChatID, "message_id": request.MessageID}
	if err := s.call(ctx, token, "deleteMessage", body, nil); err != nil {
		return softAction(err), nil
	}
	return domain.TelegramActionResult{Done: true}, nil
}

func (s *Service) AnswerTelegramCallbackQuery(ctx context.Context, request domain.TelegramCallbackAnswerRequest) (domain.TelegramActionResult, error) {
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramActionResult{}, err
	}
	body := map[string]any{"callback_query_id": request.CallbackQueryID}
	if request.Text != "" {
		body["text"] = request.Text
	}
	if request.ShowAlert {
		body["show_alert"] = true
	}
	if err := s.call(ctx, token, "answerCallbackQuery", body, nil); err != nil {
		return softAction(err), nil
	}
	return domain.TelegramActionResult{Done: true}, nil
}

func (s *Service) SendTelegramChatAction(ctx context.Context, request domain.TelegramChatActionRequest) (domain.TelegramActionResult, error) {
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramActionResult{}, err
	}
	body := map[string]any{"chat_id": request.ChatID, "action": request.Action}
	if err := s.call(ctx, token, "sendChatAction", body, nil); err != nil {
		return softAction(err), nil
	}
	return domain.TelegramActionResult{Done: true}, nil
}

func (s *Service) PinTelegramMessage(ctx context.Context, request domain.TelegramPinRequest) (domain.TelegramActionResult, error) {
	_, token, err := s.identityToken(request.IdentityID)
	if err != nil {
		return domain.TelegramActionResult{}, err
	}
	method, body := "pinChatMessage", map[string]any{"chat_id": request.ChatID, "message_id": request.MessageID}
	if request.Unpin {
		method = "unpinChatMessage"
		body = map[string]any{"chat_id": request.ChatID, "message_id": request.MessageID}
	} else if request.Notify {
		body["disable_notification"] = false
	} else {
		body["disable_notification"] = true
	}
	if err := s.call(ctx, token, method, body, nil); err != nil {
		return softAction(err), nil
	}
	return domain.TelegramActionResult{Done: true}, nil
}

func softMessage(err error) domain.TelegramMessageResult {
	if api, ok := err.(*apiError); ok {
		return domain.TelegramMessageResult{Reason: api.Description}
	}
	return domain.TelegramMessageResult{Reason: err.Error()}
}

func softAction(err error) domain.TelegramActionResult {
	if api, ok := err.(*apiError); ok {
		return domain.TelegramActionResult{Reason: api.Description}
	}
	return domain.TelegramActionResult{Reason: err.Error()}
}

func formatInt(value int64) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

// Service satisfies the node module port so Desktop can inject it directly.
var _ nodes.TelegramSender = (*Service)(nil)
