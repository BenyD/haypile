package query

import (
	"context"
	"testing"
)

// A search box wants honesty: a query with no real match returns
// nothing, not zero-relevance chunks with blank snippets (the answer
// path's fallback leaking in). HybridForAnswer keeps that fallback so
// the model always has something to read.
func TestHybridStrictReturnsEmptyForAbsentTerm(t *testing.T) {
	st, fake := setup(t)
	ctx := context.Background()

	// "undici" is nowhere in the corpus and has no fake embedding, so
	// both legs come up empty.
	strict, err := Hybrid(ctx, st, fake, "undici", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict) != 0 {
		t.Errorf("strict search for an absent term must return nothing, got %d: %+v", len(strict), strict)
	}

	// A present word still matches under the strict path.
	hits, err := Hybrid(ctx, st, fake, "terminate", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Error("strict search for a present term must return a hit")
	}
}
