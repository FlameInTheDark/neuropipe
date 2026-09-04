package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// askUserQuestionsToolName is the model-facing name of the clarification form
// tool. The turn parks until the user resolves every step, and the answers
// come back as this call's tool result.
const askUserQuestionsToolName = "ask_user_questions"

// questionsGuidance stays in the model-mode system prompt for the lifetime of
// the conversation because the tool itself is always offered. It explains
// when asking beats guessing and what a good questionnaire looks like, so the
// model reaches for the form instead of inventing missing facts.
const questionsGuidance = "When you are missing information you need, the user's request is ambiguous, or a decision has several plausible paths, call the ask_user_questions tool instead of guessing: it renders an interactive questionnaire the user answers step by step. Batch everything you need into one call, keep each question short and self-contained, and offer 2-4 concrete options per question, each with a brief description of what choosing it implies. The user can pick an option, type their own answer, or reject any question; their answers arrive as the tool result. Never repeat a question that was already answered, and for rejected questions continue with your best judgement."

// maxQuestionAnswerChars bounds one typed custom answer so a runaway input
// cannot flood the transcript or the next provider request.
const maxQuestionAnswerChars = 4000

// questionResult is the tool-result payload the model receives once every
// step of its questionnaire has been resolved.
type questionResult struct {
	Answers []questionResultAnswer `json:"answers"`
	Note    string                 `json:"note"`
}

// questionResultAnswer reports one resolved step. Answer carries the option
// label or the typed text; Description echoes the chosen option's description
// so the implied trade-off travels with the answer.
type questionResultAnswer struct {
	Question    string `json:"question"`
	Source      string `json:"source"`
	Answer      string `json:"answer,omitempty"`
	Description string `json:"description,omitempty"`
}

// AskUserQuestionsToolDefinition builds the provider definition of the
// clarification form. The schema stays deliberately small: questions in, and
// per question an optional list of labelled options with descriptions.
func AskUserQuestionsToolDefinition() domain.ChatToolDefinition {
	optionSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"label":       map[string]any{"type": "string", "description": "Short option title shown as a selectable button (1-4 words)"},
			"description": map[string]any{"type": "string", "description": "One short sentence on what choosing this option means or implies"},
		},
		"required":             []string{"label"},
		"additionalProperties": false,
	}
	questionSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{"type": "string", "description": "The question shown to the user; short, self-contained, plain text"},
			"options": map[string]any{
				"type":        "array",
				"description": "2-4 concrete, mutually exclusive choices covering the plausible answers; omit only when free text is the only sensible form",
				"items":       optionSchema,
			},
		},
		"required":             []string{"question"},
		"additionalProperties": false,
	}
	return domain.ChatToolDefinition{
		Name:        askUserQuestionsToolName,
		Description: "Show the user an interactive questionnaire and pause until they answer. Use it whenever you are missing information you need, the request is ambiguous, or a decision has several plausible paths - asking beats guessing. Put all questions you need into one call; each question may offer 2-4 concrete options, each optionally with a short description of its implications. The user can pick an option, type a custom answer, or reject any question; their answers arrive as this tool's result and the turn continues from them.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type":        "array",
					"description": "The questions to show, in the order they should be answered",
					"items":       questionSchema,
				},
			},
			"required":             []string{"questions"},
			"additionalProperties": false,
		},
	}
}

// parseQuestions normalises the tool arguments into durable question steps.
// Empty question texts and option labels are dropped so a sloppy model call
// still renders cleanly; an empty result is an error the model can fix.
func parseQuestions(arguments map[string]any) ([]domain.ChatQuestion, error) {
	raw, _ := arguments["questions"].([]any)
	if len(raw) == 0 {
		return nil, fmt.Errorf("questions must be a non-empty array of {question, options?} objects")
	}
	questions := make([]domain.ChatQuestion, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each entry of questions must be an object with a question string and optional options")
		}
		question := strings.TrimSpace(stringArg(entry, "question"))
		if question == "" {
			continue
		}
		parsed := domain.ChatQuestion{Question: question}
		options, _ := entry["options"].([]any)
		for _, option := range options {
			optionEntry, ok := option.(map[string]any)
			if !ok {
				continue
			}
			label := strings.TrimSpace(stringArg(optionEntry, "label"))
			if label == "" {
				continue
			}
			parsed.Options = append(parsed.Options, domain.ChatQuestionOption{
				Label:       label,
				Description: strings.TrimSpace(stringArg(optionEntry, "description")),
			})
		}
		questions = append(questions, parsed)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("questions must contain at least one non-empty question")
	}
	return questions, nil
}

