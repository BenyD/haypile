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
	if !info.IsDir() {
		return stats, fmt.Errorf("%s is not a folder", abs)
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

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		sha := hex.EncodeToString(sum[:])
		seen[path] = true

		stored, err := st.FileSHA(path)
		if err != nil {
			return err
		}
		if stored == sha {
			stats.Skipped++
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}
		chunks := Split(string(data))
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
		}
		if err := st.UpsertFile(sourceID, path, sha, fi.Size(), fi.ModTime().Unix(), texts); err != nil {
			return err
		}

		stats.Indexed++
		stats.Chunks += len(texts)
		if progress != nil {
			progress(path)
		}
		return nil
	})
	if err != nil {
		return stats, err
	}
	if err := st.PruneFiles(sourceID, seen); err != nil {
		return stats, err
	}

	if emb != nil {
		embedded, err := embedMissing(st, sourceID, emb)
		stats.Embedded = embedded
		if err != nil {
			return stats, err
		}
	}
	return stats, nil
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
