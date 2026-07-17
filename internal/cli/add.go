package cli

import (
	"fmt"
	"io"

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

			stats, model, err := indexSource(cmd, args[0], tag, true)
			if err != nil {
				return err
			}
			printIndexStats(out, stats, model)
			fmt.Fprintln(out, `Try: hay search "something you remember"`)
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "tag documents in this folder for filtered search")
	return cmd
}

// indexSource indexes a folder or file. It auto-starts the daemon and
// routes through it so the source is watched; with no daemon available
// (HAYPILE_NO_DAEMON=1 or failed start) it indexes directly — everything
// works, it just isn't watched until a daemon runs. Returns the stats and
// the embedding model used, if any.
func indexSource(cmd *cobra.Command, path, tag string, progress bool) (ingest.Stats, string, error) {
	var stats ingest.Stats
	var model string

	if c, err := daemon.AutoStart(); err != nil {
		return stats, "", err
	} else if c != nil {
		if stats, err = c.AddSource(path, tag); err != nil {
			return stats, "", err
		}
		if h, err := c.Status(); err == nil {
			model = h.Model
		}
		return stats, model, nil
	}

	emb, err := embed.FromEnv()
	if err != nil {
		return stats, "", err
	}
	st, err := index.Open(index.DefaultPath())
	if err != nil {
		return stats, "", err
	}
	defer st.Close()

	var report func(string)
	if progress {
		out := cmd.OutOrStdout()
		report = func(p string) { fmt.Fprintf(out, "  %s\n", p) }
	}
	if stats, err = ingest.IndexFolder(st, path, tag, emb, report); err != nil {
		return stats, "", err
	}
	if emb != nil {
		model = emb.Model()
	}
	return stats, model, nil
}

func printIndexStats(out io.Writer, stats ingest.Stats, model string) {
	fmt.Fprintf(out, "Indexed %d files (%d chunks), %d unchanged.\n",
		stats.Indexed, stats.Chunks, stats.Skipped)
	if stats.Failed > 0 {
		fmt.Fprintf(out, "Warning: %d files could not be read and were skipped.\n", stats.Failed)
	}
	if model != "" {
		fmt.Fprintf(out, "Embedded %d chunks for semantic search (%s).\n", stats.Embedded, model)
	}
}