// validateAnswers aligns submitted answers with the stored question steps.
// Every step must be resolved exactly once: an option label matching the
// offered choices, a non-empty custom text, or a rejection. Option
// descriptions are resolved server-side from the stored form so the client
// cannot invent them.
func validateAnswers(questions []domain.ChatQuestion, answers []domain.ChatQuestionAnswer) ([]domain.ChatQuestionAnswer, error) {
	if len(answers) != len(questions) {
		return nil, fmt.Errorf("expected %d answers, got %d", len(questions), len(answers))
	}
	validated := make([]domain.ChatQuestionAnswer, 0, len(answers))
	for index, question := range questions {
		answer := answers[index]
		switch answer.Source {
		case domain.ChatAnswerSourceOption:
			label := strings.TrimSpace(answer.ChosenLabel)
			if label == "" {
				return nil, fmt.Errorf("answer %d must name the chosen option", index+1)
			}
			matched := -1
			for position, option := range question.Options {
				if strings.EqualFold(strings.TrimSpace(option.Label), label) {
					matched = position
					break
				}
			}
			if matched < 0 {
				return nil, fmt.Errorf("answer %d chose %q which was not one of the offered options", index+1, label)
			}
			validated = append(validated, domain.ChatQuestionAnswer{
				Question:          question.Question,
				Source:            domain.ChatAnswerSourceOption,
				ChosenLabel:       question.Options[matched].Label,
				ChosenDescription: question.Options[matched].Description,
			})
		case domain.ChatAnswerSourceCustom:
			text := strings.TrimSpace(answer.Custom)
			if text == "" {
				return nil, fmt.Errorf("answer %d must contain the typed answer", index+1)
			}
			if len(text) > maxQuestionAnswerChars {
				text = strings.TrimSpace(text[:maxQuestionAnswerChars])
			}
			validated = append(validated, domain.ChatQuestionAnswer{
				Question: question.Question,
				Source:   domain.ChatAnswerSourceCustom,
				Custom:   text,
			})
		case domain.ChatAnswerSourceRejected:
			validated = append(validated, domain.ChatQuestionAnswer{
				Question: question.Question,
				Source:   domain.ChatAnswerSourceRejected,
			})
		default:
			return nil, fmt.Errorf("answer %d has an unknown source %q", index+1, answer.Source)
		}
	}
	return validated, nil
}

// questionResultContent renders the model-facing tool result: one entry per
// step plus a note explaining the source vocabulary, so the model can tell a
// picked option from typed text from a refusal.
func questionResultContent(answers []domain.ChatQuestionAnswer) string {
	result := questionResult{
		Answers: make([]questionResultAnswer, 0, len(answers)),
		Note:    `source is "option" (the user picked one of the offered options), "custom" (the user typed their own answer), or "rejected" (the user declined to answer - continue with your best judgement and do not ask again).`,
	}
	for _, answer := range answers {
		entry := questionResultAnswer{Question: answer.Question, Source: answer.Source}
		switch answer.Source {
		case domain.ChatAnswerSourceOption:
			entry.Answer = answer.ChosenLabel
			entry.Description = answer.ChosenDescription
		case domain.ChatAnswerSourceCustom:
			entry.Answer = answer.Custom
		}
		result.Answers = append(result.Answers, entry)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return `{"answers":[]}`
	}
	return string(data)
}

// pauseForQuestions persists an ask_user_questions call as a pending form
// and parks the turn: the tool result is only written once the user submits
// answers, and ResolveQuestions re-enqueues the model round with them.
func (s *Service) pauseForQuestions(ctx context.Context, conversation domain.ChatConversation, chatRunID string, call domain.ChatToolCall, questions []domain.ChatQuestion) error {
	record, err := s.store.CreateChatQuestions(ctx, domain.ChatQuestions{
		ConversationID: conversation.ID,
		ChatRunID:      chatRunID,
		ToolCallID:     call.ID,
		Questions:      questions,
	})
	if err != nil {
		return err
	}
	if err := s.store.UpdateChatRun(ctx, chatRunID, domain.RunPending, "Waiting for your answers", "", ""); err != nil {
		return err
	}
	detail, err := json.Marshal(questions)
	if err != nil {
		detail = []byte("[]")
	}
	if _, err := s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: chatRunID, Kind: "questions", Summary: fmt.Sprintf("Asked %d question(s)", len(questions)), Detail: string(detail), Status: domain.RunPending}); err != nil {
		return err
	}
	if s.emit != nil {
		s.emit("chat.questions.requested", record)
	}
	s.emitUpdate(chatRunID)
	return nil
}

// skipToolCallForPause answers a sibling tool call with a skip notice while
// the turn pauses on an open question form. Every tool_call_id keeps a
// matching result, which keeps the resumed transcript provider-valid, and
// the model can repeat the call once the answers arrive.
func (s *Service) skipToolCallForPause(ctx context.Context, conversationID, chatRunID string, call domain.ChatToolCall) {
	result := "Skipped: the turn paused to ask the user a question before this tool could run. Call it again after the answers arrive if it is still needed."
	_, _ = s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: conversationID, ChatRunID: chatRunID, Role: domain.ChatRoleTool, ToolCallID: call.ID, ToolName: call.Name, Content: result})
	_, _ = s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: chatRunID, Kind: "tool", Summary: toolSummary(call), Detail: result, Status: domain.RunSkipped})
}

