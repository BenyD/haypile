// Package index owns Haypile's storage: a single SQLite file holding
// sources, files, chunks, and the FTS5 keyword index. The vector index
// (sqlite-vec) joins this schema at M1.
package index

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

const schema = `
CREATE TABLE IF NOT EXISTS sources (
	id       INTEGER PRIMARY KEY,
	path     TEXT NOT NULL UNIQUE,
	tag      TEXT NOT NULL DEFAULT '',
	added_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS files (
	id        INTEGER PRIMARY KEY,
	source_id INTEGER NOT NULL REFERENCES sources(id),
	path      TEXT NOT NULL UNIQUE,
	sha256    TEXT NOT NULL,
	size      INTEGER NOT NULL,
	mtime     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
	id      INTEGER PRIMARY KEY,
	file_id INTEGER NOT NULL REFERENCES files(id),
	seq     INTEGER NOT NULL,
	page    INTEGER NOT NULL DEFAULT 0, -- 1-based; 0 = format has no pages
	text    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chunks_file ON chunks(file_id);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

-- One vector per chunk, from the model recorded in meta('embed_model').
CREATE TABLE IF NOT EXISTS embeddings (
	chunk_id INTEGER PRIMARY KEY REFERENCES chunks(id),
	vector   BLOB NOT NULL
);

-- Content-addressed cache: the same text is never embedded twice, even
-- across files, re-indexes, or removals.
CREATE TABLE IF NOT EXISTS embedding_cache (
	sha    TEXT NOT NULL,
	model  TEXT NOT NULL,
	vector BLOB NOT NULL,
	PRIMARY KEY (sha, model)
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
	text,
	content='chunks',
	content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
	INSERT INTO chunks_fts(rowid, text) VALUES (new.id, new.text);
END;
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, text) VALUES ('delete', old.id, old.text);
END;
`

// Store is the handle to one Haypile database.
type Store struct {
	db *sql.DB
}

