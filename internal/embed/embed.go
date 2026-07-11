// Package embed turns text into vectors for semantic search. Two backends
// implement Embedder: an OpenAI-compatible endpoint (this file's siblings)
// and the bundled in-binary model (landing later in M1). Vectors from
// different models are never comparable — the index records which model
// built it.
package embed

import (
	"context"
	"math"
)

// Embedder produces one vector per input text. Implementations must return
// L2-normalized vectors so similarity is a plain dot product.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Model identifies the producing model, e.g. "endpoint/nomic-embed-text".
	Model() string
}

// normalize scales v to unit length in place. Zero vectors are left as-is.
func normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	inv := 1 / math.Sqrt(sum)
	for i := range v {
		v[i] = float32(float64(v[i]) * inv)
	}
}
