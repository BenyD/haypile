package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
	"github.com/BenyD/haypile/internal/llm"
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
	// Daemonless indexing still gets scanned-page OCR when a local LLM
	// is up; the hook only probes if a textless PDF page shows up.
	ingest.SetOCR(llm.OCRHook())
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
	fmt.Fprintf(out, "Indexed %d %s (%d %s), %d unchanged.\n",
		stats.Indexed, plural(stats.Indexed, "file", "files"),
		stats.Chunks, plural(stats.Chunks, "chunk", "chunks"), stats.Skipped)
	if stats.Failed > 0 {
		fmt.Fprintf(out, "Warning: %d %s could not be read and %s skipped.\n",
			stats.Failed, plural(stats.Failed, "file", "files"), plural(stats.Failed, "was", "were"))
	}
	if model != "" {
		fmt.Fprintf(out, "Embedded %d chunks for semantic search (%s).\n", stats.Embedded, model)
	}
	if stats.ScanSkipped > 0 {
		fmt.Fprintf(out, "%d %s scanned (image only) and indexed empty: no vision model is running to read %s.\n",
			stats.ScanSkipped, plural(stats.ScanSkipped, "page looks", "pages look"), plural(stats.ScanSkipped, "it", "them"))
		fmt.Fprintln(out, "To make scans searchable: hay llm setup installs one (llava), then re-add this folder.")
	}
}

// plural picks the wording that matches the count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
