package ingest

import "strings"

// Chunk budgets in bytes. ~4 bytes per token for English text puts 2000
// bytes at the roadmap's ~500-token target with ~12% overlap; both are knobs
// the eval set owns.
const (
	chunkBudget  = 2000
	chunkOverlap = 250
)

// Chunk is one indexable piece of a document.
type Chunk struct {
	Seq  int
	Page int // 1-based page number; 0 when the format has no pages
	Text string
}

// SplitSections chunks extracted sections for indexing. Structure decides
// the boundaries: chunks never cross a section (page, heading scope), and
// within a section paragraphs pack together up to the budget. Consecutive
// chunks overlap so a fact straddling a boundary is findable in both.
func SplitSections(sections []Section) []Chunk {
	var chunks []Chunk
	emit := func(text string, page int) {
		text = strings.TrimSpace(text)
		if text != "" {
			chunks = append(chunks, Chunk{Seq: len(chunks), Page: page, Text: text})
		}
	}

	for _, sec := range sections {
		text := strings.ReplaceAll(sec.Text, "\r\n", "\n")

		var buf strings.Builder
		pack := func(para string) {
			if buf.Len()+len(para)+2 > chunkBudget {
				tail := overlapTail(buf.String())
				emit(buf.String(), sec.Page)
				buf.Reset()
				// The tail is context, not content: drop it rather than
				// let it push the next chunk past the budget.
				if tail != "" && len(tail)+len(para)+2 <= chunkBudget {
					buf.WriteString(tail)
					buf.WriteString("\n\n")
				}
			}
			buf.WriteString(para)
			buf.WriteString("\n\n")
		}

		for _, para := range strings.Split(text, "\n\n") {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			if len(para) <= chunkBudget {
				pack(para)
				continue
			}
			// An oversized paragraph still has structure: break it at
			// line boundaries and pack those. Only a single line bigger
			// than the whole budget gets cut mid-text.
			for _, piece := range packLines(para) {
				pack(piece)
			}
		}
		emit(buf.String(), sec.Page)
	}
	return chunks
}

// packLines splits an oversized paragraph into budget-sized pieces of
// whole lines, so PDF pages and other line-broken text never get cut
// mid-sentence when a byte boundary would do it.
func packLines(para string) []string {
	var pieces []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			pieces = append(pieces, s)
		}
		b.Reset()
	}
	for _, line := range strings.Split(para, "\n") {
		if len(line) > chunkBudget {
			flush()
			pieces = append(pieces, hardSplit(line)...)
			continue
		}
		if b.Len()+len(line)+1 > chunkBudget {
			flush()
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()
	return pieces
}

// hardSplit cuts an oversized paragraph at word boundaries, each piece
// starting with the tail of the previous one as overlap. Text without
// spaces (CJK prose) cuts at the budget, on a rune boundary.
func hardSplit(para string) []string {
	var pieces []string
	for len(para) > chunkBudget {
		cut := strings.LastIndexByte(para[:chunkBudget], ' ')
		if cut <= 0 {
			cut = runeStart(para, chunkBudget)
		}
		pieces = append(pieces, para[:cut])
		rest := strings.TrimSpace(para[cut:])
		if tail := overlapTail(para[:cut]); tail != "" {
			rest = tail + " " + rest
		}
		para = rest
	}
	if strings.TrimSpace(para) != "" {
		pieces = append(pieces, para)
	}
	return pieces
}

// overlapTail returns the last ~chunkOverlap bytes of s, cut at a word
// boundary (or a rune boundary when there are no spaces). It is what
// consecutive chunks share.
func overlapTail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= chunkOverlap {
		return ""
	}
	tail := s[runeStart(s, len(s)-chunkOverlap):]
	if sp := strings.IndexByte(tail, ' '); sp >= 0 {
		tail = tail[sp+1:]
	}
	return strings.TrimSpace(tail)
}

// runeStart backs i up to the start of the UTF-8 rune it points into, so
// slicing at i never splits a character.
func runeStart(s string, i int) int {
	for i > 0 && s[i]&0xC0 == 0x80 {
		i--
	}
	return i
}
