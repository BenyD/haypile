package daemon

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/BenyD/haypile/internal/ingest"
)

// drivesRoot is the virtual path one level above a Windows drive root.
// Browsing it lists the machine's drives; other platforms have a real
// filesystem root and never see it.
const drivesRoot = "::drives"

func driveListing() (*BrowseResponse, error) {
	if runtime.GOOS != "windows" {
		return nil, errors.New("path must be absolute")
	}
	resp := &BrowseResponse{Path: "Drives", Dirs: []BrowseDir{}, Files: []BrowseDir{}}
	for l := 'A'; l <= 'Z'; l++ {
		root := string(l) + `:\`
		if _, err := os.Stat(root); err == nil {
			resp.Dirs = append(resp.Dirs, BrowseDir{Name: root, Path: root})
		}
	}
	return resp, nil
}

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
	if path == drivesRoot {
		resp, err := driveListing()
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
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
	} else if runtime.GOOS == "windows" {
		// Above a drive root sits the drive list, so the picker can
		// cross from C: to D: without typing a path.
		resp.Parent = drivesRoot
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
