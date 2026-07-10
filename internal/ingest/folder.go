package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BenyD/haypile/internal/index"
)

// Stats reports what one IndexFolder pass did.
type Stats struct {
	Indexed int // files (re)indexed this pass
	Skipped int // files already indexed and unchanged
	Chunks  int // chunks written this pass
}

// IndexFolder walks folder, indexes every supported file into st, and prunes
// records for files that vanished from disk. Unchanged files (same content
// hash) are skipped. progress, if non-nil, is called with each path as it is
// indexed.
func IndexFolder(st *index.Store, folder, tag string, progress func(path string)) (Stats, error) {
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

	return stats, st.PruneFiles(sourceID, seen)
}
