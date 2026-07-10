package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search indexed documents, results with citations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — M0 in progress (see docs/PRD.md §11)")
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "restrict search to folders indexed with this tag")
	return cmd
}
