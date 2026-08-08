package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/pathnorm"
)

// embedBatch is how many chunks are sent to the embedder per call.
const embedBatch = 64

// extractVersion salts the stored content hash. Bump it whenever
// extraction or chunking output changes, so already-indexed files stop
// matching and re-index on the next pass; re-embedding stays cheap
// because unchanged chunk text hits the embedding cache.
const extractVersion = "2"

// Stats reports what one IndexFolder pass did.
type Stats struct {
	Indexed  int // files (re)indexed this pass
	Skipped  int // files already indexed and unchanged
	Failed   int // files whose extraction failed (indexing continues)
	Chunks   int // chunks written this pass
	Embedded int // vectors computed this pass (cache hits excluded)
	// ScanSkipped counts pages that looked scanned (an image, no text)
	// but had no vision model to transcribe them; ScanFailed counts
	// pages where the model errored or read nothing. Both index empty.
	// Surfaced by the CLI and web UI so the silence is never mistaken
	// for a broken index.
	ScanSkipped int
	ScanFailed  int
}

// Progress phases, in the order a pass runs them.
const (
	PhaseExtracting = "extracting"
	PhaseEmbedding  = "embedding"
)

// Progress is a snapshot of one indexing pass. Extraction is measured in
// files and bytes because that is what the walk knows up front; embedding
// is measured in chunks because extraction only just created them. Totals
// are fixed for the life of a phase, so done/total is an honest fraction.
type Progress struct {
	Phase       string `json:"phase"`
	Path        string `json:"path,omitempty"` // file just finished (extracting)
	FilesDone   int    `json:"files_done"`
	FilesTotal  int    `json:"files_total"`
	BytesDone   int64  `json:"bytes_done"`
	BytesTotal  int64  `json:"bytes_total"`
	ChunksDone  int    `json:"chunks_done"`
	ChunksTotal int    `json:"chunks_total"`
}

// IndexFolder walks folder, indexes every supported file into st, and prunes
// records for files that vanished from disk. Unchanged files (same content
// hash) are skipped. If emb is non-nil, chunks are embedded for semantic
// search, cheapest first: the content-addressed cache is consulted before
// the model runs. progress, if non-nil, receives a snapshot after every
// file and every embedding batch.
func IndexFolder(st *index.Store, folder, tag string, emb embed.Embedder, progress func(Progress)) (Stats, error) {
	var stats Stats

	// Canonical casing here means every stored path — the source root and
	// the walked files under it — carries the actual on-disk casing, so
	// exact SQL equality is enough everywhere downstream.
	abs, err := pathnorm.Canon(folder)
	if err != nil {
		return stats, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return stats, err
	}

	// A single file is a valid source too: `hay add contract.pdf` should
	// just work. Unsupported formats error here (the user named this file
	// deliberately) where a folder walk would silently pass them by.
	if !info.IsDir() {
		if !Supported(abs) {
			return stats, fmt.Errorf("%s: unsupported format (want %s)", abs, supportedList())
		}
		sourceID, err := st.AddSource(abs, tag)
		if err != nil {
			return stats, err
		}
		tr := newTracker(1, info.Size(), progress)
		if err := indexFile(st, sourceID, abs, &stats, tr.fileDone); err != nil {
			return stats, err
		}
		return stats, embedIfConfigured(st, sourceID, emb, &stats, tr.embedDone)
	}

	// The folder's own config supplies the tag default and the exclude
	// patterns. Reconciliation is free: excluded files are never marked
	// seen, so the prune pass below drops any that were indexed before
	// the pattern existed.
	cfg, err := LoadConfig(abs)
	if err != nil {
		return stats, err
	}
	if tag == "" {
		tag = cfg.Tag
	}

	sourceID, err := st.AddSource(abs, tag)
	if err != nil {
		return stats, err
	}

	// walkEligible visits every file the pass will touch; used once to
	// total the work (so progress can be a fraction, not a spinner) and
	// once to do it. The counting walk is filesystem-metadata only, so
	// its cost is noise next to extraction.
	walkEligible := func(visit func(path string, size int64) error) error {
		return filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(abs, path)
			if relErr != nil {
				return relErr
			}
			if d.IsDir() {
				// Skip hidden directories (.git and friends).
				if name := d.Name(); strings.HasPrefix(name, ".") && path != abs {
					return filepath.SkipDir
				}
				if rel != "." && cfg.Excluded(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !Supported(path) || cfg.Excluded(rel) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil // vanished mid-walk; the next pass sees the truth
			}
			return visit(path, info.Size())
		})
	}

	var totalFiles int
	var totalBytes int64
	if progress != nil {
		if err := walkEligible(func(_ string, size int64) error {
			totalFiles++
			totalBytes += size
			return nil
		}); err != nil {
			return stats, err
		}
	}
	tr := newTracker(totalFiles, totalBytes, progress)

	seen := make(map[string]bool)
	err = walkEligible(func(path string, _ int64) error {
		// A file that fails to index is not pruned: its previous version
		// stays searchable until a readable version appears.
		seen[path] = true
		return indexFile(st, sourceID, path, &stats, tr.fileDone)
	})
	if err != nil {
		return stats, err
	}
	if err := st.PruneFiles(sourceID, seen); err != nil {
		return stats, err
	}

	return stats, embedIfConfigured(st, sourceID, emb, &stats, tr.embedDone)
}

