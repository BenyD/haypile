package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome resolves a leading ~ in a user-supplied path. Shells
// usually do this first, but not always: cmd.exe never expands ~, and
// any shell passes it through when quoted. Without this, filepath.Abs
// turns "~/Documents" into a literal "./~/Documents" and the error
// blames a folder the user never named.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
