//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// countOutbound measures this process's live TCP connections, excluding
// listeners and localhost — the "0 external connections" trust commitment
// as an observable number. Best-effort: without lsof it says so instead
// of guessing.
func countOutbound() (int, string) {
	out, err := exec.Command("lsof", "-a", "-p", fmt.Sprint(os.Getpid()), "-i", "TCP", "-n", "-P").Output()
	if err != nil {
		// lsof exits 1 when there are no matching descriptors at all.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return 0, ""
		}
		return 0, "unverified: lsof unavailable on this system"
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "->") {
			continue // header or LISTEN socket
		}
		if strings.Contains(line, "->127.0.0.1") || strings.Contains(line, "->[::1]") {
			continue // CLI client connections to our own API
		}
		count++
	}
	return count, ""
}
