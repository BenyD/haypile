//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a console-subsystem child (netstat, powershell) from
// opening a console window. The daemon runs detached with no console of
// its own, so without this Windows allocates a fresh one per child: a
// visible window flash for every netstat the status poller spawns.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
