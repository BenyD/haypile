// Package realworld runs the retrieval baseline against a corpus of real
// public PDFs (fetched by fetch-corpus.sh, kept out of git). Unlike the
// fixture eval one directory up, which is a merge-blocking gate, this test
// reports recall@3 as a number to be recorded per release. It fails only
// below a coarse floor; the recorded baseline is what catches drift.
package realworld

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
	"github.com/BenyD/haypile/internal/query"
)

const (
	topK = 3
	// floor is a smoke threshold, not the quality bar. The bar is the
	// recorded baseline in BASELINE.md not going down.
	floor = 0.5
)

type evalCase struct {
	Query   string   `yaml:"query"`
	Expects []string `yaml:"expects"`
	Kind    string   `yaml:"kind"`
}

type querySet struct {
	Queries []evalCase `yaml:"queries"`
}

func TestRealWorldRecall(t *testing.T) {
	if _, err := os.Stat("corpus"); err != nil {
		t.Skip("no corpus: run eval/realworld/fetch-corpus.sh first")
	}

	data, err := os.ReadFile("queries.yaml")
	if err != nil {
		t.Fatalf("read queries.yaml: %v", err)
	}
	var qs querySet
	if err := yaml.Unmarshal(data, &qs); err != nil {
		t.Fatalf("parse queries.yaml: %v", err)
	}
	if len(qs.Queries) == 0 {
		t.Fatal("queries.yaml has no queries")
	}

	const weights = "../../internal/embed/bundled/model.safetensors"
	if os.Getenv("HAYPILE_MODEL_PATH") == "" && os.Getenv("HAYPILE_EMBED_ENDPOINT") == "" {
		if _, err := os.Stat(weights); err == nil {
			t.Setenv("HAYPILE_MODEL_PATH", weights)
		}
	}
	emb, err := embed.FromEnv()
	if err != nil {
		t.Fatalf("embedder config: %v", err)
	}
	if emb == nil {
		t.Skip("no embedder: the real-world baseline is a semantic measurement (run hack/fetch-model.sh)")
	}

	st, err := index.Open(filepath.Join(t.TempDir(), "realworld.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	stats, err := ingest.IndexFolder(st, "corpus", "", emb, nil)
	if err != nil {
		t.Fatalf("index corpus: %v", err)
	}
	t.Logf("indexed %d files (%d chunks)", stats.Indexed, stats.Chunks)
	if stats.Indexed == 0 {
		t.Fatal("corpus indexed zero files")
	}

	hits := 0
	for _, c := range qs.Queries {
		results, err := query.Hybrid(context.Background(), st, emb, c.Query, "", topK)
		if err != nil {
			t.Fatalf("search %q: %v", c.Query, err)
		}
		var got []string
		hit := false
		for _, r := range results {
			got = append(got, filepath.Base(r.Path))
		}
	expects:
		for _, expect := range c.Expects {
			suffix := filepath.FromSlash(expect)
			for _, r := range results {
				if strings.HasSuffix(r.Path, suffix) {
					hit = true
					break expects
				}
			}
		}
		if hit {
			hits++
		} else {
			t.Logf("MISS  %-60q top%d: %v", c.Query, topK, got)
		}
	}

	recall := float64(hits) / float64(len(qs.Queries))
	t.Logf("recall@%d = %.2f (%d/%d)", topK, recall, hits, len(qs.Queries))
	if recall < floor {
		t.Errorf("recall@%d = %.2f is below the smoke floor %.2f", topK, recall, floor)
	}
}
