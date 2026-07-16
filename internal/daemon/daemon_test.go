package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startDaemon runs a full daemon on an ephemeral port against a temp
// HAYPILE_DIR and returns a client for it.
func startDaemon(t *testing.T) *Client {
	t.Helper()
	t.Setenv("HAYPILE_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, "127.0.0.1:0", "test") }()
	t.Cleanup(func() {
		cancel()
		if err := <-errc; err != nil {
			t.Errorf("daemon exited with error: %v", err)
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c := Discover(); c != nil {
			return c
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("daemon did not become healthy within 5s")
	return nil
}

func writeDoc(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// waitForHit polls the query API until the expected path shows up (or not,
// when expect is false). This is the M3 definition of done: changes are
// searchable within seconds, not on the next manual add.
func waitForHit(t *testing.T, c *Client, q, pathSuffix string, expect bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		results, err := c.Query(q, "", 10)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		found := false
		for _, r := range results {
			if strings.HasSuffix(r.Path, pathSuffix) {
				found = true
			}
		}
		if found == expect {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%q: expected %s in results = %v within 10s", q, pathSuffix, expect)
}

func TestDaemonAPIRoundTrip(t *testing.T) {
	c := startDaemon(t)

	docs := t.TempDir()
	writeDoc(t, docs, "note.md", "# Plan\n\nShip the walrus feature by Friday.")

	stats, err := c.AddSource(docs, "work")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if stats.Indexed != 1 {
		t.Fatalf("stats = %+v, want 1 indexed", stats)
	}

	results, err := c.Query("walrus feature", "", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 || !strings.HasSuffix(results[0].Path, "note.md") {
		t.Fatalf("results = %+v, want note.md", results)
	}

	sources, err := c.Sources()
	if err != nil || len(sources) != 1 || sources[0].Tag != "work" {
		t.Fatalf("Sources = %+v (err %v), want one tagged source", sources, err)
	}

	st, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.OK || st.Files != 1 || st.Chunks == 0 {
		t.Fatalf("Status = %+v, want OK with counts", st)
	}

	removed, err := c.RemoveSource(docs)
	if err != nil || !removed {
		t.Fatalf("RemoveSource = %v, %v; want true", removed, err)
	}
	results, _ = c.Query("walrus feature", "", 5)
	if len(results) != 0 {
		t.Fatalf("removed source still searchable: %+v", results)
	}
}

func TestDaemonWatchesForChanges(t *testing.T) {
	c := startDaemon(t)

	docs := t.TempDir()
	writeDoc(t, docs, "first.md", "The aardvark memo predates the watcher.")
	if _, err := c.AddSource(docs, ""); err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	// Definition of done: drop a file in a watched folder → searchable in
	// seconds, no manual re-add.
	writeDoc(t, docs, "dropped.md", "The capybara contract arrived after indexing.")
	waitForHit(t, c, "capybara contract", "dropped.md", true)

	// Modify: new content must replace the old.
	writeDoc(t, docs, "dropped.md", "Now it discusses the pangolin amendment instead.")
	waitForHit(t, c, "pangolin amendment", "dropped.md", true)
	waitForHit(t, c, "capybara contract", "dropped.md", false)

	// Delete: the file must leave the index.
	if err := os.Remove(filepath.Join(docs, "dropped.md")); err != nil {
		t.Fatal(err)
	}
	waitForHit(t, c, "pangolin amendment", "dropped.md", false)

	// New subdirectory: its files must be picked up too.
	sub := filepath.Join(docs, "deeper")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, sub, "nested.md", "The quokka rider hides in a new subfolder.")
	waitForHit(t, c, "quokka rider", "nested.md", true)
}

func TestDaemonRejectsBadRequests(t *testing.T) {
	c := startDaemon(t)

	post := func(path, body string) int {
		t.Helper()
		resp, err := http.Post(c.base+path, "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("/api/query", `{"query": ""}`); code != http.StatusBadRequest {
		t.Errorf("empty query returned %d, want 400", code)
	}
	if code := post("/api/query", `{not json`); code != http.StatusBadRequest {
		t.Errorf("bad json returned %d, want 400", code)
	}
	if code := post("/api/sources", fmt.Sprintf(`{"path": %q}`, filepath.Join(t.TempDir(), "nope"))); code != http.StatusBadRequest {
		t.Errorf("missing folder returned %d, want 400", code)
	}
}

func TestDiscoverRejectsWrongDatabase(t *testing.T) {
	c := startDaemon(t)
	if c == nil {
		t.Fatal("no daemon")
	}
	// Point HAYPILE_DIR elsewhere: the runtime file is not there, and even
	// a copied one would fail the DB identity check.
	t.Setenv("HAYPILE_DIR", t.TempDir())
	if got := Discover(); got != nil {
		t.Fatal("Discover found a daemon for a different HAYPILE_DIR")
	}

	// Health lies about nothing: a runtime file pointing at the live
	// daemon still fails because its DB path differs.
	rt := Runtime{PID: os.Getpid(), Addr: strings.TrimPrefix(c.base, "http://")}
	data, _ := json.Marshal(rt)
	dir := filepath.Dir(filepath.Join(os.Getenv("HAYPILE_DIR"), "x"))
	if err := os.WriteFile(filepath.Join(dir, runtimeFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Discover(); got != nil {
		t.Fatal("Discover trusted a daemon serving a different database")
	}
}
