package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background daemon",
		Long: "Stops the background daemon gracefully. Folder watching pauses until " +
			"the next command starts it again; search keeps working through direct " +
			"index access in the meantime.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			c := daemon.Discover()
			if c == nil {
				fmt.Fprintln(out, "Daemon is not running.")
				return nil
			}
			if err := c.Shutdown(); err != nil {
				return fmt.Errorf("asking the daemon to stop: %w", err)
			}
			// The daemon finishes in-flight requests before exiting; wait
			// so "stopped" means stopped, not stopping.
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if daemon.Discover() == nil {
					fmt.Fprintln(out, "Daemon stopped.")
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			return fmt.Errorf("daemon is still running after 10s; it may be mid-request")
		},
	}
}
