// Package pathnorm canonicalizes filesystem paths at the boundaries where
// they enter the index — CLI arguments, API requests, watcher events — so
// one file is always one row no matter how its path was typed.
//
// Windows filesystems are case-insensitive: C:\Docs and c:\docs name the
// same folder, and the index must treat them as one source. Stored paths
// stay display-faithful (the actual on-disk casing); comparisons fold case
// only where the OS does. Folding happens here in the app layer because
// SQLite's COLLATE NOCASE is ASCII-only.
package pathnorm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CaseInsensitive reports whether this OS compares paths without regard to
// case. It is a variable only so tests can exercise the Windows behavior
// on any platform; production code never writes it.
var CaseInsensitive = runtime.GOOS == "windows"

// Equal reports whether two paths name the same file: byte equality
// everywhere, case-folded equality too on case-insensitive filesystems.
func Equal(a, b string) bool {
	if a == b {
		return true
	}
	return CaseInsensitive && strings.EqualFold(a, b)
}

// HasPrefix reports whether path is root itself or lies under it. The
// prefix must end at a separator boundary, so /data/docs2 is never under
// /data/docs.
func HasPrefix(path, root string) bool {
	if Equal(path, root) {
		return true
	}
	prefix := root + string(filepath.Separator)
	return len(path) >= len(prefix) && Equal(path[:len(prefix)], prefix)
}

// Canon returns the canonical form of path for index storage and lookup:
// absolute and cleaned everywhere, and on case-insensitive filesystems
// with every existing component in its actual on-disk casing (and the
// drive letter uppercased). Components that no longer exist on disk keep
// the casing they were given, so already-deleted paths still canonicalize
// — the store's case-folded lookup catches those.
func Canon(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !CaseInsensitive {
		return abs, nil
	}
	return canonCase(abs), nil
}

// canonCase rebuilds abs component by component, replacing each with its
// on-disk casing. One ReadDir per component; Canon runs only at ingestion
// and lookup boundaries (and watcher events, which are debounced), so the
// cost never sits on a hot path.
func canonCase(abs string) string {
	vol := filepath.VolumeName(abs)
	rest := abs[len(vol):]
	if len(vol) == 2 && vol[1] == ':' {
		vol = strings.ToUpper(vol)
	}
	cur := vol + string(filepath.Separator)
	resolving := true
	for _, comp := range strings.Split(rest, string(filepath.Separator)) {
		if comp == "" {
			continue
		}
		if resolving {
			if name, ok := onDiskName(cur, comp); ok {
				comp = name
			} else {
				// Vanished (or unreadable) from here down: nothing left
				// to resolve against, keep the given casing.
				resolving = false
			}
		}
		cur = filepath.Join(cur, comp)
	}
	return filepath.Clean(cur)
}

// onDiskName returns the entry in dir matching want, preferring an exact
// match over a case-folded one so case-sensitive filesystems holding both
// Readme and README resolve to the one that was asked for.
func onDiskName(dir, want string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	folded := ""
	for _, e := range entries {
		name := e.Name()
		if name == want {
			return name, true
		}
		if folded == "" && strings.EqualFold(name, want) {
			folded = name
		}
	}
	return folded, folded != ""
}
