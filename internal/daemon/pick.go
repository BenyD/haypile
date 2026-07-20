package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// The native file dialog: the daemon is a real process on the user's
// machine, so it can open the OS picker the browser is forbidden to
// offer (web pages never learn absolute paths). The web UI calls this
// and falls back to the in-page browser when the platform cannot help
// (headless boxes, Linux without zenity).

// pickMu serializes dialogs; two at once is never what anyone wants.
var pickMu sync.Mutex

// errPickCanceled means the user closed the dialog without choosing.
var errPickCanceled = errors.New("canceled")

// nativePick is swapped out by tests; opening real dialogs in CI is not
// a thing.
var nativePick = nativePickImpl

func nativePickImpl(ctx context.Context, kind string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		verb := "choose folder with prompt \"Add a folder to Haypile\""
		if kind == "file" {
			verb = "choose file with prompt \"Add a file to Haypile\""
		}
		// Without the activate, the dialog opens behind the browser and
		// unfocused: osascript is not the frontmost app. Activating
		// itself needs no automation permission.
		cmd = exec.CommandContext(ctx, "osascript",
			"-e", "tell me to activate",
			"-e", "POSIX path of ("+verb+")")
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`
		if kind == "file" {
			script = `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.OpenFileDialog; if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
		}
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-STA", "-Command", script)
	default:
		args := []string{"--file-selection", "--title=Add to Haypile"}
		if kind != "file" {
			args = append(args, "--directory")
		}
		if _, err := exec.LookPath("zenity"); err != nil {
			return "", fmt.Errorf("no dialog helper on this platform: %w", err)
		}
		cmd = exec.CommandContext(ctx, "zenity", args...)
	}

	out, err := cmd.Output()
	if err != nil {
		// Every helper reports "user closed the dialog" as a plain
		// non-zero exit with no output.
		if len(strings.TrimSpace(string(out))) == 0 {
			return "", errPickCanceled
		}
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errPickCanceled
	}
	return path, nil
}

// handlePick serves POST /api/pick?kind=folder|file: opens the native
// picker and returns the chosen absolute path. 501 tells the UI to fall
// back to its in-page browser; 204 means the user canceled.
func (s *Server) handlePick(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "folder" && kind != "file" {
		writeError(w, http.StatusBadRequest, errors.New("kind must be folder or file"))
		return
	}
	if !pickMu.TryLock() {
		writeError(w, http.StatusConflict, errors.New("a picker dialog is already open"))
		return
	}
	defer pickMu.Unlock()

	path, err := nativePick(r.Context(), kind)
	switch {
	case errors.Is(err, errPickCanceled):
		w.WriteHeader(http.StatusNoContent)
	case err != nil:
		writeError(w, http.StatusNotImplemented, err)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"path": path})
	}
}
