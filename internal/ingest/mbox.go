package ingest

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// extractMbox pulls every message out of an mbox mailbox. One message is
// one section, leading with its Subject/From/Date so a chunk reads like
// the email it came from; Page carries the 1-based message ordinal, which
// is what a citation into a mailbox has to point at. A message that fails
// to parse indexes as its raw text rather than vanishing: unsearchable
// mail defeats the point of indexing a mailbox.
func extractMbox(path string) ([]Section, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sections []Section
	ordinal := 0
	var raw bytes.Buffer

	flush := func() {
		if raw.Len() == 0 {
			return
		}
		ordinal++
		text := messageText(raw.Bytes())
		if text != "" {
			sections = append(sections, Section{Text: text, Page: ordinal})
		}
		raw.Reset()
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20) // single lines can be huge
	first := true
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "From ") {
			if first && raw.Len() == 0 {
				first = false
				continue // the separator itself is not message content
			}
			flush()
			continue
		}
		if first {
			// A file whose first line is not an mbox separator is not an
			// mbox; erroring here counts it as Failed instead of indexing
			// noise under a lying extension.
			return nil, errors.New("not an mbox: missing leading From separator")
		}
		// mboxrd escaping: body lines that would look like a separator
		// gain a leading ">" on write; drop exactly one on read.
		if strings.HasPrefix(line, ">") && strings.HasPrefix(strings.TrimLeft(line, ">"), "From ") {
			line = line[1:]
		}
		raw.WriteString(line)
		raw.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading mbox: %w", err)
	}
	flush()
	return sections, nil
}

// messageText renders one raw RFC 5322 message as indexable text:
// a Subject/From/Date header line, then the flattened body. Parse
// failures fall back to the raw bytes; mail that cannot be parsed can
// still be found.
func messageText(raw []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return strings.TrimSpace(string(raw))
	}

	dec := &mime.WordDecoder{}
	decode := func(s string) string {
		if d, err := dec.DecodeHeader(s); err == nil {
			return d
		}
		return s
	}

	var b strings.Builder
	if s := decode(msg.Header.Get("Subject")); s != "" {
		b.WriteString("Subject: " + s + "\n")
	}
	if f := decode(msg.Header.Get("From")); f != "" {
		b.WriteString("From: " + f + "\n")
	}
	if d := msg.Header.Get("Date"); d != "" {
		b.WriteString("Date: " + d + "\n")
	}
	b.WriteString("\n")

	body := partText(msg.Header.Get("Content-Type"),
		msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
	b.WriteString(strings.TrimSpace(body))
	return strings.TrimSpace(b.String())
}

// partText flattens one MIME part (possibly multipart) to plain text.
// text/plain wins; text/html is stripped to its text when no plain part
// exists; attachments and other media contribute nothing.
func partText(contentType, encoding string, r io.Reader) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "text/plain" // unlabeled parts are almost always plain text
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return ""
		}
		mr := multipart.NewReader(r, boundary)
		var plain, htmlText []string
		for {
			p, err := mr.NextPart()
			if err != nil {
				break // io.EOF or malformed remainder: keep what we have
			}
			if strings.HasPrefix(p.Header.Get("Content-Disposition"), "attachment") {
				continue
			}
			t := partText(p.Header.Get("Content-Type"),
				p.Header.Get("Content-Transfer-Encoding"), p)
			if t == "" {
				continue
			}
			pt, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if pt == "text/html" {
				htmlText = append(htmlText, t)
			} else {
				plain = append(plain, t)
			}
		}
		if len(plain) > 0 {
			return strings.Join(plain, "\n\n")
		}
		return strings.Join(htmlText, "\n\n")
	}

	switch strings.ToLower(encoding) {
	case "quoted-printable":
		r = quotedprintable.NewReader(r)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, r)
	}

	switch mediaType {
	case "text/plain":
		data, _ := io.ReadAll(r)
		return strings.TrimSpace(string(data))
	case "text/html":
		return strings.TrimSpace(strippedHTML(r))
	default:
		return "" // images, PDFs-as-attachments, and other media
	}
}

// strippedHTML reduces an HTML body to its visible text, whitespace
// collapsed, script and style dropped.
func strippedHTML(r io.Reader) string {
	z := html.NewTokenizer(r)
	var out []string
	skip := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.Join(out, " ")
		case html.StartTagToken:
			name, _ := z.TagName()
			if n := string(name); n == "script" || n == "style" {
				skip++
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if n := string(name); (n == "script" || n == "style") && skip > 0 {
				skip--
			}
		case html.TextToken:
			if skip == 0 {
				if t := strings.Join(strings.Fields(string(z.Text())), " "); t != "" {
					out = append(out, t)
				}
			}
		}
	}
}
