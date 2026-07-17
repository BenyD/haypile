package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
)

func newServeCmd() *cobra.Command {
	var host string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the daemon (REST API + folder watcher on localhost:11500)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := os.Getenv("HAYPILE_ADDR")
			if addr == "" {
				addr = fmt.Sprintf("%s:%d", host, port)
			}
			if host != "127.0.0.1" && host != "localhost" {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"WARNING: binding %s exposes the API beyond this machine; v1 has no auth.\n", host)
			}
			// A daemon must die cleanly: finish in-flight requests and
			// remove its runtime file on SIGINT/SIGTERM.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return daemon.Run(ctx, addr, version)
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "bind address (non-localhost binds warn loudly: no auth in v1)")
	cmd.Flags().IntVar(&port, "port", 11500, "port for the REST API")
	return cmd
}
