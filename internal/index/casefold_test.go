package index

import (
	"testing"

	"github.com/BenyD/haypile/internal/pathnorm"
)

// forceFold flips path comparison into Windows (case-insensitive) mode for
// one test, so the folding behavior is exercised on every platform.
func forceFold(t *testing.T, v bool) {
	t.Helper()
	old := pathnorm.CaseInsensitive
	pathnorm.CaseInsensitive = v
	t.Cleanup(func() { pathnorm.CaseInsensitive = old })
}

func TestSourceLookupsFoldCase(t *testing.T) {
	forceFold(t, true)
	st := openTestStore(t)

	id, err := st.AddSource("/Data/Docs", "work")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	// Re-adding under another casing reuses the row instead of creating a
	// second source.
	id2, err := st.AddSource("/data/docs", "personal")
	if err != nil {
		t.Fatalf("AddSource other casing: %v", err)
	}
	if id2 != id {
		t.Errorf("AddSource created a second source: id %d then %d", id, id2)
	}
	sources, err := st.Sources()
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(sources))
	}
	// The stored path keeps its original, display-faithful casing; only
	// the tag was updated.
	if sources[0].Path != "/Data/Docs" || sources[0].Tag != "personal" {
		t.Errorf("source = %+v, want original casing with new tag", sources[0])
	}

	if got, err := st.SourceID("/DATA/DOCS"); err != nil || got != id {
		t.Errorf("SourceID other casing = %d, %v; want %d", got, err, id)
	}
	if tag, err := st.SourceTag("/data/DOCS"); err != nil || tag != "personal" {
		t.Errorf("SourceTag other casing = %q, %v; want personal", tag, err)
	}

	removed, err := st.RemoveSource("/data/docs")
	if err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	if !removed {
		t.Error("RemoveSource under another casing reported not indexed")
	}
	if sources, _ := st.Sources(); len(sources) != 0 {
		t.Errorf("%d sources left after remove, want 0", len(sources))
	}
}

func TestSourceLookupsStayExactOnCaseSensitiveFS(t *testing.T) {
	forceFold(t, false)
	st := openTestStore(t)

	id, err := st.AddSource("/Data/Docs", "")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	id2, err := st.AddSource("/data/docs", "")
	if err != nil {
		t.Fatalf("AddSource other casing: %v", err)
	}
	if id2 == id {
		t.Error("differently-cased paths must be distinct sources on a case-sensitive filesystem")
	}
	if _, err := st.SourceID("/DATA/DOCS"); err == nil {
		t.Error("SourceID must not fold case on a case-sensitive filesystem")
	}
	if removed, err := st.RemoveSource("/DATA/DOCS"); err != nil || removed {
		t.Errorf("RemoveSource folded case: removed=%v, err=%v", removed, err)
	}
}

func TestRemoveSourceCleansLegacyCasingDuplicates(t *testing.T) {
	// An index written before canonicalization can hold the same folder
	// twice under different casings. Simulate it with folding off, then
	// remove with folding on: one remove must clean up both rows.
	forceFold(t, false)
	st := openTestStore(t)

	id1, err := st.AddSource("/Data/Docs", "")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	id2, err := st.AddSource("/data/docs", "")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := st.UpsertFile(id1, "/Data/Docs/a.md", "sha1", 1, 1, chunksOf("alpha")); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}
	if err := st.UpsertFile(id2, "/data/docs/a.md", "sha2", 1, 1, chunksOf("alpha")); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	forceFold(t, true)
	removed, err := st.RemoveSource("/data/DOCS")
	if err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	if !removed {
		t.Fatal("RemoveSource found nothing")
	}
	sources, err := st.Sources()
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("%d sources left, want 0: %+v", len(sources), sources)
	}
	if sha, _ := st.FileSHA("/Data/Docs/a.md"); sha != "" {
		t.Error("file row from the first casing survived the remove")
	}
	if sha, _ := st.FileSHA("/data/docs/a.md"); sha != "" {
		t.Error("file row from the second casing survived the remove")
	}
}
