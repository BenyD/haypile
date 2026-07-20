package ingest

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// PDF text arrives as positioned runs, not paragraphs: PDFium's rect mode
// yields one run per same-font stretch of a baseline, with coordinates in
// PDF points (origin bottom-left, y up). This file rebuilds the structure
// the page only encodes visually — lines from runs, paragraphs from
// vertical gaps and font changes — so the chunker gets real boundaries
// instead of a wall of text.

// textRun is one positioned run of same-font text.
type textRun struct {
	text     string
	left     float64
	right    float64
	top      float64
	bottom   float64
	fontSize float64
	fontName string
}

// textLine is a visual line: the runs that share a baseline.
type textLine struct {
	runs []textRun
	top  float64
	bot  float64
}

func (l *textLine) height() float64 { return l.top - l.bot }

// fontSize of a line is its dominant (largest) run size — the size a
// heading would be judged by even with an inline smaller run.
func (l *textLine) fontSize() float64 {
	var s float64
	for _, r := range l.runs {
		if r.fontSize > s {
			s = r.fontSize
		}
	}
	return s
}

func (l *textLine) text() string {
	var b strings.Builder
	prevRight := 0.0
	for i, r := range l.runs {
		if i > 0 {
			// A visible horizontal gap between runs is a space the rects
			// don't carry; touching runs (font change mid-word) are not.
			if r.left-prevRight > 1 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(r.text)
		prevRight = r.right
	}
	return b.String()
}

// assemblePage rebuilds a page's text with paragraph breaks. Runs must be
// in PDFium's char order, which follows the content stream — reading
// order for virtually all generated PDFs, and the only order that keeps
// multi-column layouts coherent.
func assemblePage(runs []textRun) string {
	lines := groupLines(runs)
	if len(lines) == 0 {
		return ""
	}

	medH := medianLineHeight(lines)

	var b strings.Builder
	for i, ln := range lines {
		if i > 0 {
			if paragraphBreak(lines[i-1], ln, medH) {
				b.WriteString("\n\n")
			} else {
				b.WriteByte('\n')
			}
		}
		b.WriteString(ln.text())
	}
	return b.String()
}

// groupLines merges consecutive runs that overlap vertically into visual
// lines, dropping icon-font runs and empty runs on the way. Only
// consecutive runs merge: grouping across the whole page by Y would
// interleave columns.
func groupLines(runs []textRun) []*textLine {
	var lines []*textLine
	var cur *textLine
	for _, r := range runs {
		if strings.TrimSpace(r.text) == "" || iconFont(r.fontName) {
			continue
		}
		if cur != nil && overlapsVertically(cur, r) {
			cur.runs = append(cur.runs, r)
			if r.top > cur.top {
				cur.top = r.top
			}
			if r.bottom < cur.bot {
				cur.bot = r.bottom
			}
			continue
		}
		cur = &textLine{runs: []textRun{r}, top: r.top, bot: r.bottom}
		lines = append(lines, cur)
	}
	for _, l := range lines {
		sortRunsByLeft(l.runs)
	}
	return lines
}

// overlapsVertically reports whether run r sits on line l's baseline: the
// vertical overlap covers at least half of the shorter of the two.
func overlapsVertically(l *textLine, r textRun) bool {
	overlap := min(l.top, r.top) - max(l.bot, r.bottom)
	shorter := min(l.height(), r.top-r.bottom)
	return overlap > shorter/2
}

// paragraphBreak decides whether line b starts a new paragraph after a.
// The signals are the ones a human reads: extra vertical whitespace, a
// font-size change (headings), a bullet marker, or a jump back up the
// page (new column or region).
func paragraphBreak(a, b *textLine, medH float64) bool {
	gap := a.bot - b.top
	if gap < -medH/2 {
		return true // b is above a: new column or text region
	}
	if gap > 0.8*medH {
		return true
	}
	if fs, prev := b.fontSize(), a.fontSize(); prev > 0 && fs > 0 {
		if ratio := fs / prev; ratio > 1.15 || ratio < 0.87 {
			return true
		}
	}
	return startsWithBullet(b.text())
}

// startsWithBullet matches the markers PDFs actually use for list items.
func startsWithBullet(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	switch r {
	case '•', '◦', '▪', '‣', '·', '○', '➢', '➤', '❖':
		return true
	case '-', '–', '—', '*':
		next, _ := utf8.DecodeRuneInString(s[size:])
		return next == ' '
	}
	return false
}

func medianLineHeight(lines []*textLine) float64 {
	hs := make([]float64, 0, len(lines))
	for _, l := range lines {
		if h := l.height(); h > 0 {
			hs = append(hs, h)
		}
	}
	if len(hs) == 0 {
		return 1
	}
	sortFloats(hs)
	return hs[len(hs)/2]
}

// iconFont reports whether name is a symbol font whose glyphs render as
// icons (FontAwesome's phone, LinkedIn, …). Their codepoints reach the
// index as junk characters, so their runs are dropped whole.
func iconFont(name string) bool {
	n := strings.ToLower(name)
	for _, marker := range []string{
		"fontawesome", "font awesome", "glyphicon", "ionicon",
		"materialicons", "material icons", "material symbols",
		"wingdings", "webdings", "dingbat", "icomoon", "octicons",
		"bootstrap-icons", "bootstrap icons", "remixicon", "lucide",
		"heroicons", "feather", "tabler-icons", "nerd font", "nerdfont",
	} {
		if strings.Contains(n, marker) {
			return true
		}
	}
	return false
}

func sortRunsByLeft(runs []textRun) {
	for i := 1; i < len(runs); i++ {
		for j := i; j > 0 && runs[j].left < runs[j-1].left; j-- {
			runs[j], runs[j-1] = runs[j-1], runs[j]
		}
	}
}

func sortFloats(fs []float64) {
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fs[j] < fs[j-1]; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// stripJunk removes codepoints that carry no searchable content: C0/C1
// controls (except newline and tab), private-use glyphs left by icon and
// symbol fonts, and the replacement character.
func stripJunk(s string) string {
	clean := func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20, r >= 0x7F && r < 0xA0:
			return -1
		case unicode.Is(unicode.Co, r), r == unicode.ReplacementChar:
			return -1
		}
		return r
	}
	return strings.Map(clean, s)
}
