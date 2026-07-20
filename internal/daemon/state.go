package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// countOutbound lives in outbound_unix.go (lsof) and
// outbound_windows.go (netstat): the same measurement, the platform's
// own tool.
