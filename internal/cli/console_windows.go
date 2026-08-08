//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

// setupConsole prepares the Windows console for this CLI's output and
// returns a restore function for process exit.
//
// Two gaps between what the code emits and what a stock console renders:
// ANSI escapes (the rewritten progress line, warning colors) need
// virtual terminal processing, which Windows Terminal enables but legacy
// conhost does not; and the progress bar's non-ASCII glyphs need the
// output code page to be UTF-8, which no console defaults to. Both are
// per-console settings, so they are enabled here and ansiOK records
// whether escapes will be interpreted — when they will not (a conhost
// too old for VT), the CLI falls back to the plain output pipes get.
func setupConsole() func() {
	vt := false
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		h := windows.Handle(f.Fd())
		var mode uint32
		if windows.GetConsoleMode(h, &mode) != nil {
			continue // not a console (pipe, file); nothing to enable
		}
		if windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil {
			vt = true
		}
	}
	ansiOK = vt

	// The code page outlives the process in the user's console window, so
	// put it back on the way out.
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getCP := kernel32.NewProc("GetConsoleOutputCP")
	setCP := kernel32.NewProc("SetConsoleOutputCP")
	const utf8CP = 65001
	prev, _, _ := getCP.Call()
	if prev == 0 || prev == utf8CP {
		return func() {}
	}
	if ok, _, _ := setCP.Call(utf8CP); ok == 0 {
		return func() {}
	}
	return func() { setCP.Call(prev) }
}
