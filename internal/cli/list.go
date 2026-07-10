package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show indexed folders, document counts, and index health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — M0 in progress (see docs/PRD.md §11)")
		},
	}
}
