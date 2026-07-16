package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
)

func newAddCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "add <folder>",
		Short: "Index a folder and watch it for changes",
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

			out := cmd.OutOrStdout()
			stats, err := ingest.IndexFolder(st, args[0], tag, emb, func(path string) {
				fmt.Fprintf(out, "  %s\n", path)
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Indexed %d files (%d chunks), %d unchanged.\n",
				stats.Indexed, stats.Chunks, stats.Skipped)
			if stats.Failed > 0 {
				fmt.Fprintf(out, "Warning: %d files could not be read and were skipped.\n", stats.Failed)
			}
			if emb != nil {
				fmt.Fprintf(out, "Embedded %d chunks for semantic search (%s).\n",
					stats.Embedded, emb.Model())
			}
			fmt.Fprintln(out, `Try: hay search "something you remember"`)
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "tag documents in this folder for filtered search")
	return cmd
}
