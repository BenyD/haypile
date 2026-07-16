// Package bundled is the in-binary embedding backend: a pure-Go
// implementation of all-MiniLM-L6-v2 (WordPiece tokenizer + 6-layer BERT
// encoder + mean pooling). It exists so semantic search works with zero
// external dependencies — no model download, no runtime, no network.
package bundled

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Special token ids fixed by the BERT-uncased vocab shipped with the model.
const (
	maxSeqLen = 256 // sentence-transformers config for all-MiniLM-L6-v2
	// maxWordChars mirrors HF WordPiece: longer words become [UNK] outright.
	maxWordChars = 100
)

// tokenizer is a BERT-uncased WordPiece tokenizer. It reproduces the
// Hugging Face BertTokenizer pipeline: clean → CJK spacing → lowercase +
// accent strip → punctuation split → greedy longest-match WordPiece.
type tokenizer struct {
	ids map[string]int32
	cls int32
	sep int32
	unk int32
	pad int32
}

func newTokenizer(vocab io.Reader) (*tokenizer, error) {
	t := &tokenizer{ids: make(map[string]int32, 31000)}
	sc := bufio.NewScanner(vocab)
	var id int32
	for sc.Scan() {
		// Tokens are one per line; ids are line numbers. Trailing space is
		// significant in no BERT vocab entry, but trailing \r would be.
		t.ids[strings.TrimRight(sc.Text(), "\r")] = id
		id++
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading vocab: %w", err)
	}

	var ok bool
	for name, dst := range map[string]*int32{
		"[CLS]": &t.cls, "[SEP]": &t.sep, "[UNK]": &t.unk, "[PAD]": &t.pad,
	} {
		if *dst, ok = t.ids[name]; !ok {
			return nil, fmt.Errorf("vocab is missing %s", name)
		}
	}
	return t, nil
}

// encode converts text to model input ids: [CLS] wordpieces… [SEP],
// truncated to maxSeqLen total.
func (t *tokenizer) encode(text string) []int32 {
	out := make([]int32, 0, 64)
	out = append(out, t.cls)
	for _, word := range basicTokenize(text) {
		for _, piece := range t.wordpiece(word) {
			if len(out) == maxSeqLen-1 { // reserve room for [SEP]
				return append(out, t.sep)
			}
			out = append(out, piece)
		}
	}
	return append(out, t.sep)
}

// wordpiece splits one basic token into vocabulary pieces by greedy longest
// match; continuations carry the "##" prefix. Unmatchable words are [UNK].
func (t *tokenizer) wordpiece(word string) []int32 {
	runes := []rune(word)
	if len(runes) > maxWordChars {
		return []int32{t.unk}
	}
	var pieces []int32
	for start := 0; start < len(runes); {
		end := len(runes)
		var id int32
		found := false
		for ; end > start; end-- {
			s := string(runes[start:end])
			if start > 0 {
				s = "##" + s
			}
			if v, ok := t.ids[s]; ok {
				id, found = v, true
				break
			}
		}
		if !found {
			return []int32{t.unk}
		}
		pieces = append(pieces, id)
		start = end
	}
	return pieces
}

// basicTokenize performs BERT's pre-WordPiece pass: control-char cleanup,
// CJK isolation, lowercasing with accent stripping, punctuation splitting.
func basicTokenize(text string) []string {
	var cleaned strings.Builder
	cleaned.Grow(len(text))
	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD || isControl(r):
			// drop
		case isWhitespace(r):
			cleaned.WriteByte(' ')
		case isCJK(r):
			cleaned.WriteByte(' ')
			cleaned.WriteRune(r)
			cleaned.WriteByte(' ')
		default:
			cleaned.WriteRune(r)
		}
	}

	var tokens []string
	for _, word := range strings.Fields(cleaned.String()) {
		word = stripAccents(strings.ToLower(word))
		tokens = append(tokens, splitPunct(word)...)
	}
	return tokens
}

// stripAccents removes combining marks after NFD decomposition, matching
// BERT-uncased's strip_accents behavior ("café" → "cafe").
func stripAccents(s string) string {
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitPunct breaks a word so each punctuation rune is its own token:
// "it's." → ["it", "'", "s", "."].
func splitPunct(word string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range word {
		if isPunct(r) {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			out = append(out, string(r))
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// isPunct matches HF's definition: all non-alphanumeric ASCII (including
// $, +, <, =, >, ^, `, |, ~ which Unicode classes as symbols) plus every
// Unicode P* category rune.
func isPunct(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) ||
		(r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r)
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
		unicode.Is(unicode.Zs, r)
}

func isControl(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false
	}
	return unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r)
}

// isCJK reports whether r is a CJK ideograph (the ranges BERT isolates so
// each ideograph becomes its own token).
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F)
}
