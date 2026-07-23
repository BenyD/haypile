package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/BenyD/haypile/internal/llm"
	"github.com/BenyD/haypile/internal/query"
)

// AskRequest is POST /api/ask.
type AskRequest struct {
	Question string `json:"question"`
	Tag      string `json:"tag,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// AskSource is one retrieved passage in the stream's sources event —
// a QueryResult plus the human-readable citation label the answer's
// [n] markers refer to.
type AskSource struct {
	QueryResult
	Label string `json:"label"`
}

// handleAsk streams a RAG answer as server-sent events:
//
//	event: sources   data: [AskSource, ...]     (always first)
//	event: token     data: "text"               (JSON string, repeated)
//	event: error     data: {"message": "..."}   (stream aborts after)
//	event: done      data: {}                   (always last)
//
// No local LLM is not a stream: it is a plain 503 with the same
// explanation the CLI prints, so the UI can fall back to search.
func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, errors.New("question is required"))
		return
	}
	if req.Limit <= 0 || req.Limit > 20 {
		req.Limit = 6
	}

	// Everything that can fail with a plain status must fail before the
	// SSE headers are written.
	results, err := query.HybridForAnswer(r.Context(), s.st, s.emb, req.Question, req.Tag, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	client, err := llm.Detect(r.Context(), "", "")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	event := func(name string, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		flusher.Flush()
	}

	sources := make([]AskSource, len(results))
	for i, res := range results {
		sources[i] = AskSource{
			QueryResult: QueryResult{
				Path: res.Path, Page: res.Page, Chunk: res.Seq,
				Snippet: res.Snippet, Score: res.Score,
			},
			Label: llm.Citation(res),
		}
	}
	event("sources", sources)

	if len(results) == 0 {
		event("error", map[string]string{
			"message": "Nothing relevant indexed. Have you added a folder? (hay add <folder>)",
		})
		event("done", struct{}{})
		return
	}

	err = llm.AnswerStream(r.Context(), client, req.Question, results, func(token string) error {
		event("token", token)
		return nil
	})
	if err != nil {
		event("error", map[string]string{"message": err.Error()})
	}
	event("done", struct{}{})
}

// handleChunk serves GET /api/chunk?path=&chunk=&window=: the cited
// chunk's full text with its neighbors, so a citation can be read in
// place without re-opening the source file.
func (s *Server) handleChunk(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path is required"))
		return
	}
	seq, err := strconv.Atoi(r.URL.Query().Get("chunk"))
	if err != nil || seq < 0 {
		writeError(w, http.StatusBadRequest, errors.New("chunk must be a non-negative integer"))
		return
	}
	window := 1
	if v := r.URL.Query().Get("window"); v != "" {
		if window, err = strconv.Atoi(v); err != nil || window < 0 || window > 5 {
			writeError(w, http.StatusBadRequest, errors.New("window must be 0-5"))
			return
		}
	}

	passages, err := s.st.ChunkContext(path, seq, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(passages) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("no indexed chunk %d for %s", seq, path))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     path,
		"passages": passages,
	})
}
