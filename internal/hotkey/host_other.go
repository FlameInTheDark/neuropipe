//go:build !windows

package hotkey

import (
	"context"
	"fmt"
)

type unsupportedHost struct{ events chan uint32 }

func newNativeHost() nativeHost { return &unsupportedHost{events: make(chan uint32)} }

func (h *unsupportedHost) Start() error {
	return fmt.Errorf("global hotkeys are supported on Windows only")
}

func (h *unsupportedHost) Replace(context.Context, []registration) error { return nil }
func (h *unsupportedHost) Events() <-chan uint32                         { return h.events }
func (h *unsupportedHost) Stop()                                         {}
