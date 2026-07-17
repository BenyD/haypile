package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain keeps unit tests hermetic: `hay add` must never fork a real
// daemon out of the test binary. The daemon path itself is covered by the
// end-to-end smoke test against a built binary.
func TestMain(m *testing.M) {
	os.Setenv("HAYPILE_NO_DAEMON", "1")
	os.Exit(m.Run())
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestRootRegistersAllCommands(t *testing.T) {
	want := []string{"add", "search", "ask", "list", "remove", "status", "serve", "mcp-stdio", "llm"}

	root := newRootCmd()
	got := make(map[string]bool)
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Errorf("command %q is not registered on root", name)
		}
	}
}

func TestHelpRunsWithoutError(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(out, "hay") {
		t.Errorf("help output does not mention the binary name; got:\n%s", out)
	}
}

// TestAskEndToEnd drives the full RAG path: index a doc, retrieve, send
// context to a fake OpenAI-compatible server, print answer and sources.
func TestAskEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "llama3.2"}}})
		case "/v1/chat/completions":
			json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "Sixty days written notice [1]."}},
			}})
		}
	}))
	defer srv.Close()

	t.Setenv("HAYPILE_DIR", t.TempDir())
	docs := t.TempDir()
	if err := os.WriteFile(filepath.Join(docs, "contract.md"),
		[]byte("Either party may terminate with sixty days written notice."), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "add", docs); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}

	out, err := run(t, "ask", "what notice period applies?", "--endpoint", srv.URL+"/v1")
	if err != nil {
		t.Fatalf("ask: %v\n%s", err, out)
	}
	for _, want := range []string{"Sixty days written notice [1].", "Sources:", "[1] ", "contract.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("ask output missing %q:\n%s", want, out)
		}
	}
}

// TestAskWithoutLLMFallsBackToSearch: no endpoint anywhere → explain and
// show passages, exit zero. Degraded, not broken.
func TestAskWithoutLLMFallsBackToSearch(t *testing.T) {
	if _, err := http.Get("http://localhost:11434/v1/models"); err == nil {
		t.Skip("a real local LLM server is running")
	}

	t.Setenv("HAYPILE_DIR", t.TempDir())
	docs := t.TempDir()
	if err := os.WriteFile(filepath.Join(docs, "note.md"),
		[]byte("The migration to the new billing system happens in June."), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "add", docs); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}

	out, err := run(t, "ask", "billing migration")
	if err != nil {
		t.Fatalf("ask without LLM must not error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no local LLM endpoint found") || !strings.Contains(out, "note.md") {
		t.Errorf("fallback output unexpected:\n%s", out)
	}
}

func TestStatusWithoutDaemon(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())
	out, err := run(t, "status")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if !strings.Contains(out, "not running") {
		t.Errorf("status without a daemon should say so:\n%s", out)
	}
}

func TestAddSearchListRemoveEndToEnd(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())

	docs := t.TempDir()
	err := os.WriteFile(filepath.Join(docs, "contract.md"),
		[]byte("# Services Agreement\n\nEither party may terminate with sixty days written notice.\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "add", docs)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Indexed 1 files (1 chunks)") {
		t.Errorf("add output unexpected:\n%s", out)
	}

	out, err = run(t, "search", "terminate notice")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "contract.md") {
		t.Errorf("search did not cite contract.md:\n%s", out)
	}

	// Re-adding without changes must skip, not re-index.
	out, _ = run(t, "add", docs)
	if !strings.Contains(out, "1 unchanged") {
		t.Errorf("re-add did not skip unchanged file:\n%s", out)
	}

	out, err = run(t, "list")
	if err != nil || !strings.Contains(out, docs) {
		t.Errorf("list missing source (err=%v):\n%s", err, out)
	}

	if out, err = run(t, "remove", docs); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	out, _ = run(t, "search", "terminate notice")
	if strings.Contains(out, "contract.md") {
		t.Errorf("removed folder still searchable:\n%s", out)
	}
}

// TestPDFCitationsEndToEnd is M2's definition of done as a test: search a
// folder of PDFs, results cite file and page.
func TestPDFCitationsEndToEnd(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())

	docs := t.TempDir()
	pdf, err := os.ReadFile("../ingest/testdata/contract.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "contract.pdf"), pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	docx, err := os.ReadFile("../ingest/testdata/contract.docx")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "deal.docx"), docx, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "add", docs)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Indexed 2 files") {
		t.Errorf("add output unexpected:\n%s", out)
	}

	// The case number lives on page 2 of the PDF.
	out, err = run(t, "search", "2024-CV-01847")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "contract.pdf (page 2)") {
		t.Errorf("PDF result must cite file and page:\n%s", out)
	}

	// The docx cites file + chunk (no static pages in Word documents).
	out, err = run(t, "search", "certified mail")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "deal.docx (chunk") {
		t.Errorf("docx result must cite the file:\n%s", out)
	}
}

// TestAddSingleFile: `hay add resume.pdf` indexes just that document.
func TestAddSingleFile(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())

	pdf, err := os.ReadFile("../ingest/testdata/contract.pdf")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "solo.pdf")
	if err := os.WriteFile(path, pdf, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "add", path)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Indexed 1 files") {
		t.Errorf("add output unexpected:\n%s", out)
	}

	out, err = run(t, "search", "2024-CV-01847")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "solo.pdf (page 2)") {
		t.Errorf("single-file add must be searchable with page citation:\n%s", out)
	}

	if out, err = run(t, "remove", path); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
}

// TestSemanticEndToEnd drives the whole env-configured semantic path: a
// fake OpenAI-compatible endpoint, hay add embedding chunks through it, and
// a paraphrase query that keyword search alone cannot answer.
func TestSemanticEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		data := make([]map[string]any, len(req.Input))
		for i, text := range req.Input {
			// Toy semantics: termination-ish texts share a direction.
			vec := []float32{0, 1}
			if strings.Contains(text, "terminate") || strings.Contains(text, "cancellation") {
				vec = []float32{1, 0}
			}
			data[i] = map[string]any{"index": i, "embedding": vec}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()

	t.Setenv("HAYPILE_DIR", t.TempDir())
	t.Setenv("HAYPILE_EMBED_ENDPOINT", srv.URL+"/v1")
	t.Setenv("HAYPILE_EMBED_MODEL", "fake-model")

	docs := t.TempDir()
	os.WriteFile(filepath.Join(docs, "contract.md"),
		[]byte("Either party may terminate with sixty days written notice.\n"), 0o644)
	os.WriteFile(filepath.Join(docs, "kitchen.md"),
		[]byte("Going with white oak cabinets for the renovation.\n"), 0o644)

	out, err := run(t, "add", docs)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Embedded 2 chunks") {
		t.Errorf("add did not embed:\n%s", out)
	}

	// No shared words with the contract — only the vector leg can find it.
	out, err = run(t, "search", "agreement cancellation")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "contract.md") {
		t.Errorf("semantic search failed to find paraphrase:\n%s", out)
	}
}
