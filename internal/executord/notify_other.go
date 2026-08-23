//go:build !windows

package executord

import (
	"context"
	"fmt"
	"os"

	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// consoleNotifier prints a compact completion line. Notification content is
// pipeline data, so it goes to the executor's own console only and never
// into any log aggregation.
type consoleNotifier struct{}

func (consoleNotifier) Send(_ context.Context, title, message string) error {
	if os.Getenv("NEUROPIPE_EXECUTOR_QUIET") != "" {
		return nil
	}
	_, err := fmt.Fprintf(os.Stderr, "notification: %s: %s\n", title, message)
	return err
}

// NewNotifier returns the platform notification sender. Outside Windows the
// executor reports notifications on its console.
func NewNotifier() pipeline.NotificationSender { return consoleNotifier{} }