// tracker turns per-file and per-batch events into Progress snapshots.
// One exists per pass; a nil callback makes every method a no-op.
type tracker struct {
	cb  func(Progress)
	cur Progress
}

func newTracker(files int, bytes int64, cb func(Progress)) *tracker {
	return &tracker{cb: cb, cur: Progress{
		Phase:      PhaseExtracting,
		FilesTotal: files,
		BytesTotal: bytes,
	}}
}

// fileDone records one visited file, whatever its outcome: skipped and
// failed files are finished work too, or the fraction would lie on every
// re-add of an already-indexed folder.
func (t *tracker) fileDone(path string, size int64) {
	if t == nil || t.cb == nil {
		return
	}
	t.cur.Path = path
	t.cur.FilesDone++
	t.cur.BytesDone += size
	t.cb(t.cur)
}

// embedDone reports embedding progress in chunks. The first call flips
// the phase so a watcher can switch its display immediately.
func (t *tracker) embedDone(done, total int) {
	if t == nil || t.cb == nil {
		return
	}
	t.cur.Phase = PhaseEmbedding
	t.cur.Path = ""
	t.cur.ChunksDone = done
	t.cur.ChunksTotal = total
	t.cb(t.cur)
}

// IndexOne brings a single changed file under an existing source up to
// date — the daemon's watcher calls this per file event instead of
// re-walking the whole folder.
func IndexOne(st *index.Store, sourceID int64, path string, emb embed.Embedder) (Stats, error) {
	var stats Stats
	if err := indexFile(st, sourceID, path, &stats, nil); err != nil {
		return stats, err
	}
	return stats, embedIfConfigured(st, sourceID, emb, &stats, nil)
}

// indexFile brings one file up to date in the index. Unreadable or
// unparseable files are counted, not fatal — one bad document must never
// abort a pass; only storage errors do. done, if non-nil, is called once
// whatever the outcome: every visited file is finished work.
func indexFile(st *index.Store, sourceID int64, path string, stats *Stats, done func(path string, size int64)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		stats.Failed++
		if done != nil {
			done(path, 0)
		}
		return nil
	}
	if done != nil {
		defer done(path, int64(len(data)))
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:]) + ":" + extractVersion

	stored, err := st.FileSHA(path)
	if err != nil {
		return err
	}
	if stored == sha {
		stats.Skipped++
		return nil
	}

	fi, err := os.Stat(path)
	if err != nil {
		stats.Failed++
		return nil
	}
	sections, err := Extract(path)
	if err != nil {
		stats.Failed++
		return nil
	}
	for _, s := range sections {
		if s.ScanSkipped {
			stats.ScanSkipped++
		}
		if s.ScanFailed {
			stats.ScanFailed++
		}
	}
	chunks := SplitSections(sections)
	dbChunks := make([]index.Chunk, len(chunks))
	for i, c := range chunks {
		dbChunks[i] = index.Chunk{Text: c.Text, Page: c.Page}
	}
	if err := st.UpsertFile(sourceID, path, sha, fi.Size(), fi.ModTime().Unix(), dbChunks); err != nil {
		return err
	}

	stats.Indexed++
	stats.Chunks += len(chunks)
	return nil
}

func embedIfConfigured(st *index.Store, sourceID int64, emb embed.Embedder, stats *Stats, done func(done, total int)) error {
	if emb == nil {
		return nil
	}
	embedded, err := embedMissing(st, sourceID, emb, done)
	stats.Embedded = embedded
	return err
}

// embedMissing vectorizes every chunk under the source that has no vector
// yet. Cache hits are stored directly; misses go to the embedder in batches.
// done, if non-nil, is told how many of the pass's chunks are stored so far;
// it fires immediately with (0, total) so watchers can flip phase at once.
func embedMissing(st *index.Store, sourceID int64, emb embed.Embedder, done func(done, total int)) (int, error) {
	if err := st.SetEmbedModel(emb.Model()); err != nil {
		return 0, err
	}
	missing, err := st.MissingEmbeddings(sourceID)
	if err != nil {
		return 0, err
	}

	stored := 0
	report := func() {
		if done != nil {
			done(stored, len(missing))
		}
	}
	report()

	var pending []index.ChunkText
	var pendingSHAs []string
	embedded := 0

	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		texts := make([]string, len(pending))
		for i, p := range pending {
			texts[i] = p.Text
		}
		vecs, err := emb.Embed(context.Background(), texts)
		if err != nil {
			return fmt.Errorf("embedding %d chunks: %w", len(texts), err)
		}
		for i, p := range pending {
			if err := st.PutEmbedding(p.ID, pendingSHAs[i], emb.Model(), vecs[i]); err != nil {
				return err
			}
		}
		embedded += len(pending)
		stored += len(pending)
		pending = pending[:0]
		pendingSHAs = pendingSHAs[:0]
		report()
		return nil
	}

	for _, m := range missing {
		sum := sha256.Sum256([]byte(m.Text))
		sha := hex.EncodeToString(sum[:])

		cached, err := st.CachedVector(sha, emb.Model())
		if err != nil {
			return embedded, err
		}
		if cached != nil {
			if err := st.PutEmbedding(m.ID, sha, emb.Model(), cached); err != nil {
				return embedded, err
			}
			stored++
			report()
			continue
		}

		pending = append(pending, m)
		pendingSHAs = append(pendingSHAs, sha)
		if len(pending) >= embedBatch {
			if err := flush(); err != nil {
				return embedded, err
			}
		}
	}
	return embedded, flush()
}
