// Package query runs retrieval: keyword search, vector search, and the
// hybrid merge of the two. Hybrid is the accuracy strategy — keyword covers
// exact identifiers that embeddings blur; vectors cover paraphrases that
// keywords miss.
package query

import (
	"context"
	"sort"
	"strings"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
)

// rrfK is the standard Reciprocal Rank Fusion constant: scores are summed
// as 1/(k+rank). k=60 is the literature default and deliberately not tuned
// by hand — retrieval knobs change only when the eval set says so.
const rrfK = 60

// Hybrid searches with both retrievers and fuses the rankings. With no
// embedder configured, or an index built by a different model, it returns
// keyword results alone — search must always work.
func Hybrid(ctx context.Context, st *index.Store, emb embed.Embedder, q, tag string, limit int) ([]index.Result, error) {
	keyword, err := st.Search(q, tag, limit)
	if err != nil {
		return nil, err
	}
	// keywordOnly: no semantic leg to catch paraphrases, so when strict
	// AND matching finds nothing, retry with OR — partial word overlap
	// beats an empty answer, and BM25 ranks the best overlap on top.
	keywordOnly := func() ([]index.Result, error) {
		if len(keyword) == 0 && len(strings.Fields(q)) > 1 {
			return st.SearchAny(q, tag, limit)
		}
		return keyword, nil
	}
	if emb == nil {
		return keywordOnly()
	}
	if model, err := st.EmbedModel(); err != nil || model != emb.Model() {
		if err != nil {
			return nil, err
		}
		return keywordOnly()
	}

	qvecs, err := emb.Embed(ctx, []string{q})
	if err != nil || len(qvecs) != 1 {
		// The embedder is an enhancement at query time, never a point of
		// failure: degrade to keyword results.
		return keyword, nil
	}
	vector, err := st.VectorSearch(qvecs[0], strings.Fields(q), tag, limit)
	if err != nil {
		return nil, err
	}
	// The nearest hits, saved before the floors: relevant() filters in
	// place, and these are the fallback when everything gets filtered.
	// The full limit, not a taste: this path only runs when the
	// alternative is returning nothing at all, and at that point more
	// candidates for the reader beats false precision.
	nearest := append([]index.Result(nil), vector...)

	if fused := fuse(limit, keyword, relevant(vector)); len(fused) > 0 {
		return fused, nil
	}
	// Nothing survived: question-phrased queries can score under the
	// floors while the corpus plainly holds the answer, and "no results"
	// reads as "nothing indexed". The single nearest chunk is the best
	// weak signal (OR-matched keywords rank stopword overlap, which
	// buries the real passage in noise); the caller or the answering
	// model judges it.
	if len(nearest) > 0 {
		return nearest, nil
	}
	kw, err := st.SearchAny(q, tag, limit)
	if err != nil {
		return nil, err
	}
	return kw, nil
}

// Nearest-neighbor lists always fill up to limit no matter how weakly
// related the tail is; without a floor a one-word query drags in every
// vaguely similar chunk in the index. Floors are conservative: cosine
// below minCosine is noise for this model family, and anything under
// half the best hit's similarity is padding, not signal.
const (
	minCosine     = 0.25
	relativeFloor = 0.5
)

func relevant(vector []index.Result) []index.Result {
	if len(vector) == 0 {
		return vector
	}
	top := vector[0].Score
	out := vector[:0]
	for _, r := range vector {
		if r.Score >= minCosine && r.Score >= top*relativeFloor {
			out = append(out, r)
		}
	}
	return out
}

// fuse merges ranked lists with Reciprocal Rank Fusion. RRF needs no score
// normalization, which is exactly why it's used: BM25 scores and cosine
// similarities live on unrelated scales.
func fuse(limit int, lists ...[]index.Result) []index.Result {
	type key struct {
		path string
		seq  int
	}

	fused := make(map[key]*index.Result)
	scores := make(map[key]float64)
	for _, list := range lists {
		for rank, r := range list {
			k := key{r.Path, r.Seq}
			scores[k] += 1.0 / float64(rrfK+rank+1)
			if _, seen := fused[k]; !seen {
				keep := r
				fused[k] = &keep
			}
		}
	}

	out := make([]index.Result, 0, len(fused))
	for k, r := range fused {
		r.Score = scores[k]
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Deterministic order among ties.
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Seq < out[j].Seq
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