// DefaultPath returns the database location: $HAYPILE_DIR/haypile.db if set,
// otherwise ~/.haypile/haypile.db.
func DefaultPath() string {
	if dir := os.Getenv("HAYPILE_DIR"); dir != "" {
		return filepath.Join(dir, "haypile.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "haypile.db"
	}
	return filepath.Join(home, ".haypile", "haypile.db")
}

// Open creates or opens the database at path and applies the schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := driver.Open(
		"file:"+path+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=synchronous(normal)",
		fts5.Register)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Pre-1.0 migration: indexes created before page-number citations (M2)
	// lack the column. Duplicate-column errors mean already migrated.
	if _, err := db.Exec(`ALTER TABLE chunks ADD COLUMN page INTEGER NOT NULL DEFAULT 0`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// AddSource registers a folder (idempotent) and returns its id.
func (s *Store) AddSource(path, tag string) (int64, error) {
	_, err := s.db.Exec(`
		INSERT INTO sources(path, tag) VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET tag = excluded.tag`, path, tag)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(`SELECT id FROM sources WHERE path = ?`, path).Scan(&id)
	return id, err
}

// SourceID looks up a source by its exact registered path.
func (s *Store) SourceID(path string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM sources WHERE path = ?`, path).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%s is not an indexed source", path)
	}
	return id, err
}

// SourceTag returns the tag a source was registered with.
func (s *Store) SourceTag(path string) (string, error) {
	var tag string
	err := s.db.QueryRow(`SELECT tag FROM sources WHERE path = ?`, path).Scan(&tag)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return tag, err
}

// SourceInfo summarizes one indexed folder.
type SourceInfo struct {
	Path   string
	Tag    string
	Files  int
	Chunks int
}

// Sources lists all indexed folders with document and chunk counts.
func (s *Store) Sources() ([]SourceInfo, error) {
	rows, err := s.db.Query(`
		SELECT s.path, s.tag,
			(SELECT COUNT(*) FROM files f WHERE f.source_id = s.id),
			(SELECT COUNT(*) FROM chunks c JOIN files f ON c.file_id = f.id WHERE f.source_id = s.id)
		FROM sources s ORDER BY s.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SourceInfo
	for rows.Next() {
		var si SourceInfo
		if err := rows.Scan(&si.Path, &si.Tag, &si.Files, &si.Chunks); err != nil {
			return nil, err
		}
		out = append(out, si)
	}
	return out, rows.Err()
}

// RemoveSource un-indexes a folder and everything under it. It reports
// whether the folder was indexed at all.
func (s *Store) RemoveSource(path string) (bool, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM sources WHERE path = ?`, path).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM embeddings WHERE chunk_id IN
		(SELECT c.id FROM chunks c JOIN files f ON c.file_id = f.id WHERE f.source_id = ?)`, id); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM chunks WHERE file_id IN (SELECT id FROM files WHERE source_id = ?)`, id); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE source_id = ?`, id); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM sources WHERE id = ?`, id); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// FileSHA returns the stored content hash for path, or "" if unknown.
func (s *Store) FileSHA(path string) (string, error) {
	var sha string
	err := s.db.QueryRow(`SELECT sha256 FROM files WHERE path = ?`, path).Scan(&sha)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return sha, err
}

// Chunk is one indexable piece of a document as the store receives it.
type Chunk struct {
	Text string
	Page int // 1-based page number; 0 when the format has no pages
}

// UpsertFile records a file and replaces its chunks atomically.
func (s *Store) UpsertFile(sourceID int64, path, sha string, size, mtime int64, chunks []Chunk) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO files(source_id, path, sha256, size, mtime) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			source_id = excluded.source_id, sha256 = excluded.sha256,
			size = excluded.size, mtime = excluded.mtime`,
		sourceID, path, sha, size, mtime); err != nil {
		return err
	}

	var fileID int64
	if err := tx.QueryRow(`SELECT id FROM files WHERE path = ?`, path).Scan(&fileID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM embeddings WHERE chunk_id IN
		(SELECT id FROM chunks WHERE file_id = ?)`, fileID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chunks WHERE file_id = ?`, fileID); err != nil {
		return err
	}

	ins, err := tx.Prepare(`INSERT INTO chunks(file_id, seq, page, text) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for i, c := range chunks {
		if _, err := ins.Exec(fileID, i, c.Page, c.Text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PruneFiles drops files under a source that are no longer on disk.
func (s *Store) PruneFiles(sourceID int64, keep map[string]bool) error {
	rows, err := s.db.Query(`SELECT id, path FROM files WHERE source_id = ?`, sourceID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var stale []int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return err
		}
		if !keep[path] {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range stale {
		if _, err := s.db.Exec(`DELETE FROM embeddings WHERE chunk_id IN
			(SELECT id FROM chunks WHERE file_id = ?)`, id); err != nil {
			return err
		}
		if _, err := s.db.Exec(`DELETE FROM chunks WHERE file_id = ?`, id); err != nil {
			return err
		}
		if _, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}

// Result is one search hit with its citation.
type Result struct {
	Path    string
	Seq     int
	Page    int // 1-based page number; 0 when the format has no pages
	Snippet string
	Score   float64
}

// Passage is one chunk with its full text, as served by the chunk-context
// API: a cited chunk plus its neighbors so a citation can be read in
// place.
type Passage struct {
	Seq     int    `json:"chunk"`
	Page    int    `json:"page,omitempty"` // 1-based; 0 = format has no pages
	Text    string `json:"text"`
	Current bool   `json:"current"` // the chunk the citation points at
}

// ChunkContext returns the chunk at seq in the file at path together
// with up to window neighbors on each side, in order. An empty slice
// means the file (or that chunk) is not in the index.
func (s *Store) ChunkContext(path string, seq, window int) ([]Passage, error) {
	rows, err := s.db.Query(`
		SELECT c.seq, c.page, c.text
		FROM chunks c
		JOIN files f ON f.id = c.file_id
		WHERE f.path = ? AND c.seq BETWEEN ? AND ?
		ORDER BY c.seq`, path, seq-window, seq+window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Passage
	found := false
	for rows.Next() {
		var p Passage
		if err := rows.Scan(&p.Seq, &p.Page, &p.Text); err != nil {
			return nil, err
		}
		p.Current = p.Seq == seq
		found = found || p.Current
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, nil // neighbors without the cited chunk are not context
	}
	return out, nil
}

// Search runs a ranked FTS5 keyword query; every word must match. tag
// narrows results to sources indexed with that tag; empty tag searches
// everything.
func (s *Store) Search(query, tag string, limit int) ([]Result, error) {
	words := ftsWords(query)
	if len(words) == 0 {
		return nil, errors.New("empty query")
	}
	return s.searchFTS(strings.Join(words, " "), tag, limit)
}

// SearchAny is the recall variant: any word may match, BM25 ranks the
// best overlap on top. Hybrid search uses it only when there is no
// semantic leg to catch paraphrases — as a fusion input its one-word
// matches would dilute good vector rankings.
func (s *Store) SearchAny(query, tag string, limit int) ([]Result, error) {
	words := ftsWords(query)
	if len(words) == 0 {
		return nil, errors.New("empty query")
	}
	return s.searchFTS(strings.Join(words, " OR "), tag, limit)
}

func (s *Store) searchFTS(fts, tag string, limit int) ([]Result, error) {
	rows, err := s.db.Query(`
		SELECT f.path, c.seq, c.page, snippet(chunks_fts, 0, '', '', ' … ', 16), -bm25(chunks_fts)
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN files f ON f.id = c.file_id
		JOIN sources s ON s.id = f.source_id
		WHERE chunks_fts MATCH ? AND (? = '' OR s.tag = ?)
		ORDER BY bm25(chunks_fts)
		LIMIT ?`, fts, tag, tag, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Path, &r.Seq, &r.Page, &r.Snippet, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ftsWords converts free text into safe FTS5 tokens: each word is quoted,
// so user input can never be misparsed as FTS5 syntax like NEAR, AND, or
// column filters.
func ftsWords(q string) []string {
	words := strings.Fields(q)
	for i, w := range words {
		words[i] = `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
	}
	return words
}
