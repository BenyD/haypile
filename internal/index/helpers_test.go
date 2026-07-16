package index

// chunksOf wraps plain texts as pageless Chunks — most store tests don't
// care about page metadata.
func chunksOf(texts ...string) []Chunk {
	out := make([]Chunk, len(texts))
	for i, t := range texts {
		out[i] = Chunk{Text: t}
	}
	return out
}
