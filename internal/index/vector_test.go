package index

import (
	"strings"
	"testing"
)

func TestEmbedModelIsSticky(t *testing.T) {
	st := openTestStore(t)

	if m, _ := st.EmbedModel(); m != "" {
		t.Fatalf("fresh index has model %q", m)
	}
	if err := st.SetEmbedModel("model-a"); err != nil {
		t.Fatalf("first SetEmbedModel: %v", err)
	}
	if err := st.SetEmbedModel("model-a"); err != nil {
		t.Fatalf("same model must be idempotent: %v", err)
	}
	err := st.SetEmbedModel("model-b")
	if err == nil || !strings.Contains(err.Error(), "re-index") {
		t.Fatalf("switching models must error with guidance, got: %v", err)
	}
}

func TestVectorRoundTripAndSearch(t *testing.T) {
	st := openTestStore(t)

	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "s1", 1, 1, chunksOf("contract termination text", "payment text"))
	st.UpsertFile(srcID, "/docs/b.md", "s2", 1, 1, chunksOf("kitchen renovation text"))

	missing, err := st.MissingEmbeddings(srcID)
	if err != nil || len(missing) != 3 {
		t.Fatalf("MissingEmbeddings = %d, %v; want 3", len(missing), err)
	}

	// Hand-placed unit vectors: termination near x-axis, kitchen near y-axis.
	vecs := [][]float32{{1, 0}, {0.9, 0.44}, {0, 1}}
	for i, m := range missing {
		if err := st.PutEmbedding(m.ID, "sha"+m.Text, "test-model", vecs[i]); err != nil {
			t.Fatalf("PutEmbedding: %v", err)
		}
	}
	if left, _ := st.MissingEmbeddings(srcID); len(left) != 0 {
		t.Fatalf("still %d missing after PutEmbedding", len(left))
	}

	results, err := st.VectorSearch([]float32{1, 0}, "", 2)
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	if len(results) != 2 || results[0].Path != "/docs/a.md" || results[0].Seq != 0 {
		t.Fatalf("wrong nearest neighbor: %+v", results)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("results not sorted by similarity: %+v", results)
	}
}

func TestCacheSurvivesChunkDeletion(t *testing.T) {
	st := openTestStore(t)

	srcID, _ := st.AddSource("/docs", "")
	st.UpsertFile(srcID, "/docs/a.md", "v1", 1, 1, chunksOf("stable paragraph"))
	missing, _ := st.MissingEmbeddings(srcID)
	st.PutEmbedding(missing[0].ID, "sha-stable", "m", []float32{1, 0})

	// Re-index the file (new chunk ids). The chunk vector is gone but the
	// content cache must survive — same text is never embedded twice.
	st.UpsertFile(srcID, "/docs/a.md", "v2", 1, 2, chunksOf("stable paragraph", "new paragraph"))

	if missing, _ = st.MissingEmbeddings(srcID); len(missing) != 2 {
		t.Fatalf("expected 2 missing after re-index, got %d", len(missing))
	}
	vec, err := st.CachedVector("sha-stable", "m")
	if err != nil || vec == nil {
		t.Fatalf("cache lost after chunk deletion: vec=%v err=%v", vec, err)
	}
	if vec[0] != 1 || vec[1] != 0 {
		t.Errorf("cache returned wrong vector: %v", vec)
	}

	if vec, _ := st.CachedVector("sha-stable", "other-model"); vec != nil {
		t.Errorf("cache hit across models must not happen")
	}
}

func TestVectorSearchRespectsTags(t *testing.T) {
	st := openTestStore(t)

	workID, _ := st.AddSource("/work", "work")
	homeID, _ := st.AddSource("/home", "home")
	st.UpsertFile(workID, "/work/a.md", "s1", 1, 1, chunksOf("work text"))
	st.UpsertFile(homeID, "/home/b.md", "s2", 1, 1, chunksOf("home text"))

	for _, src := range []int64{workID, homeID} {
		missing, _ := st.MissingEmbeddings(src)
		for _, m := range missing {
			st.PutEmbedding(m.ID, "sha"+m.Text, "m", []float32{1, 0})
		}
	}

	r, err := st.VectorSearch([]float32{1, 0}, "work", 10)
	if err != nil || len(r) != 1 || r[0].Path != "/work/a.md" {
		t.Fatalf("tag-filtered vector search wrong: %+v err=%v", r, err)
	}
}
