package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/index"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show indexed folders, document counts, and index health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := index.Open(index.DefaultPath())
			if err != nil {
				return err
			}
			defer st.Close()

			sources, err := st.Sources()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(sources) == 0 {
				fmt.Fprintln(out, "Nothing indexed yet. Start with: hay add <folder>")
				return nil
			}

			w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "FOLDER\tTAG\tFILES\tCHUNKS")
			for _, s := range sources {
				fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", s.Path, s.Tag, s.Files, s.Chunks)
			}
			return w.Flush()
		},
	}
}
