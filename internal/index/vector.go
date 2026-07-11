package index

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
)

// EmbedModel returns the model this index's vectors were built with, or ""
// if no model has been recorded yet.
func (s *Store) EmbedModel() (string, error) {
	var m string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'embed_model'`).Scan(&m)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return m, err
}

// SetEmbedModel records the embedding model for this index. Vectors from
// different models are not comparable, so changing models requires
// re-embedding: attempting to mix is an error, not a silent quality bug.
func (s *Store) SetEmbedModel(model string) error {
	current, err := s.EmbedModel()
	if err != nil {
		return err
	}
	if current == model {
		return nil
	}
	if current != "" {
		return fmt.Errorf(
			"index was embedded with %q but %q is configured; re-index to switch models (hay remove + hay add)",
			current, model)
	}
	_, err = s.db.Exec(`INSERT INTO meta(key, value) VALUES ('embed_model', ?)`, model)
	return err
}

// ChunkText identifies one chunk awaiting a vector.
type ChunkText struct {
	ID   int64
	Text string
}

// MissingEmbeddings lists chunks under a source that have no vector yet —
// new chunks, plus any indexed while no embedder was configured.
func (s *Store) MissingEmbeddings(sourceID int64) ([]ChunkText, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.text
		FROM chunks c
		JOIN files f ON f.id = c.file_id
		LEFT JOIN embeddings e ON e.chunk_id = c.id
		WHERE f.source_id = ? AND e.chunk_id IS NULL
		ORDER BY c.id`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChunkText
	for rows.Next() {
		var ct ChunkText
		if err := rows.Scan(&ct.ID, &ct.Text); err != nil {
			return nil, err
		}
		out = append(out, ct)
	}
	return out, rows.Err()
}

// CachedVector looks up a previously computed vector by content hash.
func (s *Store) CachedVector(sha, model string) ([]float32, error) {
	var blob []byte
	err := s.db.QueryRow(
		`SELECT vector FROM embedding_cache WHERE sha = ? AND model = ?`, sha, model).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return blobToVec(blob), nil
}

// PutEmbedding stores a chunk's vector and populates the content cache.
func (s *Store) PutEmbedding(chunkID int64, sha, model string, vec []float32) error {
	blob := vecToBlob(vec)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT OR REPLACE INTO embeddings(chunk_id, vector) VALUES (?, ?)`, chunkID, blob); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO embedding_cache(sha, model, vector) VALUES (?, ?, ?)`, sha, model, blob); err != nil {
		return err
	}
	return tx.Commit()
}

// VectorSearch returns the chunks nearest to query by dot product (vectors
// are L2-normalized, so this is cosine similarity). Brute force is the v1
// strategy per the PRD: linear scan is well within budget at personal-corpus
// scale, and an ANN index is a v2 concern.
func (s *Store) VectorSearch(query []float32, tag string, limit int) ([]Result, error) {
	rows, err := s.db.Query(`
		SELECT f.path, c.seq, c.text, e.vector
		FROM embeddings e
		JOIN chunks c ON c.id = e.chunk_id
		JOIN files f ON f.id = c.file_id
		JOIN sources s ON s.id = f.source_id
		WHERE (? = '' OR s.tag = ?)`, tag, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		var text string
		var blob []byte
		if err := rows.Scan(&r.Path, &r.Seq, &text, &blob); err != nil {
			return nil, err
		}
		r.Score = float64(dot(query, blobToVec(blob)))
		r.Snippet = excerpt(text, 160)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func dot(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

func excerpt(text string, max int) string {
	if len(text) <= max {
		return text
	}
	cut := max
	for cut > 0 && text[cut]&0xC0 == 0x80 { // don't split a UTF-8 rune
		cut--
	}
	return text[:cut] + "…"
}

func vecToBlob(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func blobToVec(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}
