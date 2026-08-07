//go:build !windows

package daemon

import "golang.org/x/sys/unix"

// beNice drops the daemon to nice 10 — the Spotlight posture: a
// background indexer should never win a fight with whatever the user is
// actually doing. Nice only matters under contention, so an idle machine
// still indexes at full speed and a busy one yields; interactive queries
// are millisecond-scale and never notice either way.
//
// Deliberately nice, not Darwin's background QoS class: that confines
// the process to efficiency cores and throttles its I/O, which turns a
// first index from an hour into an afternoon. clangd walked the same
// path and settled one level up for the same reason.
func beNice() {
	// Best effort: a daemon that cannot lower its own priority (already
	// reniced by the user, odd container policies) should serve anyway.
	_ = unix.Setpriority(unix.PRIO_PROCESS, 0, 10)
}
