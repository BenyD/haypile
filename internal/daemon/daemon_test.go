package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// TestDaemonReconcilesOnConfigEdit: editing .haypile.yml by hand while
// the daemon runs re-syncs the index — newly excluded files drop out,
// removing the pattern brings them back. No re-add, no restart.
func TestDaemonReconcilesOnConfigEdit(t *testing.T) {
	c := startDaemon(t)

	docs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(docs, "drafts"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, docs, "final.md", "the signed lemur accord")
	writeDoc(t, filepath.Join(docs, "drafts"), "wip.md", "the draft lemur accord")
	if _, err := c.AddSource(docs, ""); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	waitForHit(t, c, "lemur accord", "wip.md", true)

	// Hand-edit the config: exclude drafts. The watcher must reconcile.
	writeDoc(t, docs, ".haypile.yml", "exclude:\n  - drafts/**\n")
	waitForHit(t, c, "lemur accord", "wip.md", false)
	waitForHit(t, c, "lemur accord", "final.md", true) // the rest survives

	// Remove the exclusion: the draft returns.
	writeDoc(t, docs, ".haypile.yml", "exclude: []\n")
	waitForHit(t, c, "lemur accord", "wip.md", true)
}

// TestMCPRoundTrip drives the daemon's MCP endpoint the way an editor
// would: initialize, list tools, call search, and read cited passages.
func TestMCPRoundTrip(t *testing.T) {
	c := startDaemon(t)

	docs := t.TempDir()
	writeDoc(t, docs, "contract.md", "# Deal\n\nThe indemnity cap is two million dollars.")
	if _, err := c.AddSource(docs, ""); err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	rpc := func(body string) map[string]any {
		t.Helper()
		resp, err := http.Post(c.MCPURL(), "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusAccepted {
			return nil
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	init := rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	result, _ := init["result"].(map[string]any)
	if result == nil || result["serverInfo"].(map[string]any)["name"] != "haypile" {
		t.Fatalf("initialize = %v", init)
	}
	if got := rpc(`{"jsonrpc":"2.0","method":"notifications/initialized"}`); got != nil {
		t.Fatalf("notification must return no body, got %v", got)
	}

	list := rpc(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools/list returned %d tools, want 2", len(tools))
	}

	call := rpc(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_documents","arguments":{"query":"indemnity cap"}}}`)
	content := call["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "contract.md") || !strings.Contains(text, "two million") {
		t.Fatalf("search_documents content unexpected:\n%s", text)
	}

	bad := rpc(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_documents","arguments":{}}}`)
	if isErr, _ := bad["result"].(map[string]any)["isError"].(bool); !isErr {
		t.Fatalf("empty query must be a tool error, got %v", bad)
	}

	unknown := rpc(`{"jsonrpc":"2.0","id":5,"method":"no/such"}`)
	if unknown["error"] == nil {
		t.Fatalf("unknown method must be a JSON-RPC error, got %v", unknown)
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

// rawGet issues a request against the daemon with a spoofed Host header,
// as a DNS-rebinding page would.
func rawGet(t *testing.T, c *Client, path, hostHeader string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hostHeader != "" {
		req.Host = hostHeader
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestHostGuardBlocksRebinding(t *testing.T) {
	c := startDaemon(t)

	if resp := rawGet(t, c, "/api/health", "evil.example.com"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("spoofed Host got %d, want 403", resp.StatusCode)
	}
	// Legit local names keep working, port or not.
	for _, h := range []string{"", "localhost:11500", "127.0.0.1:11500", "localhost"} {
		if resp := rawGet(t, c, "/api/health", h); resp.StatusCode != http.StatusOK {
			t.Errorf("Host %q got %d, want 200", h, resp.StatusCode)
		}
	}
	// The MCP surface is guarded too.
	if resp := rawGet(t, c, "/mcp", "evil.example.com"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("spoofed Host on /mcp got %d, want 403", resp.StatusCode)
	}
}

// fakeChatServer is a minimal OpenAI-compatible endpoint: lists one chat
// model and streams a canned answer.
func fakeChatServer(t *testing.T, answer string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			fmt.Fprint(w, `{"data":[{"id":"fake-chat"}]}`)
		case "/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			for _, word := range strings.SplitAfter(answer, " ") {
				b, _ := json.Marshal(word)
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", b)
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// sseEvents parses an SSE body into (event, data) pairs.
func sseEvents(t *testing.T, body io.Reader) [][2]string {
	t.Helper()
	var events [][2]string
	var name string
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			name = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			events = append(events, [2]string{name, strings.TrimPrefix(line, "data: ")})
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading SSE: %v", err)
	}
	return events
}

func TestAskStreamsAnswerWithSources(t *testing.T) {
	llm := fakeChatServer(t, "Sixty days written notice applies [1].")
	t.Setenv("HAYPILE_LLM_ENDPOINT", llm.URL)

	c := startDaemon(t)
	docs := t.TempDir()
	writeDoc(t, docs, "contract.md", "# Termination\n\nEither party may terminate with sixty days written notice.")
	if _, err := c.AddSource(docs, ""); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	waitForHit(t, c, "sixty days notice", "contract.md", true)

	resp, err := http.Post(c.base+"/api/ask", "application/json",
		strings.NewReader(`{"question": "what notice period applies?"}`))
	if err != nil {
		t.Fatalf("POST /api/ask: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ask returned %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	events := sseEvents(t, resp.Body)
	if len(events) < 3 {
		t.Fatalf("only %d events: %v", len(events), events)
	}
	if events[0][0] != "sources" {
		t.Errorf("first event = %s, want sources", events[0][0])
	}
	var sources []AskSource
	if err := json.Unmarshal([]byte(events[0][1]), &sources); err != nil || len(sources) == 0 {
		t.Fatalf("bad sources payload %q: %v", events[0][1], err)
	}
	if !strings.HasSuffix(sources[0].Path, "contract.md") || sources[0].Label == "" {
		t.Errorf("source missing citation: %+v", sources[0])
	}

	var answer strings.Builder
	sawDone := false
	for _, ev := range events[1:] {
		switch ev[0] {
		case "token":
			var tok string
			if err := json.Unmarshal([]byte(ev[1]), &tok); err != nil {
				t.Fatalf("bad token %q: %v", ev[1], err)
			}
			answer.WriteString(tok)
		case "done":
			sawDone = true
		case "error":
			t.Fatalf("unexpected error event: %s", ev[1])
		}
	}
	if got := answer.String(); got != "Sixty days written notice applies [1]." {
		t.Errorf("streamed answer = %q", got)
	}
	if !sawDone {
		t.Error("stream did not end with done")
	}
}

func TestAskWithoutLLMIs503(t *testing.T) {
	// An explicit endpoint that answers nothing: Detect must fail fast.
	t.Setenv("HAYPILE_LLM_ENDPOINT", "http://127.0.0.1:1")

	c := startDaemon(t)
	resp, err := http.Post(c.base+"/api/ask", "application/json",
		strings.NewReader(`{"question": "anything"}`))
	if err != nil {
		t.Fatalf("POST /api/ask: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Error == "" {
		t.Fatalf("503 without an explanation: %v (%v)", body, err)
	}
}

func TestChunkContextAPI(t *testing.T) {
	c := startDaemon(t)
	docs := t.TempDir()
	path := writeDoc(t, docs, "notes.md", "# One\n\nfirst section\n\n# Two\n\nsecond section\n\n# Three\n\nthird section")
	if _, err := c.AddSource(docs, ""); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	waitForHit(t, c, "second section", "notes.md", true)

	resp := rawGet(t, c, "/api/chunk?chunk=1&path="+url.QueryEscape(path), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chunk API returned %d", resp.StatusCode)
	}
	var out struct {
		Path     string `json:"path"`
		Passages []struct {
			Chunk   int    `json:"chunk"`
			Text    string `json:"text"`
			Current bool   `json:"current"`
		} `json:"passages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Passages) != 3 {
		t.Fatalf("got %d passages, want 3 (neighbors both sides): %+v", len(out.Passages), out)
	}
	if !out.Passages[1].Current || !strings.Contains(out.Passages[1].Text, "second section") {
		t.Errorf("middle passage wrong: %+v", out.Passages[1])
	}

	if resp := rawGet(t, c, "/api/chunk?chunk=99&path="+url.QueryEscape(path), ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("out-of-range chunk got %d, want 404", resp.StatusCode)
	}
	if resp := rawGet(t, c, "/api/chunk?chunk=0", ""); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing path got %d, want 400", resp.StatusCode)
	}
}

func TestDaemonServesWebUI(t *testing.T) {
	c := startDaemon(t)

	resp := rawGet(t, c, "/", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `id="root"`) {
		t.Error("web UI shell not served at /")
	}

	// The UI sits behind the host guard like everything else.
	if resp := rawGet(t, c, "/", "evil.example.com"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("spoofed Host on / got %d, want 403", resp.StatusCode)
	}
	// API 404s stay JSON errors, not the UI fallback.
	if resp := rawGet(t, c, "/api/nope", ""); strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Error("unknown /api route fell through to the web UI")
	}
}

func TestBrowseAPI(t *testing.T) {
	c := startDaemon(t)

	dir := t.TempDir()
	for _, d := range []string{"beta", "alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc(t, dir, "notes.txt", "indexable")
	writeDoc(t, dir, "photo.jpg", "not indexable")

	resp := rawGet(t, c, "/api/browse?path="+url.QueryEscape(dir), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("browse returned %d", resp.StatusCode)
	}
	var out struct {
		Path   string `json:"path"`
		Parent string `json:"parent"`
		Dirs   []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"dirs"`
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Path != dir || out.Parent == "" {
		t.Errorf("path/parent wrong: %+v", out)
	}
	// Sorted, dotfolders hidden.
	if len(out.Dirs) != 2 || out.Dirs[0].Name != "alpha" || out.Dirs[1].Name != "beta" {
		t.Errorf("dirs = %+v, want [alpha beta]", out.Dirs)
	}
	// Only indexable files are offered; the jpg stays invisible.
	if len(out.Files) != 1 || out.Files[0].Name != "notes.txt" {
		t.Errorf("files = %+v, want [notes.txt]", out.Files)
	}

	if resp := rawGet(t, c, "/api/browse?path=relative/nope", ""); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("relative path got %d, want 400", resp.StatusCode)
	}
	// No path browses the home directory.
	if resp := rawGet(t, c, "/api/browse", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("default browse got %d, want 200", resp.StatusCode)
	}
}

// TestOriginGuardBlocksCrossSite: a web page on any site can fire blind
// POSTs at localhost APIs (classic CSRF); the browser stamps them with
// the page's Origin, and the daemon must refuse foreign ones.
func TestOriginGuardBlocksCrossSite(t *testing.T) {
	c := startDaemon(t)

	post := func(origin string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, c.base+"/api/sources",
			strings.NewReader(`{"path": "/tmp/nope"}`))
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("https://evil.example.com"); code != http.StatusForbidden {
		t.Errorf("cross-site origin got %d, want 403", code)
	}
	// Same-origin from the web UI and dev-server origins stay allowed
	// (they fail on the bogus path, not on the origin).
	for _, o := range []string{"", "http://localhost:11500", "http://127.0.0.1:5173"} {
		if code := post(o); code == http.StatusForbidden {
			t.Errorf("origin %q was wrongly forbidden", o)
		}
	}
}

// TestPickAPI drives the native-picker endpoint with the dialog stubbed
// out; real dialogs have no place in CI.
func TestPickAPI(t *testing.T) {
	c := startDaemon(t)

	orig := nativePick
	t.Cleanup(func() { nativePick = orig })

	post := func(query string) *http.Response {
		t.Helper()
		resp, err := http.Post(c.base+"/api/pick"+query, "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	nativePick = func(ctx context.Context, kind string) (string, error) {
		return "/picked/" + kind, nil
	}
	resp := post("?kind=folder")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pick returned %d", resp.StatusCode)
	}
	var out struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Path != "/picked/folder" {
		t.Fatalf("path = %q, err %v", out.Path, err)
	}

	nativePick = func(ctx context.Context, kind string) (string, error) {
		return "", errPickCanceled
	}
	if resp := post("?kind=file"); resp.StatusCode != http.StatusNoContent {
		t.Errorf("cancel returned %d, want 204", resp.StatusCode)
	}

	nativePick = func(ctx context.Context, kind string) (string, error) {
		return "", errors.New("no display")
	}
	if resp := post("?kind=folder"); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("unsupported returned %d, want 501", resp.StatusCode)
	}

	if resp := post("?kind=everything"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad kind returned %d, want 400", resp.StatusCode)
	}
}
