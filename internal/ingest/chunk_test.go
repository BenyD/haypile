package ingest

import (
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantChunks int
	}{
		{"empty", "", 0},
		{"whitespace only", "  \n\n  \n", 0},
		{"single paragraph", "Hello world.", 1},
		{"two short paragraphs pack into one chunk", "First paragraph.\n\nSecond paragraph.", 1},
		{"windows line endings", "First.\r\n\r\nSecond.", 1},
		{
			"paragraphs beyond budget split into multiple chunks",
			strings.Repeat("This paragraph is about one hundred bytes long so that several of them overflow the budget.\n\n", 30),
			2,
		},
		{
			"single oversized paragraph is hard-split",
			strings.Repeat("word ", 800), // ~4000 bytes, no paragraph breaks
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.text)
			if len(got) != tt.wantChunks {
				t.Fatalf("Split() produced %d chunks, want %d", len(got), tt.wantChunks)
			}
			for i, c := range got {
				if c.Seq != i {
					t.Errorf("chunk %d has Seq %d", i, c.Seq)
				}
				if strings.TrimSpace(c.Text) == "" {
					t.Errorf("chunk %d is empty", i)
				}
				if len(c.Text) > maxChunkLen {
					t.Errorf("chunk %d is %d bytes, budget is %d", i, len(c.Text), maxChunkLen)
				}
			}
		})
	}
}

func TestSplitPreservesContent(t *testing.T) {
	text := "The termination clause requires sixty days notice.\n\nPayment is due net-45."
	chunks := Split(text)

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
