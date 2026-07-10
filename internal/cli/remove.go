package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/index"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <folder>",
		Short: "Un-index a folder and stop watching it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}

			st, err := index.Open(index.DefaultPath())
			if err != nil {
				return err
			}
			defer st.Close()

			found, err := st.RemoveSource(abs)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("%s is not indexed (see: hay list)", abs)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from the index.\n", abs)
			return nil
		},
	}
}
