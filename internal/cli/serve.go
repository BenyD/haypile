package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the daemon (REST API + MCP on localhost:11500)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — planned for M3 (see docs/PRD.md §11)")
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "bind address (non-localhost binds warn loudly: no auth in v1)")
	return cmd
}
