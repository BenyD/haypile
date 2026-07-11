// Package query runs retrieval: keyword search, vector search, and the
// hybrid merge of the two. Hybrid is the accuracy strategy — keyword covers
// exact identifiers that embeddings blur; vectors cover paraphrases that
// keywords miss.
package query

import (
	"context"
	"sort"

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
	if emb == nil {
		return keyword, nil
	}
	if model, err := st.EmbedModel(); err != nil || model != emb.Model() {
		return keyword, err
	}

	qvecs, err := emb.Embed(ctx, []string{q})
	if err != nil || len(qvecs) != 1 {
		// The embedder is an enhancement at query time, never a point of
		// failure: degrade to keyword results.
		return keyword, nil
	}
	vector, err := st.VectorSearch(qvecs[0], tag, limit)
	if err != nil {
		return nil, err
	}

	return fuse(limit, keyword, vector), nil
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
