//go:build !windows

package daemon

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestBeNiceLowersPriority(t *testing.T) {
	before, err := unix.Getpriority(unix.PRIO_PROCESS, 0)
	if err != nil {
		t.Fatalf("getpriority: %v", err)
	}
	if before > 10 {
		t.Skipf("already at nice %d; unprivileged processes cannot go back down", before)
	}
	beNice()
	after, err := unix.Getpriority(unix.PRIO_PROCESS, 0)
	if err != nil {
		t.Fatalf("getpriority: %v", err)
	}
	// Getpriority's return convention differs across unixes; accept the
	// raw value or the 20-biased one rather than encode either.
	if after != 10 && after != 20-10 {
		t.Fatalf("nice = %d after beNice, want 10", after)
	}
}
