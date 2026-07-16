package ingest

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxDocumentXML bounds how much document.xml we will decompress: 512MB
// of XML is far beyond any real document and cheap insurance against a
// crafted zip bomb.
const maxDocumentXML = 512 << 20

// extractDocx reads the main document part of an OOXML file with the
// stdlib alone — a .docx is a zip whose word/document.xml holds the text.
// Word has no static page numbers (pagination happens at render time), so
// docx sections carry Page 0; headings provide the structure instead.
func extractDocx(path string) ([]Section, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("not a docx (zip) file: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			// Cap decompression so a zip-bomb docx can't balloon memory;
			// no legitimate document part comes close to this.
			return parseDocumentXML(io.LimitReader(rc, maxDocumentXML))
		}
	}
	return nil, errors.New("no word/document.xml inside docx")
}

// parseDocumentXML streams the XML, collecting paragraph text and starting
// a new section at every Heading-styled paragraph.
func parseDocumentXML(r io.Reader) ([]Section, error) {
	dec := xml.NewDecoder(r)

	var sections []Section
	var section strings.Builder // paragraphs joined by blank lines
	var para strings.Builder    // current paragraph text
	flushSection := func() {
		if s := strings.TrimSpace(section.String()); s != "" {
			sections = append(sections, Section{Text: s})
		}
		section.Reset()
	}
	flushPara := func() {
		if p := strings.TrimSpace(para.String()); p != "" {
			section.WriteString(p)
			section.WriteString("\n\n")
		}
		para.Reset()
	}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("malformed document.xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pStyle":
				// A heading style starts a new section (the current
				// paragraph is the heading itself, so flush before it).
				for _, a := range t.Attr {
					if a.Name.Local == "val" && isHeadingStyle(a.Value) {
						flushSection()
					}
				}
			case "t": // run text
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return nil, fmt.Errorf("malformed document.xml: %w", err)
				}
				para.WriteString(text)
			case "tab":
				para.WriteByte(' ')
			case "br", "cr":
				para.WriteByte('\n')
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				flushPara()
			}
		}
	}
	flushPara()
	flushSection()
	return sections, nil
}

func isHeadingStyle(val string) bool {
	v := strings.ToLower(val)
	return strings.HasPrefix(v, "heading") || v == "title"
}
