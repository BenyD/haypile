// Package eval runs the retrieval-quality gate: every query in queries.yaml
// must rank an expected document in the top results against the fixture
// corpus. This is a merge-blocking test, not a benchmark — a score drop here
// is a regression in the product's core promise.
package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
)

const topK = 3

type evalCase struct {
	Query   string   `yaml:"query"`
	Expects []string `yaml:"expects"`
	Kind    string   `yaml:"kind"`
}

type querySet struct {
	Queries []evalCase `yaml:"queries"`
}

func TestRetrievalEval(t *testing.T) {
	data, err := os.ReadFile("queries.yaml")
	if err != nil {
		t.Fatalf("read queries.yaml: %v", err)
	}
	var qs querySet
	if err := yaml.Unmarshal(data, &qs); err != nil {
		t.Fatalf("parse queries.yaml: %v", err)
	}
	if len(qs.Queries) == 0 {
		t.Fatal("queries.yaml has no queries — the eval gate is not allowed to be empty")
	}

	st, err := index.Open(filepath.Join(t.TempDir(), "eval.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	stats, err := ingest.IndexFolder(st, "corpus", "", nil)
	if err != nil {
		t.Fatalf("index corpus: %v", err)
	}
	if stats.Indexed == 0 {
		t.Fatal("corpus indexed zero files")
	}

	for _, c := range qs.Queries {
		t.Run(c.Query, func(t *testing.T) {
			if c.Kind == "semantic" {
				t.Skipf("semantic retrieval lands at M1")
			}

			results, err := st.Search(c.Query, "", topK)
			if err != nil {
				t.Fatalf("search: %v", err)
			}

			var got []string
			for _, r := range results {
				got = append(got, r.Path)
			}
			for _, expect := range c.Expects {
				suffix := filepath.FromSlash(expect)
				for _, path := range got {
					if strings.HasSuffix(path, suffix) {
						return // hit
					}
				}
			}
			t.Errorf("no expected file in top %d.\n  expected one of: %v\n  got: %v",
				topK, c.Expects, got)
		})
	}
}
