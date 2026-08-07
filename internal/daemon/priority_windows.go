//go:build windows

package daemon

import "golang.org/x/sys/windows"

// beNice drops the daemon below normal priority; see priority_unix.go
// for the reasoning. BELOW_NORMAL is the analogue of nice 10 — behind
// foreground work, well above the IDLE class that background-starves.
func beNice() {
	// Best effort, same as unix: serving matters more than yielding.
	_ = windows.SetPriorityClass(windows.CurrentProcess(), windows.BELOW_NORMAL_PRIORITY_CLASS)
}
