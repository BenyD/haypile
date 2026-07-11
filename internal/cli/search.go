package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/query"
)

func newSearchCmd() *cobra.Command {
	var tag string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search indexed documents, results with citations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			emb, err := embed.FromEnv()
			if err != nil {
				return err
			}

			st, err := index.Open(index.DefaultPath())
			if err != nil {
				return err
			}
			defer st.Close()

			results, err := query.Hybrid(cmd.Context(), st, emb, args[0], tag, limit)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "No results. Have you indexed a folder? (hay add <folder>)")
				return nil
			}
			for i, r := range results {
				fmt.Fprintf(out, "%2d. %s · chunk %d\n    %s\n", i+1, r.Path, r.Seq+1, r.Snippet)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "restrict search to folders indexed with this tag")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	return cmd
}
