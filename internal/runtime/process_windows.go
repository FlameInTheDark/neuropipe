//go:build windows

package runtime

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// configureHiddenChildProcess prevents a console flash for Neuropipe-managed
// inference servers without changing how user-owned processes are handled.
func configureHiddenChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
