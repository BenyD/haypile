// Package e2e is the daemon smoke test the PRD requires in CI from M3:
// build the real binary, serve, add a folder, drop a file in, query the
// API, and assert citations — the product promise exercised end to end,
// subprocess and all, under -race.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDaemonSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("smoke test builds a binary; skipped in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "hay")
	build := exec.Command("go", "build", "-race", "-o", bin, "../cmd/hay")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building hay: %v\n%s", err, out)
	}

	home := t.TempDir()
	docs := t.TempDir()
	env := append(os.Environ(),
		"HAYPILE_DIR="+home,
		"HAYPILE_ADDR=127.0.0.1:0", // ephemeral port; discovered via daemon.json
	)

	// serve → runs until we signal it.
	serve := exec.Command(bin, "serve")
	serve.Env = env
	var serveLog bytes.Buffer
	serve.Stdout, serve.Stderr = &serveLog, &serveLog
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		serve.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- serve.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			serve.Process.Kill()
			t.Error("daemon did not shut down within 10s of SIGTERM")
		}
	}()

	addr := waitForDaemon(t, home)
	base := "http://" + addr

	// add → routed through the running daemon, so the folder is watched.
	if err := os.WriteFile(filepath.Join(docs, "pre.md"),
		[]byte("# Existing\n\nThe heron report was here before the daemon."), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command(bin, "add", docs)
	add.Env = env
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("hay add: %v\n%s\ndaemon log:\n%s", err, out, serveLog.String())
	}

	// The already-indexed file answers immediately with a citation.
	hits := query(t, base, "heron report")
	if len(hits) == 0 || !strings.HasSuffix(hits[0].Path, "pre.md") {
		t.Fatalf("query missed pre-indexed file: %+v", hits)
	}

	// Definition of done: drop a file into the watched folder and it is
	// searchable within seconds, through the API, with a citation.
	if err := os.WriteFile(filepath.Join(docs, "dropped.md"),
		[]byte("# New\n\nThe ocelot invoice arrived while the daemon watched."), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		hits = query(t, base, "ocelot invoice")
		if len(hits) > 0 && strings.HasSuffix(hits[0].Path, "dropped.md") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dropped file not searchable after 15s; last results: %+v\ndaemon log:\n%s",
				hits, serveLog.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
	if hits[0].Snippet == "" {
		t.Error("result has no snippet")
	}

	// The CLI shows the same result with its citation formatting.
	search := exec.Command(bin, "search", "ocelot invoice")
	search.Env = env
	out, err := search.CombinedOutput()
	if err != nil {
		t.Fatalf("hay search: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "dropped.md · chunk") {
		t.Errorf("CLI search lacks citation:\n%s", out)
	}

	// status reports the daemon and the verifiable-privacy line.
	status := exec.Command(bin, "status")
	status.Env = env
	out, err = status.CombinedOutput()
	if err != nil {
		t.Fatalf("hay status: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Daemon:   running") ||
		!strings.Contains(string(out), "Outbound connections:") {
		t.Errorf("status output unexpected:\n%s", out)
	}
}

type hit struct {
	Path    string `json:"path"`
	Page    int    `json:"page"`
	Chunk   int    `json:"chunk"`
	Snippet string `json:"snippet"`
}

func query(t *testing.T, base, q string) []hit {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"query": q})
	resp, err := http.Post(base+"/api/query", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Results []hit `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("query decode: %v", err)
	}
	return out.Results
}

// waitForDaemon polls for the runtime file and a healthy answer, and
// returns the daemon's address.
func waitForDaemon(t *testing.T, home string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(home, "daemon.json"))
		if err == nil {
			var rt struct {
				Addr string `json:"addr"`
			}
			if json.Unmarshal(data, &rt) == nil && rt.Addr != "" {
				resp, err := http.Get(fmt.Sprintf("http://%s/api/health", rt.Addr))
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return rt.Addr
					}
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("daemon never became healthy")
	return ""
}
