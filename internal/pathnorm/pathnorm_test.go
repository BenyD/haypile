package pathnorm

import (
	"os"
	"path/filepath"
	"testing"
)

// forceFold flips the package into Windows (case-insensitive) mode for one
// test, so the folding paths run on every platform.
func forceFold(t *testing.T, v bool) {
	t.Helper()
	old := CaseInsensitive
	CaseInsensitive = v
	t.Cleanup(func() { CaseInsensitive = old })
}

func TestEqualCaseSensitive(t *testing.T) {
	forceFold(t, false)
	if !Equal("/data/docs", "/data/docs") {
		t.Error("identical paths must be equal")
	}
	if Equal("/data/Docs", "/data/docs") {
		t.Error("case must matter on a case-sensitive filesystem")
	}
}

func TestEqualCaseInsensitive(t *testing.T) {
	forceFold(t, true)
	if !Equal("/Data/DOCS", "/data/docs") {
		t.Error("case must fold on a case-insensitive filesystem")
	}
	if Equal("/data/docs", "/data/notes") {
		t.Error("different paths must stay unequal")
	}
}

func TestHasPrefix(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + "data" + sep + "docs"

	for _, tc := range []struct {
		name string
		fold bool
		path string
		want bool
	}{
		{"root itself", false, root, true},
		{"child", false, filepath.Join(root, "a.txt"), true},
		{"sibling sharing a string prefix", false, root + "2" + sep + "a.txt", false},
		{"other casing, sensitive", false, sep + "Data" + sep + "Docs" + sep + "a.txt", false},
		{"other casing, insensitive", true, sep + "Data" + sep + "Docs" + sep + "a.txt", true},
		{"root other casing, insensitive", true, sep + "DATA" + sep + "DOCS", true},
		{"sibling other casing, insensitive", true, sep + "Data" + sep + "Docs2" + sep + "a.txt", false},
		{"outside", true, sep + "elsewhere" + sep + "a.txt", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forceFold(t, tc.fold)
			if got := HasPrefix(tc.path, root); got != tc.want {
				t.Errorf("HasPrefix(%q, %q) = %v, want %v", tc.path, root, got, tc.want)
			}
		})
	}
}

func TestCanonResolvesOnDiskCasing(t *testing.T) {
	forceFold(t, true)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "Docs", "Sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(tmp, "Docs", "Sub", "Note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	lower := filepath.Join(tmp, "docs", "sub", "note.txt")
	got, err := Canon(lower)
	if err != nil {
		t.Fatalf("Canon: %v", err)
	}
	if got != file {
		t.Errorf("Canon(%q) = %q, want on-disk casing %q", lower, got, file)
	}
}

func TestCanonKeepsCasingOfMissingComponents(t *testing.T) {
	forceFold(t, true)
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "Docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	// "docs" exists (as "Docs") and resolves; the deleted tail keeps the
	// casing it was given.
	in := filepath.Join(tmp, "docs", "Gone", "File.txt")
	got, err := Canon(in)
	if err != nil {
		t.Fatalf("Canon: %v", err)
	}
	want := filepath.Join(tmp, "Docs", "Gone", "File.txt")
	if got != want {
		t.Errorf("Canon(%q) = %q, want %q", in, got, want)
	}
}

func TestCanonIsPlainAbsWhenCaseSensitive(t *testing.T) {
	forceFold(t, false)
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "Docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(tmp, "docs")
	got, err := Canon(in)
	if err != nil {
		t.Fatalf("Canon: %v", err)
	}
	if got != in {
		t.Errorf("Canon(%q) = %q, want the path untouched on a case-sensitive OS", in, got)
	}
}

func TestOnDiskNamePrefersExactMatch(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "Readme"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Skip("filesystem folds case; the two names are one file")
	}

	got, ok := onDiskName(tmp, "README")
	if !ok || got != "README" {
		t.Errorf("onDiskName = %q, %v; want exact match README", got, ok)
	}
	got, ok = onDiskName(tmp, "readme")
	if !ok {
		t.Fatal("onDiskName found nothing for a folded match")
	}
	if got != "Readme" && got != "README" {
		t.Errorf("onDiskName = %q, want one of the on-disk casings", got)
	}
}
