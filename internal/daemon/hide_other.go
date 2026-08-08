//go:build !windows

package daemon

import "os/exec"

// hideWindow is Windows-only; Unix children inherit no window to hide.
func hideWindow(*exec.Cmd) {}
