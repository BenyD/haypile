package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Daemon status, model info, and outbound connection count (target: 0)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — planned for M3 (see docs/PRD.md §11)")
		},
	}
}
