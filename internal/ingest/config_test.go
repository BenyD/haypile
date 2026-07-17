package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigExcluded(t *testing.T) {
	cfg := Config{Exclude: []string{"drafts/**", "*.bak", "**/archive/**"}}
	tests := []struct {
		rel  string
		want bool
	}{
		{"drafts/one.md", true},
		{"drafts/deep/two.md", true},
		{"draft.md", false},
		{"notes.bak", true},
		{"sub/dir/notes.bak", true}, // bare-name pattern matches at depth
		{"old/archive/x.pdf", true},
		{"notes.md", false},
		{"archive.md", false},
	}
	for _, tt := range tests {
		if got := cfg.Excluded(tt.rel); got != tt.want {
			t.Errorf("Excluded(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Config{Tag: "acme", Exclude: []string{"drafts/**"}}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if out.Tag != "acme" || len(out.Exclude) != 1 || out.Exclude[0] != "drafts/**" {
		t.Fatalf("round trip mangled config: %+v", out)
	}
}

func TestLoadConfigMissingIsEmpty(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil || cfg.Tag != "" || len(cfg.Exclude) != 0 {
		t.Fatalf("missing config must be empty, got %+v, %v", cfg, err)
	}
}

func TestLoadConfigRejectsBadYAMLAndPatterns(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ConfigName), []byte("tag: [broken"), 0o644)
	if _, err := LoadConfig(dir); err == nil {
		t.Error("malformed YAML must error, not silently index everything")
	}
	os.WriteFile(filepath.Join(dir, ConfigName), []byte("exclude: [\"[bad\"]"), 0o644)
	if _, err := LoadConfig(dir); err == nil {
		t.Error("invalid glob pattern must error")
	}
}

// TestIndexFolderExcludesAndReconciles is the feature's contract: adding
// an exclude pattern and re-indexing drops the excluded files; removing
// it brings them back.
func TestIndexFolderExcludesAndReconciles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "drafts"), 0o755)
	os.WriteFile(filepath.Join(dir, "final.md"), []byte("the signed penguin agreement"), 0o644)
	os.WriteFile(filepath.Join(dir, "drafts", "wip.md"), []byte("the draft penguin agreement"), 0o644)

	st := openTestStore(t)

	// No config: both files indexed.
	stats, err := IndexFolder(st, dir, "", nil, nil)
	if err != nil || stats.Indexed != 2 {
		t.Fatalf("initial pass: %+v, %v", stats, err)
	}

	// Exclude drafts and re-index: the draft must be pruned.
	if err := (Config{Exclude: []string{"drafts/**"}}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := IndexFolder(st, dir, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	results, _ := st.Search("penguin agreement", "", 10)
	if len(results) != 1 || filepath.Base(results[0].Path) != "final.md" {
		t.Fatalf("after exclude: %+v, want only final.md", results)
	}

	// Remove the exclude: the draft returns.
	if err := (Config{}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := IndexFolder(st, dir, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	if results, _ = st.Search("penguin agreement", "", 10); len(results) != 2 {
		t.Fatalf("after removing exclude: %+v, want both files back", results)
	}
}

func TestIndexFolderUsesConfigTag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("tagged flamingo text"), 0o644)
	if err := (Config{Tag: "cases"}).Save(dir); err != nil {
		t.Fatal(err)
	}

	st := openTestStore(t)
	if _, err := IndexFolder(st, dir, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	if results, _ := st.Search("flamingo", "cases", 10); len(results) != 1 {
		t.Fatal("config tag was not applied")
	}

	// An explicit tag wins over the config.
	if _, err := IndexFolder(st, dir, "override", nil, nil); err != nil {
		t.Fatal(err)
	}
	if results, _ := st.Search("flamingo", "override", 10); len(results) != 1 {
		t.Fatal("explicit tag must override config tag")
	}
}
