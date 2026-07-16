package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BenyD/haypile/internal/index"
)

func openTestStore(t *testing.T) *index.Store {
	t.Helper()
	st, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSupported(t *testing.T) {
	for path, want := range map[string]bool{
		"notes.md":      true,
		"NOTES.MD":      true,
		"paper.txt":     true,
		"deal.docx":     true,
		"scan.PDF":      true,
		"photo.jpg":     false,
		"archive.zip":   false,
		"extensionless": false,
	} {
		if got := Supported(path); got != want {
			t.Errorf("Supported(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestExtractMarkdownSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	content := `Intro before any heading.

# Termination

Either party may cancel with notice.

## Details

Notice goes by mail.

` + "```\n# not a heading, just code\n```" + `

# Payment

Net-45.`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 4 {
		var heads []string
		for _, s := range secs {
			heads = append(heads, strings.SplitN(s.Text, "\n", 2)[0])
		}
		t.Fatalf("got %d sections %v, want 4 (intro, termination, details, payment)", len(secs), heads)
	}
	if !strings.HasPrefix(secs[1].Text, "# Termination") {
		t.Errorf("section 1 starts %q, want the Termination heading", strings.SplitN(secs[1].Text, "\n", 2)[0])
	}
	if !strings.Contains(secs[2].Text, "# not a heading, just code") {
		t.Error("fenced code block was wrongly treated as a section boundary")
	}
	for _, s := range secs {
		if s.Page != 0 {
			t.Errorf("markdown section has page %d, want 0", s.Page)
		}
	}
}

func TestExtractDocx(t *testing.T) {
	secs, err := Extract("testdata/contract.docx")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Title, Termination, and Payment Terms headings each start a section.
	if len(secs) != 3 {
		t.Fatalf("got %d sections, want 3", len(secs))
	}

	all := ""
	for _, s := range secs {
		all += s.Text + "\n"
		if s.Page != 0 {
			t.Errorf("docx section has page %d, want 0 (docx has no static pages)", s.Page)
		}
	}
	for _, phrase := range []string{
		"Master Services Agreement",
		"sixty days written notice",
		"Notice must be sent by certified mail.", // tab must become a space
		"net-45",
	} {
		if !strings.Contains(all, phrase) {
			t.Errorf("extracted docx text is missing %q", phrase)
		}
	}
	if !strings.HasPrefix(secs[1].Text, "Termination") {
		t.Errorf("section 1 must start with its heading, got %q", strings.SplitN(secs[1].Text, "\n", 2)[0])
	}
}

func TestExtractDocxRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.docx")
	if err := os.WriteFile(path, []byte("this is not a zip file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(path); err == nil {
		t.Fatal("want an error for a non-zip .docx")
	}
}

func TestExtractPDFPages(t *testing.T) {
	secs, err := Extract("testdata/contract.pdf")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 3 {
		t.Fatalf("got %d sections, want 3 (one per page)", len(secs))
	}
	for i, s := range secs {
		if s.Page != i+1 {
			t.Errorf("section %d has page %d, want %d", i, s.Page, i+1)
		}
	}
	if !strings.Contains(secs[0].Text, "terminated by either party") {
		t.Errorf("page 1 text missing termination clause, got %q", secs[0].Text)
	}
	if !strings.Contains(secs[1].Text, "2024-CV-01847") {
		t.Errorf("page 2 text missing case number, got %q", secs[1].Text)
	}
	if strings.TrimSpace(secs[2].Text) != "" {
		t.Errorf("page 3 is empty in the fixture, got %q", secs[2].Text)
	}
}

func TestExtractPDFRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 truncated garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(path); err == nil {
		t.Fatal("want an error for a corrupt PDF")
	}
}

func TestIndexSingleFile(t *testing.T) {
	st := openTestStore(t)

	stats, err := IndexFolder(st, "testdata/contract.pdf", "", nil, nil)
	if err != nil {
		t.Fatalf("IndexFolder(single file): %v", err)
	}
	if stats.Indexed != 1 || stats.Chunks == 0 {
		t.Fatalf("stats = %+v, want one indexed file with chunks", stats)
	}

	results, err := st.Search("2024-CV-01847", "", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 || results[0].Page != 2 {
		t.Fatalf("results = %+v, want the case number cited on page 2", results)
	}

	// Re-adding the same file must skip, not re-index.
	stats, err = IndexFolder(st, "testdata/contract.pdf", "", nil, nil)
	if err != nil || stats.Skipped != 1 {
		t.Fatalf("re-add: stats = %+v, err = %v; want Skipped 1", stats, err)
	}
}

func TestIndexSingleFileUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(path, []byte{0xFF, 0xD8}, 0o644); err != nil {
		t.Fatal(err)
	}
	st := openTestStore(t)
	if _, err := IndexFolder(st, path, "", nil, nil); err == nil {
		t.Fatal("want an error when a named file has an unsupported format")
	}
}

// TestIndexFolderSkipsBrokenFiles is the failure-tolerance contract: one
// unreadable document must not abort the folder pass.
func TestIndexFolderSkipsBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.md"), []byte("# Fine\n\nSearchable text."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.pdf"), []byte("not a pdf at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := openTestStore(t)
	stats, err := IndexFolder(st, dir, "", nil, nil)
	if err != nil {
		t.Fatalf("IndexFolder: %v", err)
	}
	if stats.Indexed != 1 || stats.Failed != 1 {
		t.Fatalf("stats = %+v, want Indexed 1, Failed 1", stats)
	}
}
