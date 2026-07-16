package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runtimeFile is how CLI processes find a running daemon: a small JSON
// file next to the database. Its address is verified with a health check
// before use, so a stale file after a crash is harmless.
const runtimeFile = "daemon.json"

// Runtime is the on-disk record of a running daemon.
type Runtime struct {
	PID  int    `json:"pid"`
	Addr string `json:"addr"`
}

func writeRuntimeFile(dataDir, addr string) error {
	data, err := json.Marshal(Runtime{PID: os.Getpid(), Addr: addr})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, runtimeFile), data, 0o644)
}

func removeRuntimeFile(dataDir string) {
	os.Remove(filepath.Join(dataDir, runtimeFile))
}

func readRuntimeFile(dataDir string) (Runtime, error) {
	var rt Runtime
	data, err := os.ReadFile(filepath.Join(dataDir, runtimeFile))
	if err != nil {
		return rt, err
	}
	return rt, json.Unmarshal(data, &rt)
}

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
