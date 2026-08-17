// Package dialogs provides native window dialogs for Blueprint nodes that
// must interact with the user mid-execution. It is the only place where
// Wails v3 native dialog primitives are exposed to node modules; node
// packages depend on the focused DialogOpener contracts declared in
// internal/nodes.
package dialogs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// QuestionResult identifies which button the user pressed on a question dialog.
type QuestionResult string

const (
	QuestionYes    QuestionResult = "yes"
	QuestionNo     QuestionResult = "no"
	QuestionCancel QuestionResult = "cancel"
)

// InputRequest carries the data needed to render a styled input dialog in the
// React layer. Wails v3 has no native text-input dialog, so the input dialog
// is rendered through the existing Neuropipe styled modal primitives and
// resolves via a Wails-bound callback.
type InputRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Label       string `json:"label"`
	InputType   string `json:"inputType"`
	Continue    string `json:"continueLabel"`
	Cancel      string `json:"cancelLabel"`
	Placeholder string `json:"placeholder"`
}

// InputResponse is returned from a styled input dialog.
type InputResponse struct {
	Canceled bool   `json:"canceled"`
	Value    string `json:"value"`
}

// inputPending tracks one outstanding input dialog request.
type inputPending struct {
	done     chan struct{}
	response InputResponse
	err      error
}

// EventSink emits Wails v3 custom events to the React renderer. The desktop
// app supplies the application's Event manager.
type EventSink func(event string, payload ...any)

// Service opens native Wails v3 dialogs and tracks pending input-dialog
// requests. It is safe for concurrent use from multiple pipeline executions.
type Service struct {
	app       *application.App
	emit      EventSink
	pendingMu sync.Mutex
	pending   map[string]*inputPending
}

// New creates a Dialog service. The supplied app is used to invoke native
// Wails v3 dialogs through the application's Dialog manager. The event sink
// emits input-dialog requests to the React renderer.
func New(app *application.App, emit EventSink) *Service {
	return &Service{app: app, emit: emit, pending: make(map[string]*inputPending)}
}

// SetApp replaces the Wails v3 application reference. It is called from the
// desktop Startup hook before any pipeline runs, so the service can be
// constructed before the Wails app exists.
func (s *Service) SetApp(app *application.App) {
	s.app = app
}

// ShowMessage opens a native Wails v3 information dialog with a single OK
// button and blocks until the user dismisses it.
func (s *Service) ShowMessage(ctx context.Context, title, message string) error {
	if s.app == nil {
		return errors.New("dialogs service is not initialised")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan struct{})
	dialog := s.app.Dialog.Info().
		SetTitle(title).
		SetMessage(message)
	// The label must be exactly "Ok": Wails maps the Windows MessageBox OK
	// result to "Ok" and only invokes OnClick when the button label matches
	// case-sensitively. "OK" never matched, the callback never fired, and the
	// pipeline stayed blocked on this node after the dialog closed.
	dialog.AddButton("Ok").SetAsDefault().OnClick(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})
	dialog.Show()
	select {
	case <-done:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ShowQuestion opens a native Wails v3 question dialog with Yes and No
// buttons and blocks until the user chooses one.
func (s *Service) ShowQuestion(ctx context.Context, title, message string) (QuestionResult, error) {
	if s.app == nil {
		return QuestionCancel, errors.New("dialogs service is not initialised")
	}
	if err := ctx.Err(); err != nil {
		return QuestionCancel, err
	}
	result := make(chan QuestionResult, 1)
	dialog := s.app.Dialog.Question().
		SetTitle(title).
		SetMessage(message)
	yes := dialog.AddButton("Yes")
	yes.SetAsDefault().OnClick(func() {
		select {
		case result <- QuestionYes:
		default:
		}
	})
	no := dialog.AddButton("No")
	no.SetAsCancel().OnClick(func() {
		select {
		case result <- QuestionNo:
		default:
		}
	})
	dialog.Show()
	select {
	case choice := <-result:
		return choice, nil
	case <-ctx.Done():
		return QuestionCancel, ctx.Err()
	}
}

// ShowInput emits a Wails v3 custom event that asks the React layer to display
// a styled input dialog and blocks until the user responds or the context is
// cancelled.
func (s *Service) ShowInput(ctx context.Context, request InputRequest) (InputResponse, error) {
	if s.app == nil {
		return InputResponse{Canceled: true}, errors.New("dialogs service is not initialised")
	}
	if s.emit == nil {
		return InputResponse{Canceled: true}, errors.New("dialogs event sink is not configured")
	}
	if request.ID == "" {
		request.ID = newRequestID()
	}
	pending := &inputPending{done: make(chan struct{})}
	s.pendingMu.Lock()
	s.pending[request.ID] = pending
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, request.ID)
		s.pendingMu.Unlock()
	}()

	s.emit("dialog.input.request", request)

	select {
	case <-pending.done:
		return pending.response, pending.err
	case <-ctx.Done():
		s.emit("dialog.input.cancel", map[string]string{"id": request.ID})
		return InputResponse{Canceled: true}, ctx.Err()
	}
}

// ResolveInput is the Wails-bound callback the React layer invokes after the
// user closes the input dialog.
func (s *Service) ResolveInput(id string, response InputResponse) bool {
	s.pendingMu.Lock()
	pending, ok := s.pending[id]
	s.pendingMu.Unlock()
	if !ok {
		return false
	}
	pending.response = response
	close(pending.done)
	return true
}

// newRequestID returns a short, unpredictable identifier for one dialog.
func newRequestID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "dialog-input"
	}
	return "di-" + hex.EncodeToString(buffer)
}

var _ = fmt.Sprintf
