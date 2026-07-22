package cli

import (
	"fmt"
	"io"
	"time"

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
			// Suggesting a search right after nothing became searchable
			// would point at the void.
			if stats.Chunks > 0 || stats.Skipped > 0 {
				fmt.Fprintln(out, `Try: hay search "something you remember"`)
			}
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
		if progress {
			defer liveProgress(cmd, c, path)()
		}
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
		fmt.Fprintf(out, "Indexing %s…\n", path)
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

// liveProgress keeps a folder-sized index pass from looking hung. The
// daemon owns the work and answers one blocking request, so its own
// growing counts are the only honest signal: poll them onto a single
// rewritten line. Returns the stop function, which clears the line so
// the summary lands on a clean row. Interactive terminals only; pipes
// and CI logs get the one plain line and nothing else.
func liveProgress(cmd *cobra.Command, c *daemon.Client, path string) func() {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Indexing %s…\n", path)
	if !isTerminal(out) {
		return func() {}
	}

	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(700 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				fmt.Fprint(out, "\r\033[K")
				return
			case <-tick.C:
				s, err := c.Status()
				if err != nil {
					continue
				}
				fmt.Fprintf(out, "\r\033[K  %d files, %d chunks so far…", s.Files, s.Chunks)
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

func printIndexStats(out io.Writer, stats ingest.Stats, model string) {
	fmt.Fprintf(out, "Indexed %d %s (%d %s), %d unchanged.\n",
		stats.Indexed, plural(stats.Indexed, "file", "files"),
		stats.Chunks, plural(stats.Chunks, "chunk", "chunks"), stats.Skipped)
	if stats.Failed > 0 {
		warnf(out, "%d %s could not be read and %s skipped.",
			stats.Failed, plural(stats.Failed, "file", "files"), plural(stats.Failed, "was", "were"))
	}
	// "Embedded 0 chunks" is noise when nothing new was indexed.
	if model != "" && stats.Chunks > 0 {
		fmt.Fprintf(out, "Embedded %d %s for semantic search (%s).\n",
			stats.Embedded, plural(stats.Embedded, "chunk", "chunks"), model)
	}
	if stats.ScanSkipped > 0 {
		warnf(out, "%d scanned %s indexed empty: no vision model is running.",
			stats.ScanSkipped, plural(stats.ScanSkipped, "page", "pages"))
		hintf(out, "hay llm setup installs one; re-add this folder after.")
	}
	if stats.ScanFailed > 0 {
		warnf(out, "%d scanned %s indexed empty: the vision model errored or read nothing.",
			stats.ScanFailed, plural(stats.ScanFailed, "page", "pages"))
		hintf(out, "re-add this folder to retry; a slow first load usually works the second time")
	}
}

// plural picks the wording that matches the count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
