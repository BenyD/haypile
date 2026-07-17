package ingest

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.html")
	content := `<!DOCTYPE html>
<html>
<head>
  <title>Ignore me</title>
  <style>.x { color: red; }</style>
  <script>var secret = "do not index";</script>
</head>
<body>
  <p>Intro before any heading.</p>
  <h1>Termination</h1>
  <p>Either party may
     cancel with notice.</p>
  <h2>Details</h2>
  <ul><li>Notice goes by mail.</li></ul>
</body>
</html>`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 3 {
		var heads []string
		for _, s := range secs {
			heads = append(heads, strings.SplitN(s.Text, "\n", 2)[0])
		}
		t.Fatalf("got %d sections %v, want 3 (intro, termination, details)", len(secs), heads)
	}

	all := ""
	for _, s := range secs {
		all += s.Text + "\n"
		if s.Page != 0 {
			t.Errorf("html section has page %d, want 0", s.Page)
		}
	}
	if strings.Contains(all, "do not index") || strings.Contains(all, "color: red") || strings.Contains(all, "Ignore me") {
		t.Errorf("script/style/head content leaked into extracted text: %q", all)
	}
	if !strings.HasPrefix(secs[1].Text, "Termination") {
		t.Errorf("section 1 must start with its heading, got %q", strings.SplitN(secs[1].Text, "\n", 2)[0])
	}
	// Whitespace across the line break inside the paragraph must collapse.
	if !strings.Contains(all, "Either party may cancel with notice.") {
		t.Errorf("paragraph whitespace not collapsed, got %q", all)
	}
}

// writeTestPptx builds a minimal but valid .pptx (a zip of slide parts) so
// tests need no committed binary fixture.
func writeTestPptx(t *testing.T, slides map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deck.pptx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, body := range slides {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func slideXML(paras ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><p:sld xmlns:a="http://a" xmlns:p="http://p"><p:cSld><p:spTree><p:sp><p:txBody>`)
	for _, p := range paras {
		b.WriteString(`<a:p><a:r><a:t>` + p + `</a:t></a:r></a:p>`)
	}
	b.WriteString(`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`)
	return b.String()
}

func TestExtractPptx(t *testing.T) {
	path := writeTestPptx(t, map[string]string{
		"ppt/slides/slide1.xml":            slideXML("Quarterly Revenue", "Up 30% over last year."),
		"ppt/slides/slide2.xml":            slideXML("Roadmap", "Ship OCR in v1.x."),
		"ppt/slides/slide10.xml":           slideXML("Thank You"),
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships/>`, // must be ignored
	})

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 3 {
		t.Fatalf("got %d sections, want 3 (one per slide)", len(secs))
	}

	// slide10 must sort after slide2, landing on page 3 (position order).
	for i, want := range []struct {
		page   int
		phrase string
	}{
		{1, "Quarterly Revenue"},
		{2, "Ship OCR in v1.x."},
		{3, "Thank You"},
	} {
		if secs[i].Page != want.page {
			t.Errorf("section %d has page %d, want %d", i, secs[i].Page, want.page)
		}
		if !strings.Contains(secs[i].Text, want.phrase) {
			t.Errorf("section %d missing %q, got %q", i, want.phrase, secs[i].Text)
		}
	}
}

func TestExtractPptxRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.pptx")
	if err := os.WriteFile(path, []byte("this is not a zip file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(path); err == nil {
		t.Fatal("want an error for a non-zip .pptx")
	}
}
