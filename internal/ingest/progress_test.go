package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
)

// unitEmbedder returns a fixed unit vector per text; enough to drive the
// embedding phase without a model.
type unitEmbedder struct{}

func (unitEmbedder) Model() string { return "test/unit" }
func (unitEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0}
	}
	return out, nil
}

var _ embed.Embedder = unitEmbedder{}

// The progress stream is a promise the UI builds an ETA on: totals fixed
// up front, counts monotonic, every file accounted for whatever its
// outcome, and the embedding phase arriving after extraction with its
// own totals.
func TestIndexFolderProgress(t *testing.T) {
	dir := t.TempDir()
	var wantBytes int64
	for _, f := range []struct{ name, body string }{
		{"a.md", "# alpha\n\nsome alpha text"},
		{"b.md", "# beta\n\nsome beta text"},
		{"c.txt", "gamma plain text"},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
		wantBytes += int64(len(f.body))
	}

	st, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var events []Progress
	_, err = IndexFolder(st, dir, "", unitEmbedder{}, func(p Progress) {
		events = append(events, p)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no progress events")
	}

	phaseFlip := -1
	for i, p := range events {
		if p.Phase == PhaseEmbedding {
			phaseFlip = i
			break
		}
	}
	if phaseFlip == -1 {
		t.Fatal("embedding phase never reported")
	}

	for i, p := range events[:phaseFlip] {
		if p.Phase != PhaseExtracting {
			t.Fatalf("event %d: phase %q before embedding began", i, p.Phase)
		}
		if p.FilesTotal != 3 || p.BytesTotal != wantBytes {
			t.Fatalf("event %d: totals (%d files, %d bytes), want (3, %d)",
				i, p.FilesTotal, p.BytesTotal, wantBytes)
		}
		if i > 0 && (p.FilesDone < events[i-1].FilesDone || p.BytesDone < events[i-1].BytesDone) {
			t.Fatalf("event %d: progress went backwards", i)
		}
	}
	last := events[phaseFlip-1]
	if last.FilesDone != 3 || last.BytesDone != wantBytes {
		t.Fatalf("extraction ended at %d files, %d bytes; want 3, %d",
			last.FilesDone, last.BytesDone, wantBytes)
	}

	final := events[len(events)-1]
	if final.Phase != PhaseEmbedding || final.ChunksDone != final.ChunksTotal || final.ChunksTotal == 0 {
		t.Fatalf("embedding ended at %d/%d chunks", final.ChunksDone, final.ChunksTotal)
	}

	// Re-adding an unchanged folder must still walk to 100%: skipped
	// files are finished work, not invisible work.
	events = events[:0]
	_, err = IndexFolder(st, dir, "", unitEmbedder{}, func(p Progress) {
		events = append(events, p)
	})
	if err != nil {
		t.Fatal(err)
	}
	var sawFullWalk bool
	for _, p := range events {
		if p.Phase == PhaseExtracting && p.FilesDone == 3 {
			sawFullWalk = true
		}
	}
	if !sawFullWalk {
		t.Fatal("unchanged re-add never reported the walk completing")
	}
}
