//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

// detach launches the daemon in its own process group, detached from the
// CLI's console.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000208} // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
}
