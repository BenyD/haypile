package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/pathnorm"
)

func forceFold(t *testing.T, v bool) {
	t.Helper()
	old := pathnorm.CaseInsensitive
	pathnorm.CaseInsensitive = v
	t.Cleanup(func() { pathnorm.CaseInsensitive = old })
}

// On a case-insensitive filesystem, adding a folder under any casing must
// register one source with the on-disk casing, and re-adding it under
// another casing must not double-index anything.
func TestIndexFolderCanonicalizesCasing(t *testing.T) {
	forceFold(t, true)

	st, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	tmp := t.TempDir()
	docs := filepath.Join(tmp, "Docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "note.md"), []byte("indexed body text"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add with the "wrong" casing, the way a Windows user retypes paths.
	lower := filepath.Join(tmp, "docs")
	stats, err := IndexFolder(st, lower, "", nil, nil)
	if err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}
	if stats.Indexed != 1 {
		t.Fatalf("indexed %d files, want 1", stats.Indexed)
	}

	sources, err := st.Sources()
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(sources))
	}
	if sources[0].Path != docs {
		t.Errorf("source stored as %q, want the on-disk casing %q", sources[0].Path, docs)
	}

	// Re-add under a third casing: same source, file skipped as unchanged.
	stats, err = IndexFolder(st, strings.ToUpper(lower), "", nil, nil)
	if err != nil {
		t.Fatalf("IndexFolder re-add: %v", err)
	}
	if stats.Indexed != 0 || stats.Skipped != 1 {
		t.Errorf("re-add: indexed=%d skipped=%d, want 0 indexed and 1 skipped", stats.Indexed, stats.Skipped)
	}
	sources, err = st.Sources()
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(sources) != 1 || sources[0].Files != 1 {
		t.Errorf("after re-add: %+v, want one source with one file", sources)
	}
}