// ResolveQuestions stores the user's answers to a paused ask_user_questions
// form and resumes the model turn with them as the tool result. A run that
// already finished (or was cancelled) while the form sat open keeps its
// final state: the answers still land in the transcript, but no new model
// round is queued.
func (s *Service) ResolveQuestions(ctx context.Context, id string, answers []domain.ChatQuestionAnswer) error {
	record, err := s.store.GetChatQuestions(ctx, id)
	if err != nil {
		return err
	}
	if record.Status != domain.ChatQuestionsPending {
		return fmt.Errorf("chat questions %q are no longer pending", id)
	}
	validated, err := validateAnswers(record.Questions, answers)
	if err != nil {
		return err
	}
	if _, err := s.store.ResolveChatQuestions(ctx, id, domain.ChatQuestionsAnswered, validated); err != nil {
		return err
	}
	result := questionResultContent(validated)
	if _, err := s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: record.ConversationID, ChatRunID: record.ChatRunID, Role: domain.ChatRoleTool, ToolCallID: record.ToolCallID, ToolName: askUserQuestionsToolName, Content: result}); err != nil {
		return err
	}
	declined := 0
	for _, answer := range validated {
		if answer.Source == domain.ChatAnswerSourceRejected {
			declined++
		}
	}
	summary := fmt.Sprintf("Answered %d question(s)", len(validated)-declined)
	if declined == len(validated) {
		summary = "Questions declined"
	} else if declined > 0 {
		summary = fmt.Sprintf("%s, %d declined", summary, declined)
	}
	if _, err := s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: record.ChatRunID, Kind: "questions", Summary: summary, Detail: result, Status: domain.RunCompleted}); err != nil {
		return err
	}
	run, err := s.store.GetChatRun(ctx, record.ChatRunID)
	if err != nil {
		return err
	}
	if isFinished(run.Status) {
		return nil
	}
	// Mark the run resumable BEFORE enqueueing: a fast model round could
	// otherwise complete and then be overwritten back to pending.
	if err := s.store.UpdateChatRun(ctx, record.ChatRunID, domain.RunPending, "Working", "", ""); err != nil {
		return err
	}
	s.emitUpdate(record.ChatRunID)
	return s.enqueue(ctx, modelJob{conversationID: record.ConversationID, chatRunID: record.ChatRunID})
}

// expirePendingQuestions closes every outstanding question form when the
// user sends a new message instead of answering. Each form gets a rejected
// tool result so the transcript keeps the tool-call/tool-result pairing
// providers require, and the paused run is retired as superseded.
func (s *Service) expirePendingQuestions(ctx context.Context, conversationID string) {
	pending, err := s.store.ListPendingChatQuestions(ctx, conversationID)
	if err != nil {
		return
	}
	for _, record := range pending {
		if _, err := s.store.ResolveChatQuestions(ctx, record.ID, domain.ChatQuestionsExpired, nil); err != nil {
			continue
		}
		result := `{"answers":[],"note":"The user sent a new message without answering these questions. Treat the message that follows as the user moving on; do not re-ask these questions."}`
		_, _ = s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: record.ConversationID, ChatRunID: record.ChatRunID, Role: domain.ChatRoleTool, ToolCallID: record.ToolCallID, ToolName: askUserQuestionsToolName, Content: result})
		_, _ = s.store.AddChatRunEvent(ctx, domain.ChatRunEvent{ChatRunID: record.ChatRunID, Kind: "questions", Summary: "Questions skipped", Detail: "The user sent a new message without answering.", Status: domain.RunCompleted})
		if run, runErr := s.store.GetChatRun(ctx, record.ChatRunID); runErr == nil && !isFinished(run.Status) {
			_ = s.store.UpdateChatRun(ctx, record.ChatRunID, domain.RunCancelled, "Superseded", run.ExecutionID, "Superseded by a new message")
		}
	}
}

// cancelQuestionsForRun retires pending question forms on a stopped turn and
// writes rejected tool results so the transcript keeps the tool-call/tool-
// result pairing on the next round.
func (s *Service) cancelQuestionsForRun(ctx context.Context, run domain.ChatRun) error {
	pending, err := s.store.ListPendingChatQuestions(ctx, run.ConversationID)
	if err != nil {
		return err
	}
	for _, record := range pending {
		if record.ChatRunID != run.ID {
			continue
		}
		result := `{"answers":[],"note":"The turn was stopped before the user answered these questions."}`
		if _, err := s.store.CreateChatMessage(ctx, domain.ChatMessage{ConversationID: record.ConversationID, ChatRunID: record.ChatRunID, Role: domain.ChatRoleTool, ToolCallID: record.ToolCallID, ToolName: askUserQuestionsToolName, Content: result}); err != nil {
			return err
		}
	}
	return s.store.CancelChatQuestionsForRun(ctx, run.ID)
}
