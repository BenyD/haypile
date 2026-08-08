package daemon

import (
	"path/filepath"
	"testing"

	"github.com/BenyD/haypile/internal/pathnorm"
)

func forceFold(t *testing.T, v bool) {
	t.Helper()
	old := pathnorm.CaseInsensitive
	pathnorm.CaseInsensitive = v
	t.Cleanup(func() { pathnorm.CaseInsensitive = old })
}

// ownerOf only reads the sources map under its own lock, so a bare struct
// is enough — no fsnotify, no goroutines.
func TestOwnerOfFoldsCase(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + "Data" + sep + "Docs"
	nested := filepath.Join(root, "Deep")
	w := &watcher{sources: map[string]int64{root: 1, nested: 2}}

	forceFold(t, true)
	// The OS may report event paths in a casing that differs from the
	// registered root; those events must still find their source.
	id, got, ok := w.ownerOf(sep + "data" + sep + "docs" + sep + "a.txt")
	if !ok || id != 1 || got != root {
		t.Errorf("ownerOf folded child = (%d, %q, %v), want (1, %q, true)", id, got, ok, root)
	}
	// Longest root still wins under folding.
	id, _, ok = w.ownerOf(sep + "DATA" + sep + "DOCS" + sep + "DEEP" + sep + "b.txt")
	if !ok || id != 2 {
		t.Errorf("nested source lost to its parent: id=%d ok=%v", id, ok)
	}
	// A sibling sharing a string prefix stays outside.
	if _, _, ok := w.ownerOf(sep + "data" + sep + "docs2" + sep + "c.txt"); ok {
		t.Error("ownerOf matched a sibling folder")
	}

	forceFold(t, false)
	if _, _, ok := w.ownerOf(sep + "data" + sep + "docs" + sep + "a.txt"); ok {
		t.Error("ownerOf folded case on a case-sensitive filesystem")
	}
}

func TestUnwatchSourceFoldsCase(t *testing.T) {
	forceFold(t, true)
	sep := string(filepath.Separator)
	// A legacy daemon can hold the same root under two casings; one
	// unwatch drops both.
	w := &watcher{sources: map[string]int64{
		sep + "Data" + sep + "Docs": 1,
		sep + "data" + sep + "docs": 1,
		sep + "Other":               2,
	}}
	w.unwatchSource(sep + "DATA" + sep + "DOCS")
	if len(w.sources) != 1 {
		t.Fatalf("sources left = %v, want only /Other", w.sources)
	}
	if _, ok := w.sources[sep+"Other"]; !ok {
		t.Error("unwatch removed an unrelated source")
	}
}
