package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <folder>",
		Short: "Un-index a folder and stop watching it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — M0 in progress (see docs/PRD.md §11)")
		},
	}
}
