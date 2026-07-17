package ingest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// extractHTML pulls readable text from an HTML file, sectioning at headings
// (h1–h6) the same way markdown and docx do so a chunk never straddles two
// topics and leads with its heading. script, style, and head content is
// dropped. HTML has no pages, so sections carry Page 0.
func extractHTML(path string) ([]Section, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	z := html.NewTokenizer(f)

	var sections []Section
	var section strings.Builder // text blocks joined by blank lines
	var para strings.Builder    // current text block, whitespace collapsed on flush
	skip := 0                   // >0 while inside a dropped element (script/style/head)

	flushSection := func() {
		if s := strings.TrimSpace(section.String()); s != "" {
			sections = append(sections, Section{Text: s})
		}
		section.Reset()
	}
	// strings.Fields collapses every run of whitespace, including the newlines
	// HTML sprinkles between inline tags, into single spaces.
	flushPara := func() {
		if p := strings.Join(strings.Fields(para.String()), " "); p != "" {
			section.WriteString(p)
			section.WriteString("\n\n")
		}
		para.Reset()
	}

	for {
		switch z.Next() {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				flushPara()
				flushSection()
				return sections, nil
			}
			return nil, fmt.Errorf("malformed html: %w", z.Err())

		case html.TextToken:
			if skip == 0 {
				para.Write(z.Text())
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			if htmlDrop[a] {
				skip++
				continue
			}
			if isHTMLHeading(a) {
				// The heading leads a fresh section: close the running
				// paragraph and section, then let the heading text collect.
				flushPara()
				flushSection()
			} else if htmlBlock[a] {
				flushPara()
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			if htmlDrop[a] {
				if skip > 0 {
					skip--
				}
				continue
			}
			if isHTMLHeading(a) || htmlBlock[a] {
				flushPara()
			}
		}
	}
}

// htmlDrop names elements whose text is never content. The tokenizer emits
// script and style bodies as raw text tokens, so they must be skipped
// explicitly rather than parsed.
var htmlDrop = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Head:     true,
	atom.Noscript: true,
	atom.Template: true,
}

// htmlBlock names block-level elements whose boundaries separate text blocks,
// so words on either side never run together into one token.
var htmlBlock = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Br: true, atom.Hr: true,
	atom.Li: true, atom.Ul: true, atom.Ol: true, atom.Dl: true,
	atom.Dt: true, atom.Dd: true, atom.Blockquote: true, atom.Pre: true,
	atom.Section: true, atom.Article: true, atom.Header: true,
	atom.Footer: true, atom.Main: true, atom.Aside: true, atom.Nav: true,
	atom.Figure: true, atom.Figcaption: true, atom.Address: true,
	atom.Table: true, atom.Tr: true, atom.Td: true, atom.Th: true,
	atom.Caption: true,
}

func isHTMLHeading(a atom.Atom) bool {
	switch a {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}
