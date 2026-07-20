//go:build windows

package daemon

import (
	"os"
	"os/exec"
)

// countOutbound measures this process's live TCP connections, excluding
// listeners and localhost — the "0 external connections" trust commitment
// as an observable number. netstat ships with every Windows; lsof does
// not exist there.
func countOutbound() (int, string) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0, "unverified: netstat unavailable on this system"
	}
	n := parseNetstatOutbound(string(out), os.Getpid())
	out6, err := exec.Command("netstat", "-ano", "-p", "tcpv6").Output()
	if err == nil {
		n += parseNetstatOutbound(string(out6), os.Getpid())
	}
	return n, ""
}
