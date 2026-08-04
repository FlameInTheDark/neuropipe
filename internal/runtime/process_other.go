//go:build !windows

package runtime

import "os/exec"

// configureHiddenChildProcess is a portable no-op for non-Windows test builds.
func configureHiddenChildProcess(_ *exec.Cmd) {}
