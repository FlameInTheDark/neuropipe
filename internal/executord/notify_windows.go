//go:build windows

package executord

import (
	"github.com/FlameInTheDark/neuropipe/internal/notifications"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
)

// NewNotifier returns the platform notification sender. On Windows the
// executor shows native toasts for Notification action nodes.
func NewNotifier() pipeline.NotificationSender {
	return notifications.NewToastSender("Neuropipe")
}
