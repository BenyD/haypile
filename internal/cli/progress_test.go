package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/BenyD/haypile/internal/ingest"
)

func TestETATrackerConverges(t *testing.T) {
	var e etaTracker
	now := time.Now()

	// First sample of a phase only calibrates; no estimate yet.
	if _, ok := e.update(ingest.PhaseExtracting, 0, 1000, now); ok {
		t.Fatal("estimate before any measured rate")
	}
	// Steady 100 units/second for 5 samples: ~5s left at 500/1000.
	for i := 1; i <= 5; i++ {
		now = now.Add(time.Second)
		eta, ok := e.update(ingest.PhaseExtracting, int64(i*100), 1000, now)
		if i > 1 && !ok {
			t.Fatalf("sample %d: no estimate", i)
		}
		if i == 5 {
			if eta < 4*time.Second || eta > 7*time.Second {
				t.Fatalf("eta %v, want ~5s", eta)
			}
		}
	}

	// Phase change throws the old rate away: chunks/sec is not bytes/sec.
	if _, ok := e.update(ingest.PhaseEmbedding, 0, 500, now.Add(time.Second)); ok {
		t.Fatal("estimate carried across a phase change")
	}
}

func TestRenderProgress(t *testing.T) {
	p := ingest.Progress{Phase: ingest.PhaseExtracting, FilesDone: 151, FilesTotal: 312, BytesDone: 1, BytesTotal: 2}
	line := renderProgress(p, 2*time.Minute, true)
	for _, want := range []string{"extracting", "48%", "151/312 files", "~2m left"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}

	p = ingest.Progress{Phase: ingest.PhaseEmbedding, ChunksDone: 14000, ChunksTotal: 62000}
	line = renderProgress(p, 0, false)
	for _, want := range []string{"embedding", "22%", "14.0k/62.0k chunks"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
	if strings.Contains(line, "left") {
		t.Fatalf("line %q shows an ETA it does not have", line)
	}
}

func TestMilestoneReporterEmitsOncePerTenth(t *testing.T) {
	var m milestoneReporter
	var lines []string
	// One snapshot per chunk through a small pass: 10 lines, not 100.
	for done := 0; done <= 100; done++ {
		p := ingest.Progress{Phase: ingest.PhaseEmbedding, ChunksDone: done, ChunksTotal: 100}
		if line := m.step(&p); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 11 { // 0%, 10% … 100%
		t.Fatalf("got %d lines, want 11:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], "0%") || !strings.Contains(lines[0], "embedding") {
		t.Errorf("first line %q", lines[0])
	}
	if !strings.Contains(lines[10], "100%") || !strings.Contains(lines[10], "100/100 chunks") {
		t.Errorf("last line %q", lines[10])
	}
}

func TestMilestoneReporterResetsPerPhase(t *testing.T) {
	var m milestoneReporter
	ext := ingest.Progress{Phase: ingest.PhaseExtracting, FilesDone: 10, FilesTotal: 10}
	if line := m.step(&ext); !strings.Contains(line, "extracting") {
		t.Fatalf("extracting line %q", line)
	}
	// Embedding starts at 0% after extraction hit 100%; a shared counter
	// would swallow every line of the new phase.
	emb := ingest.Progress{Phase: ingest.PhaseEmbedding, ChunksDone: 0, ChunksTotal: 500}
	if line := m.step(&emb); !strings.Contains(line, "embedding") {
		t.Fatalf("embedding start swallowed, got %q", line)
	}
}

func TestMilestoneReporterIgnoresEmptySnapshots(t *testing.T) {
	var m milestoneReporter
	if line := m.step(nil); line != "" {
		t.Errorf("nil snapshot printed %q", line)
	}
	// A phase that has not counted its work yet is not 0% of anything.
	p := ingest.Progress{Phase: ingest.PhaseEmbedding, ChunksTotal: 0}
	if line := m.step(&p); line != "" {
		t.Errorf("zero-total snapshot printed %q", line)
	}
}

func TestFmtETA(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "almost done"},
		{47 * time.Second, "~45s left"},
		{3 * time.Minute, "~3m left"},
	}
	for _, c := range cases {
		if got := fmtETA(c.d); got != c.want {
			t.Errorf("fmtETA(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
