package daemon

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BenyD/haypile/internal/ingest"
)

// BrowseDir is one directory entry in a browse listing.
type BrowseDir struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BrowseResponse is GET /api/browse: the source picker's view of one
// directory. Files are listed only when Haypile can index them, so the
// picker never offers something `hay add` would reject.
type BrowseResponse struct {
	Path   string      `json:"path"`
	Parent string      `json:"parent,omitempty"`
	Dirs   []BrowseDir `json:"dirs"`
	Files  []BrowseDir `json:"files"`
}

// handleBrowse lists the subdirectories of a path so the web UI can offer
// a folder picker. Browsers cannot reveal real filesystem paths, so the
// daemon, which is the user's own local process, does the walking. Served
// localhost-only behind the host and origin guards like everything else.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		path = home
	}
	if !filepath.IsAbs(path) {
		writeError(w, http.StatusBadRequest, errors.New("path must be absolute"))
		return
	}
	path = filepath.Clean(path)

	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	resp := BrowseResponse{Path: path, Dirs: []BrowseDir{}, Files: []BrowseDir{}}
	if parent := filepath.Dir(path); parent != path {
		resp.Parent = parent
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		entry := BrowseDir{Name: e.Name(), Path: filepath.Join(path, e.Name())}
		switch {
		case e.IsDir():
			resp.Dirs = append(resp.Dirs, entry)
		case ingest.Supported(e.Name()):
			resp.Files = append(resp.Files, entry)
		}
	}
	byName := func(list []BrowseDir) func(i, j int) bool {
		return func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		}
	}
	sort.Slice(resp.Dirs, byName(resp.Dirs))
	sort.Slice(resp.Files, byName(resp.Files))
	writeJSON(w, http.StatusOK, resp)
}
