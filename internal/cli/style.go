package cli

import (
	"fmt"
	"io"
	"os"
)

// The CLI's entire design system: a yellow "!" for things that need
// attention and dim continuation lines for how to fix them. Plain text
// everywhere color is unwelcome (pipes, NO_COLOR, dumb terminals), so
// tests and scripts see stable output.

// isTerminal reports whether out is an interactive terminal, the only
// place cursor tricks (a rewritten progress line) belong.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func colorEnabled(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(out)
}

// warnf prints one attention line, prefixed "!".
func warnf(out io.Writer, format string, a ...any) {
	if colorEnabled(out) {
		fmt.Fprintf(out, "\033[33m! "+format+"\033[0m\n", a...)
		return
	}
	fmt.Fprintf(out, "! "+format+"\n", a...)
}

// hintf prints the fix for the warning above it, indented and dimmed.
func hintf(out io.Writer, format string, a ...any) {
	if colorEnabled(out) {
		fmt.Fprintf(out, "\033[2m  "+format+"\033[0m\n", a...)
		return
	}
	fmt.Fprintf(out, "  "+format+"\n", a...)
}
