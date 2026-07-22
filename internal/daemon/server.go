// Package daemon is hay's long-running process: a localhost REST API and
// a filesystem watcher that keeps the index fresh. The CLI talks to it
// when it's running and falls back to direct index access when it isn't —
// users never have to learn the distinction.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
	"github.com/BenyD/haypile/internal/llm"
	"github.com/BenyD/haypile/internal/query"
	"github.com/BenyD/haypile/internal/webui"
)

// DefaultAddr is where the daemon listens. Localhost only: the API has no
// auth in v1 and must never be reachable from the network.
const DefaultAddr = "127.0.0.1:11500"

// Server is one running daemon.
type Server struct {
	st      *index.Store
	emb     embed.Embedder
	watcher *watcher
	http    *http.Server
	dataDir string
	dbPath  string
	version string
	started time.Time
}

// Run starts the daemon and blocks until ctx is cancelled: it opens the
// store, watches every registered source, serves the API on addr, and
// cleans up its runtime file on the way out.
func Run(ctx context.Context, addr, version string) error {
	dbPath := index.DefaultPath()
	st, err := index.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	emb, err := embed.FromEnv()
	if err != nil {
		return err
	}

	// Scanned PDF pages are transcribed by the user's local LLM when one
	// is around; the hook probes lazily, so this costs nothing up front.
	ingest.SetOCR(llm.OCRHook())

	s := &Server{
		st:      st,
		emb:     emb,
		dataDir: filepath.Dir(dbPath),
		dbPath:  dbPath,
		version: version,
		started: time.Now(),
	}

	s.watcher, err = newWatcher(st, emb)
	if err != nil {
		return fmt.Errorf("starting watcher: %w", err)
	}
	defer s.watcher.close()

	sources, err := st.Sources()
	if err != nil {
		return err
	}
	for _, src := range sources {
		if err := s.watcher.watchSource(src.Path); err != nil {
			// A vanished folder must not stop the daemon; it simply is
			// not watched until it reappears through a new `hay add`.
			fmt.Printf("warning: cannot watch %s: %v\n", src.Path, err)
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s (already running?): %w", addr, err)
	}
	if err := writeRuntimeFile(s.dataDir, ln.Addr().String()); err != nil {
		ln.Close()
		return err
	}
	defer removeRuntimeFile(s.dataDir)

	boundHost, _, _ := net.SplitHostPort(addr)
	s.http = &http.Server{Handler: hostGuard(boundHost, originGuard(s.routes()))}
	errc := make(chan error, 1)
	go func() { errc <- s.http.Serve(ln) }()
	fmt.Printf("hay daemon listening on http://%s\n", ln.Addr())

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// hostGuard rejects requests whose Host header does not name this
// machine. A malicious web page can point its own domain's DNS at
// 127.0.0.1 (DNS rebinding) and reach a localhost API from the victim's
// browser — but the Host header still carries the attacker's domain,
// so refusing unknown hosts closes the hole. When the daemon is bound
// to a wildcard address the user has deliberately exposed it, and the
// guard steps aside: it cannot know which external names are legit.
func hostGuard(boundHost string, next http.Handler) http.Handler {
	if boundHost == "" || boundHost == "0.0.0.0" || boundHost == "::" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]") // bare IPv6 literal
		if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != boundHost {
			writeError(w, http.StatusForbidden,
				fmt.Errorf("refusing request for host %q: this API answers only to localhost", host))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originGuard is the CSRF companion to hostGuard. A malicious page on
// any website can fire blind cross-site POSTs at localhost; the browser
// stamps those with the page's Origin, while same-origin requests from
// the web UI carry our own localhost origin and non-browser clients
// (CLI, curl, MCP editors) send none. Only a foreign origin is refused.
func originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && o != "null" {
			host := strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://")
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			host = strings.Trim(host, "[]")
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				writeError(w, http.StatusForbidden,
					fmt.Errorf("refusing cross-site request from origin %q", o))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", s.handleHealth)
	r.Get("/api/status", s.handleStatus)
	r.Post("/api/query", s.handleQuery)
	r.Post("/api/ask", s.handleAsk)
	r.Get("/api/chunk", s.handleChunk)
	r.Get("/api/browse", s.handleBrowse)
	r.Post("/api/pick", s.handlePick)
	r.Get("/api/sources", s.handleSources)
	r.Post("/api/sources", s.handleAddSource)
	r.Delete("/api/sources", s.handleRemoveSource)
	r.Post("/mcp", s.handleMCP)
	r.Get("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// No server-initiated streams: this server only answers requests.
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	// Everything else is the bundled web UI (hay web). Exact routes above
	// win; chi only falls through to the catch-all.
	r.Handle("/*", webui.Handler())
	return r
}

// Health identifies a live daemon and which database it serves — the CLI
// refuses to route through a daemon running against a different index.
type Health struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
	DB      string `json:"db"`
	Model   string `json:"model,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	h := Health{OK: true, Version: s.version, DB: s.dbPath}
	if s.emb != nil {
		h.Model = s.emb.Model()
	}
	writeJSON(w, http.StatusOK, h)
}

// Status is what `hay status` shows.
type Status struct {
	Health
	UptimeSeconds int64        `json:"uptime_seconds"`
	Sources       []SourceInfo `json:"sources"`
	Files         int          `json:"files"`
	Chunks        int          `json:"chunks"`
	PendingJobs   int          `json:"pending_jobs"`
	// OutboundConns is the daemon's live non-listening TCP connections —
	// the roadmap's verifiable "0 external connections" claim, measured, not
	// asserted.
	OutboundConns int    `json:"outbound_connections"`
	OutboundNote  string `json:"outbound_note,omitempty"`
}

type SourceInfo struct {
	Path   string `json:"path"`
	Tag    string `json:"tag,omitempty"`
	Files  int    `json:"files"`
	Chunks int    `json:"chunks"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sources, err := s.st.Sources()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	st := Status{
		Health:        Health{OK: true, Version: s.version, DB: s.dbPath},
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		PendingJobs:   s.watcher.pending(),
	}
	if s.emb != nil {
		st.Model = s.emb.Model()
	}
	for _, src := range sources {
		st.Sources = append(st.Sources, SourceInfo(src))
		st.Files += src.Files
		st.Chunks += src.Chunks
	}
	st.OutboundConns, st.OutboundNote = countOutbound()
	writeJSON(w, http.StatusOK, st)
}

// QueryRequest is POST /api/query.
type QueryRequest struct {
	Query string `json:"query"`
	Tag   string `json:"tag,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// QueryResult is one hit; Page 0 means the format has no pages.
type QueryResult struct {
	Path    string `json:"path"`
	Page    int    `json:"page,omitempty"`
	Chunk   int    `json:"chunk"`
	Snippet string `json:"snippet"`
	// Text is the chunk's full text, for callers that answer or reason
	// from the result (hay ask via daemon). Display surfaces use Snippet.
	Text  string  `json:"text,omitempty"`
	Score float64 `json:"score"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, errors.New("query is required"))
		return
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 10
	}

	results, err := query.Hybrid(r.Context(), s.st, s.emb, req.Query, req.Tag, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]QueryResult, len(results))
	for i, res := range results {
		out[i] = QueryResult{
			Path: res.Path, Page: res.Page, Chunk: res.Seq,
			Snippet: res.Snippet, Text: res.Text, Score: res.Score,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.st.Sources()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]SourceInfo, len(sources))
	for i, src := range sources {
		out[i] = SourceInfo(src)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

// AddSourceRequest is POST /api/sources. Indexing runs synchronously so
// the caller gets real stats back; the watcher keeps it fresh afterwards.
type AddSourceRequest struct {
	Path string `json:"path"`
	Tag  string `json:"tag,omitempty"`
}

func (s *Server) handleAddSource(w http.ResponseWriter, r *http.Request) {
	var req AddSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	stats, err := ingest.IndexFolder(s.st, req.Path, req.Tag, s.emb, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	abs, err := filepath.Abs(req.Path)
	if err == nil {
		if werr := s.watcher.watchSource(abs); werr != nil {
			fmt.Printf("warning: cannot watch %s: %v\n", abs, werr)
		}
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleRemoveSource(w http.ResponseWriter, r *http.Request) {
	var req AddSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	abs, err := filepath.Abs(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	removed, err := s.st.RemoveSource(abs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.watcher.unwatchSource(abs)
	writeJSON(w, http.StatusOK, map[string]bool{"removed": removed})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
