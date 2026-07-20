package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
)

func newWebCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Open the local web UI (search, ask, read citations in your browser)",
		Long: `Starts the daemon if it is not already running and opens the bundled
web UI. Everything is served from localhost by the same daemon that
powers the CLI; nothing leaves your machine.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := daemon.AutoStart()
			if err != nil {
				return err
			}
			if c == nil {
				return errors.New("no daemon available (is HAYPILE_NO_DAEMON set?)")
			}

			url := c.BaseURL()
			fmt.Fprintf(cmd.OutOrStdout(), "Web UI: %s\n", url)
			if !noBrowser {
				if err := openBrowser(url); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Could not open a browser (%v); open the URL yourself.\n", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the URL without opening a browser")
	return cmd
}

// openBrowser asks the OS to open url with its default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
