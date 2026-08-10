//go:build windows

package hotkey

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmHotkey         = 0x0312
	wmControl        = 0x8001
	wmQuit           = 0x0012
	modifierNoRepeat = 0x4000
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	registerHotkeyProc    = user32.NewProc("RegisterHotKey")
	unregisterHotkeyProc  = user32.NewProc("UnregisterHotKey")
	getMessageProc        = user32.NewProc("GetMessageW")
	peekMessageProc       = user32.NewProc("PeekMessageW")
	postThreadMessageProc = user32.NewProc("PostThreadMessageW")
)

type nativeMessage struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	point   struct{ x, y int32 }
}

type replaceRequest struct {
	registrations []registration
	done          chan error
}

type windowsHost struct {
	mu       sync.Mutex
	started  bool
	stopped  bool
	threadID uint32
	commands chan replaceRequest
	events   chan uint32
	done     chan struct{}
}

type hostStart struct {
	threadID uint32
	err      error
}

func newNativeHost() nativeHost {
	return &windowsHost{
		commands: make(chan replaceRequest, 1),
		events:   make(chan uint32, 32),
		done:     make(chan struct{}),
	}
}

func (h *windowsHost) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if h.stopped {
		return fmt.Errorf("global hotkey host has stopped")
	}
	ready := make(chan hostStart, 1)
	go h.run(ready)
	result := <-ready
	if result.err != nil {
		return result.err
	}
	h.threadID = result.threadID
	h.started = true
	return nil
}

func (h *windowsHost) Replace(ctx context.Context, registrations []registration) error {
	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return fmt.Errorf("global hotkey host is not running")
	}
	threadID, done := h.threadID, h.done
	h.mu.Unlock()

	request := replaceRequest{registrations: registrations, done: make(chan error, 1)}
	select {
	case h.commands <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return fmt.Errorf("global hotkey host stopped")
	}
	if result, _, callErr := postThreadMessageProc.Call(uintptr(threadID), wmControl, 0, 0); result == 0 {
		select {
		case <-h.commands:
		default:
		}
		return fmt.Errorf("signal global hotkey host: %v", callErr)
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return fmt.Errorf("global hotkey host stopped")
	}
}

func (h *windowsHost) Events() <-chan uint32 { return h.events }

func (h *windowsHost) Stop() {
	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return
	}
	threadID, done := h.threadID, h.done
	h.started, h.stopped = false, true
	h.mu.Unlock()
	if result, _, _ := postThreadMessageProc.Call(uintptr(threadID), wmQuit, 0, 0); result != 0 {
		<-done
	}
}

func (h *windowsHost) run(ready chan<- hostStart) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(h.done)

	var message nativeMessage
	// PeekMessage creates the message queue required by PostThreadMessage before
	// Start makes this thread available to Reload.
	_, _, _ = peekMessageProc.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 0)
	ready <- hostStart{threadID: windows.GetCurrentThreadId()}

	registered := make(map[uint32]registration)
	for {
		result, _, _ := getMessageProc.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			h.unregisterAll(registered)
			return
		}
		switch message.message {
		case wmHotkey:
			select {
			case h.events <- uint32(message.wParam):
			default:
			}
		case wmControl:
			request := <-h.commands
			registered = h.replace(registered, request)
		case wmQuit:
			h.unregisterAll(registered)
			return
		}
	}
}

func (h *windowsHost) replace(current map[uint32]registration, request replaceRequest) map[uint32]registration {
	h.unregisterAll(current)
	next := make(map[uint32]registration, len(request.registrations))
	for _, registration := range request.registrations {
		if err := registerShortcut(registration); err != nil {
			h.unregisterAll(next)
			for _, prior := range current {
				_ = registerShortcut(prior)
			}
			request.done <- err
			return current
		}
		next[registration.id] = registration
	}
	request.done <- nil
	return next
}

func (h *windowsHost) unregisterAll(registrations map[uint32]registration) {
	for id := range registrations {
		_, _, _ = unregisterHotkeyProc.Call(0, uintptr(id))
	}
}

func registerShortcut(registration registration) error {
	modifiers := registration.shortcut.modifiers | modifierNoRepeat
	if result, _, callErr := registerHotkeyProc.Call(0, uintptr(registration.id), uintptr(modifiers), uintptr(registration.shortcut.key)); result == 0 {
		return fmt.Errorf("global hotkey %q is unavailable: %v", registration.shortcut.canonical, callErr)
	}
	return nil
}
