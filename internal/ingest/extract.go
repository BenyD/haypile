// Package ingest turns documents on disk into indexed chunks: it walks
// folders, extracts text with provenance (page numbers, headings), and
// splits it for the index.
package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Section is a contiguous run of extracted text with its provenance. The
// chunker never merges across section boundaries — a section is the unit
// a citation points at (a PDF page, a markdown heading's scope).
type Section struct {
	Text string
	Page int // 1-based page number; 0 when the format has no pages
}

var extractors = map[string]func(path string) ([]Section, error){
	".md":       extractMarkdown,
	".markdown": extractMarkdown,
	".txt":      extractPlain,
	".docx":     extractDocx,
	".pdf":      extractPDF,
	".pptx":     extractPptx,
	".html":     extractHTML,
	".htm":      extractHTML,
}

// Supported reports whether the file at path is a format Haypile can index.
func Supported(path string) bool {
	_, ok := extractors[strings.ToLower(filepath.Ext(path))]
	return ok
}

// supportedList names the indexable extensions for error messages.
func supportedList() string {
	exts := make([]string, 0, len(extractors))
	for e := range extractors {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	return strings.Join(exts, " ")
}

// Extract pulls the text out of a document as ordered sections.
func Extract(path string) ([]Section, error) {
	fn, ok := extractors[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return nil, fmt.Errorf("unsupported format: %s", path)
	}
	secs, err := fn(path)
	if err != nil {
		return nil, fmt.Errorf("extracting %s: %w", path, err)
	}
	return secs, nil
}

func extractPlain(path string) ([]Section, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}
	return []Section{{Text: text}}, nil
}

// extractMarkdown sections a document at its headings, so chunks never
// straddle two topics and each chunk inherits its heading as context.
func extractMarkdown(path string) ([]Section, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sections []Section
	var cur strings.Builder
	inFence := false
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			sections = append(sections, Section{Text: s})
		}
		cur.Reset()
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence && isHeading(trimmed) {
			flush()
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
	}
	flush()
	return sections, nil
}

// isHeading matches ATX headings: 1-6 #s followed by a space.
func isHeading(line string) bool {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && n < len(line) && line[n] == ' '
}
