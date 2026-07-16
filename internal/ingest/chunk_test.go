package ingest

import (
	"strings"
	"testing"
)

func sectionsOf(texts ...string) []Section {
	secs := make([]Section, len(texts))
	for i, t := range texts {
		secs[i] = Section{Text: t}
	}
	return secs
}

func TestSplitSections(t *testing.T) {
	tests := []struct {
		name       string
		sections   []Section
		wantChunks int
	}{
		{"no sections", nil, 0},
		{"empty section", sectionsOf(""), 0},
		{"whitespace only", sectionsOf("  \n\n  \n"), 0},
		{"single paragraph", sectionsOf("Hello world."), 1},
		{"two short paragraphs pack into one chunk",
			sectionsOf("First paragraph.\n\nSecond paragraph."), 1},
		{"windows line endings", sectionsOf("First.\r\n\r\nSecond."), 1},
		{"sections never merge",
			sectionsOf("First section.", "Second section."), 2},
		{
			"paragraphs beyond budget split into multiple chunks",
			sectionsOf(strings.Repeat("This paragraph is about one hundred bytes long so that several of them overflow the budget.\n\n", 30)),
			2,
		},
		{
			"single oversized paragraph is hard-split",
			sectionsOf(strings.Repeat("word ", 1000)), // ~5000 bytes, no breaks
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitSections(tt.sections)
			if len(got) != tt.wantChunks {
				t.Fatalf("SplitSections() produced %d chunks, want %d", len(got), tt.wantChunks)
			}
			for i, c := range got {
				if c.Seq != i {
					t.Errorf("chunk %d has Seq %d", i, c.Seq)
				}
				if strings.TrimSpace(c.Text) == "" {
					t.Errorf("chunk %d is empty", i)
				}
				if len(c.Text) > chunkBudget {
					t.Errorf("chunk %d is %d bytes, budget is %d", i, len(c.Text), chunkBudget)
				}
			}
		})
	}
}

func TestSplitSectionsPageMetadata(t *testing.T) {
	long := strings.Repeat("Text that fills the page with content beyond one chunk budget. ", 45) // ~2900 bytes
	chunks := SplitSections([]Section{
		{Text: "Page one is short.", Page: 1},
		{Text: long, Page: 2},
	})

	if len(chunks) < 3 {
		t.Fatalf("got %d chunks, want ≥3 (page 2 must split)", len(chunks))
	}
	if chunks[0].Page != 1 {
		t.Errorf("chunk 0 has page %d, want 1", chunks[0].Page)
	}
	for _, c := range chunks[1:] {
		if c.Page != 2 {
			t.Errorf("chunk %d has page %d, want 2 — every piece of a split page keeps its page", c.Seq, c.Page)
		}
	}
}

func TestSplitSectionsOverlap(t *testing.T) {
	// Two chunks from one section must share text: a fact straddling the
	// boundary has to be findable in both.
	para := "The quarterly numbers were reviewed by the board in the autumn meeting. "
	chunks := SplitSections(sectionsOf(strings.Repeat(para, 40))) // ~2900 bytes

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	tail := chunks[0].Text[len(chunks[0].Text)-100:]
	if !strings.Contains(chunks[1].Text, tail[strings.Index(tail, " ")+1:]) {
		t.Error("second chunk does not contain the tail of the first (no overlap)")
	}
}

func TestSplitSectionsPreservesContent(t *testing.T) {
	text := "The termination clause requires sixty days notice.\n\nPayment is due net-45."
	chunks := SplitSections(sectionsOf(text))

	joined := ""
	for _, c := range chunks {
		joined += c.Text + " "
	}
	for _, phrase := range []string{"termination clause", "sixty days", "net-45"} {
		if !strings.Contains(joined, phrase) {
			t.Errorf("phrase %q lost during chunking", phrase)
		}
	}
}

func TestHardSplitOverlap(t *testing.T) {
	pieces := hardSplit(strings.Repeat("alpha beta gamma delta ", 200)) // ~4600 bytes
	if len(pieces) < 2 {
		t.Fatalf("got %d pieces, want ≥2", len(pieces))
	}
	for i := 1; i < len(pieces); i++ {
		prevTail := overlapTail(pieces[i-1])
		if prevTail == "" || !strings.HasPrefix(pieces[i], prevTail) {
			t.Errorf("piece %d does not start with the previous piece's tail", i)
		}
	}
}
