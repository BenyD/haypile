package daemon

import (
	"strconv"
	"strings"
)

// parseNetstatOutbound counts pid's live TCP connections in Windows
// netstat -ano output, excluding listeners and loopback peers. Kept
// portable (and unit-tested) even though only Windows runs it.
//
//	Proto  Local Address    Foreign Address   State        PID
//	TCP    10.0.0.5:52034   93.184.216.34:443 ESTABLISHED  4212
func parseNetstatOutbound(output string, pid int) int {
	want := strconv.Itoa(pid)
	count := 0
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.EqualFold(f[0], "TCP") {
			continue
		}
		if f[4] != want || strings.EqualFold(f[3], "LISTENING") {
			continue
		}
		peer := f[2]
		if strings.HasPrefix(peer, "127.") || strings.HasPrefix(peer, "[::1]") ||
			strings.HasPrefix(peer, "0.0.0.0") || strings.HasPrefix(peer, "[::]") {
			continue // our own API's clients, or no peer at all
		}
		count++
	}
	return count
}
