//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// detach puts the auto-started daemon in its own session so it survives
// the CLI process and its terminal.
func Detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
