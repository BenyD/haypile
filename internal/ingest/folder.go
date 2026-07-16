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
)

// embedBatch is how many chunks are sent to the embedder per call.
const embedBatch = 64

// Stats reports what one IndexFolder pass did.
type Stats struct {
	Indexed  int // files (re)indexed this pass
	Skipped  int // files already indexed and unchanged
	Failed   int // files whose extraction failed (indexing continues)
	Chunks   int // chunks written this pass
	Embedded int // vectors computed this pass (cache hits excluded)
}

// IndexFolder walks folder, indexes every supported file into st, and prunes
// records for files that vanished from disk. Unchanged files (same content
// hash) are skipped. If emb is non-nil, chunks are embedded for semantic
// search, cheapest first: the content-addressed cache is consulted before
// the model runs. progress, if non-nil, is called with each path as it is
// indexed.
func IndexFolder(st *index.Store, folder, tag string, emb embed.Embedder, progress func(path string)) (Stats, error) {
	var stats Stats

	abs, err := filepath.Abs(folder)
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
		if err := indexFile(st, sourceID, abs, &stats, progress); err != nil {
			return stats, err
		}
		return stats, embedIfConfigured(st, sourceID, emb, &stats)
	}

	sourceID, err := st.AddSource(abs, tag)
	if err != nil {
		return stats, err
	}

	seen := make(map[string]bool)
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories (.git and friends).
			if name := d.Name(); strings.HasPrefix(name, ".") && path != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if !Supported(path) {
			return nil
		}
		// A file that fails to index is not pruned: its previous version
		// stays searchable until a readable version appears.
		seen[path] = true
		return indexFile(st, sourceID, path, &stats, progress)
	})
	if err != nil {
		return stats, err
	}
	if err := st.PruneFiles(sourceID, seen); err != nil {
		return stats, err
	}

	return stats, embedIfConfigured(st, sourceID, emb, &stats)
}

// indexFile brings one file up to date in the index. Unreadable or
// unparseable files are counted, not fatal — one bad document must never
// abort a pass; only storage errors do.
func indexFile(st *index.Store, sourceID int64, path string, stats *Stats, progress func(path string)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		stats.Failed++
		return nil
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

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
	if progress != nil {
		progress(path)
	}
	return nil
}

func embedIfConfigured(st *index.Store, sourceID int64, emb embed.Embedder, stats *Stats) error {
	if emb == nil {
		return nil
	}
	embedded, err := embedMissing(st, sourceID, emb)
	stats.Embedded = embedded
	return err
}

// embedMissing vectorizes every chunk under the source that has no vector
// yet. Cache hits are stored directly; misses go to the embedder in batches.
func embedMissing(st *index.Store, sourceID int64, emb embed.Embedder) (int, error) {
	if err := st.SetEmbedModel(emb.Model()); err != nil {
		return 0, err
	}
	missing, err := st.MissingEmbeddings(sourceID)
	if err != nil {
		return 0, err
	}

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
		pending = pending[:0]
		pendingSHAs = pendingSHAs[:0]
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
