package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
)

func newAddCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "add <folder-or-file>",
		Short: "Index a folder (or a single document) and watch it for changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Adding a folder is the moment watching should begin, so this
			// command auto-starts the daemon and routes through it. With no
			// daemon (HAYPILE_NO_DAEMON=1 or failed start), indexing still
			// works directly — it just isn't watched until one runs.
			var stats ingest.Stats
			var model string
			if c, err := daemon.AutoStart(); err != nil {
				return err
			} else if c != nil {
				if stats, err = c.AddSource(args[0], tag); err != nil {
					return err
				}
				if h, err := c.Status(); err == nil {
					model = h.Model
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
				if stats, err = ingest.IndexFolder(st, args[0], tag, emb, func(path string) {
					fmt.Fprintf(out, "  %s\n", path)
				}); err != nil {
					return err
				}
				if emb != nil {
					model = emb.Model()
				}
			}

			fmt.Fprintf(out, "Indexed %d files (%d chunks), %d unchanged.\n",
				stats.Indexed, stats.Chunks, stats.Skipped)
			if stats.Failed > 0 {
				fmt.Fprintf(out, "Warning: %d files could not be read and were skipped.\n", stats.Failed)
			}
			if model != "" {
				fmt.Fprintf(out, "Embedded %d chunks for semantic search (%s).\n",
					stats.Embedded, model)
			}
			fmt.Fprintln(out, `Try: hay search "something you remember"`)
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "tag documents in this folder for filtered search")
	return cmd
}
