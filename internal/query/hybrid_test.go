package query

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
)

// fakeEmbedder maps exact texts to fixed vectors — deterministic semantics
// for tests without a model.
type fakeEmbedder struct {
	vectors map[string][]float32
}

func (f *fakeEmbedder) Model() string { return "fake/test" }

func (f *fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, ok := f.vectors[t]
		if !ok {
			v = []float32{0, 0}
		}
		out[i] = v
	}
	return out, nil
}

var _ embed.Embedder = (*fakeEmbedder)(nil)

func setup(t *testing.T) (*index.Store, *fakeEmbedder) {
	t.Helper()
	st, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/contract.md", "s1", 1, 1,
		chunksOf("Either party may terminate this agreement with sixty days notice."))
	st.UpsertFile(srcID, "/docs/kitchen.md", "s2", 1, 1,
		chunksOf("Going with white oak cabinets for the renovation."))

	fake := &fakeEmbedder{vectors: map[string][]float32{
		// The semantic case: query shares no words with the contract chunk
		// but points in the same direction.
		"agreement cancellation": {1, 0},
	}}

	if err := st.SetEmbedModel(fake.Model()); err != nil {
		t.Fatal(err)
	}
	missing, _ := st.MissingEmbeddings(srcID)
	for _, m := range missing {
		vec := []float32{0, 1} // kitchen direction
		if m.Text[0] == 'E' {  // contract chunk
			vec = []float32{0.95, 0.31}
		}
		st.PutEmbedding(m.ID, "sha:"+m.Text, fake.Model(), vec)
	}
	return st, fake
}

func TestHybridFindsParaphrase(t *testing.T) {
	st, fake := setup(t)

	// Keyword-only misses: no shared words.
	kw, _ := st.Search("agreement cancellation", "", 10)
	if len(kw) != 0 {
		t.Fatalf("precondition broken: keyword search found %d results", len(kw))
	}

	// Hybrid finds the termination clause via the vector leg.
	results, err := Hybrid(context.Background(), st, fake, "agreement cancellation", "", 10)
	if err != nil {
		t.Fatalf("Hybrid: %v", err)
	}
	if len(results) == 0 || results[0].Path != "/docs/contract.md" {
		t.Fatalf("paraphrase not found: %+v", results)
	}
}

func TestHybridWithoutEmbedderIsKeyword(t *testing.T) {
	st, _ := setup(t)

	results, err := Hybrid(context.Background(), st, nil, "oak cabinets", "", 10)
	if err != nil || len(results) != 1 || results[0].Path != "/docs/kitchen.md" {
		t.Fatalf("keyword fallback broken: %+v err=%v", results, err)
	}
}

func TestHybridModelMismatchDegradesToKeyword(t *testing.T) {
	st, _ := setup(t)

	// Same vectors, different declared model → the vector leg must not run.
	other := &fakeEmbedder{vectors: map[string][]float32{"agreement cancellation": {1, 0}}}

	results, err := Hybrid(context.Background(), st, &mismatchedEmbedder{other}, "agreement cancellation", "", 10)
	if err != nil {
		t.Fatalf("Hybrid: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("mismatched model must degrade to keyword-only (no hits here), got %+v", results)
	}
}

type mismatchedEmbedder struct{ *fakeEmbedder }

func (m *mismatchedEmbedder) Model() string { return "fake/other" }

func TestFuseRewardsAgreement(t *testing.T) {
	a := index.Result{Path: "/a", Seq: 0}
	b := index.Result{Path: "/b", Seq: 0}
	c := index.Result{Path: "/c", Seq: 0}

	// b appears in BOTH lists (rank 2 each); a and c each top only one
	// list. Appearing in both retrievers must beat appearing in one —
	// that's the agreement property RRF provides.
	fusedList := fuse(3,
		[]index.Result{a, b},
		[]index.Result{c, b},
	)
	if fusedList[0].Path != "/b" {
		got := []string{fusedList[0].Path, fusedList[1].Path, fusedList[2].Path}
		t.Fatalf("agreement should rank first, got order %v", got)
	}
}

// chunksOf wraps plain texts as pageless index.Chunks.
func chunksOf(texts ...string) []index.Chunk {
	out := make([]index.Chunk, len(texts))
	for i, t := range texts {
		out[i] = index.Chunk{Text: t}
	}
	return out
}
