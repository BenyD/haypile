package ingest

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// extractPptx reads the slide parts of a PowerPoint OOXML file with the
// stdlib alone — a .pptx is a zip whose ppt/slides/slideN.xml parts hold the
// text, one file per slide. Each slide becomes one section carrying its slide
// number as the page, so citations point at "slide N" (rendered as page N).
func extractPptx(path string) ([]Section, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("not a pptx (zip) file: %w", err)
	}
	defer zr.Close()

	// Collect slide parts and order them by their numeric suffix: zip and
	// lexical order would put slide10 before slide2.
	type slide struct {
		num int
		f   *zip.File
	}
	var slides []slide
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue // skips ppt/slides/_rels/*, layouts, masters, notes
		}
		numStr := strings.TrimSuffix(strings.TrimPrefix(name, "ppt/slides/slide"), ".xml")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		slides = append(slides, slide{num, f})
	}
	if len(slides) == 0 {
		return nil, errors.New("no slides found inside pptx")
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	var sections []Section
	for i, s := range slides {
		rc, err := s.f.Open()
		if err != nil {
			return nil, err
		}
		// Cap decompression so a zip-bomb pptx can't balloon memory.
		text, err := parseSlideXML(io.LimitReader(rc, maxDocumentXML))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if text != "" {
			// Page is the slide's position (1-based), even if earlier slides
			// were blank, so it stays aligned to what the deck shows.
			sections = append(sections, Section{Text: text, Page: i + 1})
		}
	}
	return sections, nil
}

// parseSlideXML streams a slide part, joining run text (a:t) and breaking a
// line at each paragraph (a:p) and soft break (a:br).
func parseSlideXML(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)

	var slide strings.Builder // paragraphs joined by newlines
	var para strings.Builder  // current paragraph text
	flushPara := func() {
		if p := strings.TrimSpace(para.String()); p != "" {
			slide.WriteString(p)
			slide.WriteByte('\n')
		}
		para.Reset()
	}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("malformed slide xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t": // a:t — run text
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return "", fmt.Errorf("malformed slide xml: %w", err)
				}
				para.WriteString(text)
			case "tab":
				para.WriteByte(' ')
			case "br":
				para.WriteByte('\n')
			}
		case xml.EndElement:
			if t.Name.Local == "p" { // a:p — paragraph
				flushPara()
			}
		}
	}
	flushPara()
	return strings.TrimSpace(slide.String()), nil
}
