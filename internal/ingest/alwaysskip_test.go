package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BenyD/haypile/internal/index"
)

// A code project's dependency tree is full of markdown that is not the
// user's writing. Walking into it buries real documents in thousands of
// dependency READMEs, so those directories are skipped whatever the
// config says.
func TestExcludedSkipsMachineDirectories(t *testing.T) {
	var cfg Config
	for _, rel := range []string{
		"node_modules/undici/README.md",
		"containers/ai/node_modules/miniflare/README.md",
		"vendor/github.com/x/doc.md",
		"target/doc/index.html",
		"dist/readme.md",
		"__pycache__/notes.txt",
	} {
		if !cfg.Excluded(rel) {
			t.Errorf("%s must be excluded by default", rel)
		}
	}
	for _, rel := range []string{
		"README.md",
		"docs/architecture.md",
		"notes/node_modules.md", // a document about them, not one of them
	} {
		if cfg.Excluded(rel) {
			t.Errorf("%s must not be excluded", rel)
		}
	}
}

// End to end: a folder shaped like a real repo indexes its own docs and
// none of its dependencies'.
func TestIndexFolderSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "Lensdrop delivers photo galleries.")
	write("docs/design.md", "The upload pipeline resumes through network drops.")
	write("node_modules/undici/README.md", "Undici is an HTTP client.")
	write("packages/api/node_modules/x/README.md", "Some dependency.")
	write("dist/bundle.md", "Generated output.")

	st, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stats, err := IndexFolder(st, root, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Indexed != 2 {
		t.Errorf("indexed %d files, want 2 (README.md and docs/design.md)", stats.Indexed)
	}
	hits, err := st.Search("Undici HTTP client", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("dependency docs must not be searchable, got %d hits", len(hits))
	}
}
