package index

import (
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSearchFindsIndexedChunks(t *testing.T) {
	st := openTestStore(t)

	srcID, err := st.AddSource("/docs", "")
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	err = st.UpsertFile(srcID, "/docs/contract.md", "sha1", 100, 1, []string{
		"Either party may terminate this agreement with sixty days written notice.",
		"Payment is due within forty-five days of invoice.",
	})
	if err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	results, err := st.Search("terminate agreement", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Path != "/docs/contract.md" || results[0].Seq != 0 {
		t.Errorf("wrong citation: %+v", results[0])
	}
	if !strings.Contains(results[0].Snippet, "terminate") {
		t.Errorf("snippet missing match: %q", results[0].Snippet)
	}
}

func TestUpsertReplacesChunks(t *testing.T) {
	st := openTestStore(t)

	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "v1", 10, 1, []string{"old content about zebras"})
	st.UpsertFile(srcID, "/docs/a.md", "v2", 12, 2, []string{"new content about llamas"})

	if r, _ := st.Search("zebras", "", 10); len(r) != 0 {
		t.Errorf("stale chunk still searchable after upsert")
	}
	if r, _ := st.Search("llamas", "", 10); len(r) != 1 {
		t.Errorf("new chunk not searchable after upsert")
	}
}

func TestFileSHARoundTrip(t *testing.T) {
	st := openTestStore(t)

	if sha, _ := st.FileSHA("/docs/a.md"); sha != "" {
		t.Errorf("unknown file returned sha %q", sha)
	}
	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "abc123", 10, 1, []string{"text"})
	if sha, _ := st.FileSHA("/docs/a.md"); sha != "abc123" {
		t.Errorf("got sha %q, want abc123", sha)
	}
}

func TestRemoveSourceCascades(t *testing.T) {
	st := openTestStore(t)

	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "v1", 10, 1, []string{"searchable pelican text"})

	found, err := st.RemoveSource("/docs")
	if err != nil || !found {
		t.Fatalf("RemoveSource: found=%v err=%v", found, err)
	}
	if r, _ := st.Search("pelican", "", 10); len(r) != 0 {
		t.Errorf("chunks survive source removal")
	}
	if srcs, _ := st.Sources(); len(srcs) != 0 {
		t.Errorf("source survives removal")
	}

	if found, _ := st.RemoveSource("/nowhere"); found {
		t.Errorf("removing unknown source reported found")
	}
}

func TestTagFiltering(t *testing.T) {
	st := openTestStore(t)

	workID, _ := st.AddSource("/work", "work")
	homeID, _ := st.AddSource("/home", "home")
	st.UpsertFile(workID, "/work/a.md", "s1", 1, 1, []string{"quarterly heron report"})
	st.UpsertFile(homeID, "/home/b.md", "s2", 1, 1, []string{"heron watching notes"})

	if r, _ := st.Search("heron", "", 10); len(r) != 2 {
		t.Errorf("untagged search: got %d results, want 2", len(r))
	}
	r, _ := st.Search("heron", "work", 10)
	if len(r) != 1 || r[0].Path != "/work/a.md" {
		t.Errorf("tagged search wrong: %+v", r)
	}
}

func TestQuerySyntaxCannotBreakSearch(t *testing.T) {
	st := openTestStore(t)
	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "s", 1, 1, []string{"plain text"})

	// FTS5 operators and syntax in user input must be treated as literals,
	// never parsed — a search box that errors on quotes is broken.
	for _, q := range []string{`"unbalanced`, `NEAR(a b)`, `a AND b OR c`, `col:value`, `text*`} {
		if _, err := st.Search(q, "", 10); err != nil {
			t.Errorf("query %q returned error: %v", q, err)
		}
	}

	if _, err := st.Search("   ", "", 10); err == nil {
		t.Errorf("blank query should error")
	}
}
