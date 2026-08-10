package chathistory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	datanodes "github.com/FlameInTheDark/neuropipe/internal/nodes/data"
)

// Node is this package's implementation of the shared Blueprint node contract.
type Node = nodes.Implementation

var _ nodes.Node = Node{}

func Register(registrar nodes.Registrar) error {
	inputs := []domain.NodePort{datanodes.Pin("chatId", "Chat ID", domain.PinInput, domain.DataText), datanodes.Pin("limit", "Limit", domain.PinInput, domain.DataNumber)}
	return registrar.Register(Node{Metadata: datanodes.Node("data:chat_history", "Chat", "Read Chat History", "Read earlier messages in this local chat conversation.", "history", inputs, []domain.NodePort{datanodes.Pin("messages", "Messages", domain.PinOutput, domain.DataList)}, nil, map[string]any{"limit": 50}), Executor: nodes.Outputs(Evaluate)})
}

// Evaluate asks only the injected history-reader interface for persisted chat
// data; it does not import any storage or application package.
func Evaluate(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (map[string]any, error) {
	history, ok := runtime.(nodes.ChatHistoryReader)
	if !ok {
		return nil, fmt.Errorf("chat history is unavailable for this execution")
	}
	chatID, _ := invocation.Inputs["chatId"].(string)
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, fmt.Errorf("read chat history requires a chat ID")
	}
	limit, ok := integer(invocation.Inputs["limit"])
	if !ok || limit < 1 {
		return nil, fmt.Errorf("read chat history requires a positive integer Limit")
	}
	messages, err := history.ReadChatHistory(ctx, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("read chat history: %w", err)
	}
	values := make([]any, 0, len(messages))
	for _, message := range messages {
		values = append(values, map[string]any{"id": message.ID, "role": string(message.Role), "content": message.Content, "createdAt": message.CreatedAt.Format(time.RFC3339)})
	}
	return map[string]any{"messages": values}, nil
}

func integer(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number > math.MaxInt || number < math.MinInt {
			return 0, false
		}
		return int(number), true
	case int:
		return number, true
	case int64:
		if int64(int(number)) != number {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}
