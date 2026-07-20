package ingest

import (
	"strings"
	"testing"
)

// run builds a textRun for a single 12pt-high line at the given top; text
// runs left to right from x with a loose 6pt-per-char width.
func run(text string, x, top float64) textRun {
	return textRun{
		text: text, left: x, right: x + 6*float64(len(text)),
		top: top, bottom: top - 12, fontSize: 10, fontName: "Helvetica",
	}
}

func TestAssemblePageParagraphBreaks(t *testing.T) {
	// Three tightly-leaded lines, a large gap, then two more: the gap is
	// the only paragraph boundary.
	runs := []textRun{
		run("line one", 50, 700),
		run("line two", 50, 686),
		run("line three", 50, 672),
		run("after the gap", 50, 630),
		run("still after", 50, 616),
	}
	got := assemblePage(runs)
	want := "line one\nline two\nline three\n\nafter the gap\nstill after"
	if got != want {
		t.Errorf("assemblePage() = %q, want %q", got, want)
	}
}

func TestAssemblePageFontSizeJumpBreaks(t *testing.T) {
	heading := run("Experience", 50, 700)
	heading.fontSize = 14
	runs := []textRun{
		heading,
		run("body text under it", 50, 686),
	}
	got := assemblePage(runs)
	if !strings.Contains(got, "Experience\n\nbody") {
		t.Errorf("font-size jump must break the paragraph, got %q", got)
	}
}

func TestAssemblePageBulletBreaks(t *testing.T) {
	runs := []textRun{
		run("• first item", 50, 700),
		run("continues here", 50, 686),
		run("• second item", 50, 672),
	}
	got := assemblePage(runs)
	want := "• first item\ncontinues here\n\n• second item"
	if got != want {
		t.Errorf("assemblePage() = %q, want %q", got, want)
	}
}

func TestAssemblePageColumnJumpBreaks(t *testing.T) {
	// Text that moves back up the page starts a new region.
	runs := []textRun{
		run("bottom of column one", 50, 100),
		run("top of column two", 300, 700),
	}
	got := assemblePage(runs)
	want := "bottom of column one\n\ntop of column two"
	if got != want {
		t.Errorf("assemblePage() = %q, want %q", got, want)
	}
}

func TestAssemblePageMergesRunsOnOneLine(t *testing.T) {
	// Two runs on the same baseline (font change mid-line): merged into
	// one line, with a space only across the visible gap.
	a := run("plain and", 50, 700)
	b := run("bold", a.right+4, 700)
	b.fontName = "Helvetica-Bold"
	touching := run("!", b.right+0.2, 700)

	got := assemblePage([]textRun{a, b, touching})
	want := "plain and bold!"
	if got != want {
		t.Errorf("assemblePage() = %q, want %q", got, want)
	}
}

func TestAssemblePageDropsIconFonts(t *testing.T) {
	icon := run("ï", 50, 700)
	icon.fontName = "FontAwesome6Brands-Regular"
	runs := []textRun{
		icon,
		run("linkedin.com/in/benydishon", 60, 700),
	}
	got := assemblePage(runs)
	want := "linkedin.com/in/benydishon"
	if got != want {
		t.Errorf("icon-font run must be dropped, got %q", got)
	}
}

func TestStripJunk(t *testing.T) {
	// C1 controls, private-use icon glyphs, and the replacement char
	// vanish; newlines, tabs, and real text survive.
	in := "phone \u0083 mail \u0080 ok\nkeep\ttabs \ue001 pua \ufffd end"
	want := "phone  mail  ok\nkeep\ttabs  pua  end"
	if got := stripJunk(in); got != want {
		t.Errorf("stripJunk() = %q, want %q", got, want)
	}
}

func TestSplitSectionsPacksLinesNotBytes(t *testing.T) {
	// One blank-line-free "paragraph" of many lines, like a PDF page
	// before layout-aware extraction or a pre-formatted text file: pieces
	// must break at line boundaries, never mid-line.
	line := "Every line here is exactly the same and short enough to pack."
	para := strings.TrimSpace(strings.Repeat(line+"\n", 120)) // ~7400 bytes

	chunks := SplitSections([]Section{{Text: para}})
	if len(chunks) < 3 {
		t.Fatalf("got %d chunks, want ≥3", len(chunks))
	}
	for _, c := range chunks {
		if len(c.Text) > chunkBudget {
			t.Errorf("chunk %d is %d bytes, budget is %d", c.Seq, len(c.Text), chunkBudget)
		}
		for i, l := range strings.Split(c.Text, "\n") {
			// A chunk may open with the tail of the previous chunk's
			// last line — that's the overlap. Everything else must be
			// a whole line.
			if i == 0 && strings.HasSuffix(line, l) {
				continue
			}
			if l != "" && l != line {
				t.Fatalf("chunk %d contains a cut line: %q", c.Seq, l)
			}
		}
	}
}
