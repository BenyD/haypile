package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// prompter owns the command's stdin for interactive questions. One
// instance per command run: bufio reads ahead, so a second scanner on the
// same reader would find it already drained.
type prompter struct {
	sc  *bufio.Scanner
	out io.Writer
}

func newPrompter(cmd *cobra.Command) *prompter {
	return &prompter{sc: bufio.NewScanner(cmd.InOrStdin()), out: cmd.OutOrStdout()}
}

// line asks for one line; empty input keeps def.
func (p *prompter) line(prompt, def string) string {
	fmt.Fprintf(p.out, "%s ", prompt)
	if !p.sc.Scan() {
		return def
	}
	if s := strings.TrimSpace(p.sc.Text()); s != "" {
		return s
	}
	return def
}

// yesNo asks a y/n question; empty or unrecognized input takes def.
func (p *prompter) yesNo(prompt string, def bool) bool {
	fmt.Fprintf(p.out, "%s ", prompt)
	if !p.sc.Scan() {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(p.sc.Text())) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return def
	}
}
