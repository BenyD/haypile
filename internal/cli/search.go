package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
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
			// A running daemon answers faster (model already loaded) and
			// fresher (watcher applied); otherwise search directly.
			var results []index.Result
			if c := daemon.Discover(); c != nil {
				var err error
				if results, err = c.Query(args[0], tag, limit); err != nil {
					return err
				}
			} else {
				emb, err := embed.FromEnv()
				if err != nil {
					return err
				}
				st, err := index.Open(index.DefaultPath())
				if err != nil {
					return err
				}
				defer st.Close()
				if results, err = query.Hybrid(cmd.Context(), st, emb, args[0], tag, limit); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "No results. Have you indexed a folder? (hay add <folder>)")
				return nil
			}
			for i, r := range results {
				fmt.Fprintf(out, "%2d. %s\n    %s\n", i+1, citation(r), r.Snippet)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "restrict search to folders indexed with this tag")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	return cmd
}

// citation renders "where this came from": file plus page for paginated
// formats, file plus chunk position otherwise. Citations are non-negotiable
// output — every result must be traceable to its source.
func citation(r index.Result) string {
	if r.Page > 0 {
		return fmt.Sprintf("%s · page %d", r.Path, r.Page)
	}
	return fmt.Sprintf("%s · chunk %d", r.Path, r.Seq+1)
}
