// Package notifications provides Windows-native side effects for the desktop shell.
package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

// ToastSender sends local Windows toast notifications under one application ID.
type ToastSender struct {
	appID    string
	setup    sync.Once
	setupErr error
}

// NewToastSender creates a native notification sender for one desktop app.
func NewToastSender(appID string) *ToastSender {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		appID = "Neuropipe"
	}
	return &ToastSender{appID: appID}
}

// Send registers the application with Windows once, then displays one toast.
func (s *ToastSender) Send(ctx context.Context, title, message string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send toast: %w", err)
	}
	s.setup.Do(func() {
		s.setupErr = toast.SetAppData(toast.AppData{AppID: s.appID})
	})
	if s.setupErr != nil {
		return fmt.Errorf("register toast application: %w", s.setupErr)
	}
	if err := (&toast.Notification{AppID: s.appID, Title: title, Body: message}).Push(); err != nil {
		return fmt.Errorf("push toast: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send toast: %w", err)
	}
	return nil
}
