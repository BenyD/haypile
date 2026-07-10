package ingest

import "strings"

// maxChunkLen is the M0 chunk budget in bytes (~400 tokens). Structure-aware
// chunking with overlap replaces this naive splitter at M2.
const maxChunkLen = 1600

// Chunk is one indexable piece of a document.
type Chunk struct {
	Seq  int
	Text string
}

// Split divides text into chunks on paragraph boundaries, packing adjacent
// paragraphs together up to maxChunkLen. Paragraphs longer than the budget
// are hard-split at the nearest space.
func Split(text string) []Chunk {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	var chunks []Chunk
	emit := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			chunks = append(chunks, Chunk{Seq: len(chunks), Text: s})
		}
	}

	var buf strings.Builder
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if len(para) > maxChunkLen {
			emit(buf.String())
			buf.Reset()
			for len(para) > maxChunkLen {
				cut := strings.LastIndexByte(para[:maxChunkLen], ' ')
				if cut <= 0 {
					cut = maxChunkLen
				}
				emit(para[:cut])
				para = strings.TrimSpace(para[cut:])
			}
			emit(para)
			continue
		}

		if buf.Len()+len(para)+2 > maxChunkLen {
			emit(buf.String())
			buf.Reset()
		}
		buf.WriteString(para)
		buf.WriteString("\n\n")
	}
	emit(buf.String())

	return chunks
}
