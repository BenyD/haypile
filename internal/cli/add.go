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

			stats, model, err := indexSource(cmd, expandHome(args[0]), tag, true)
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

	var report func(ingest.Progress)
	var clearLine func()
	if progress {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Indexing %s…\n", path)
		if canRewrite(out) {
			// One rewritten line with a fraction and an ETA; redraws
			// are throttled so tiny files do not flood the terminal.
			var eta etaTracker
			var lastDraw time.Time
			report = func(p ingest.Progress) {
				now := time.Now()
				if now.Sub(lastDraw) < 100*time.Millisecond && p.FilesDone != p.FilesTotal {
					return
				}
				lastDraw = now
				fmt.Fprintf(out, "\r\033[K  %s", progressLine(&eta, p, now))
			}
			clearLine = func() { fmt.Fprint(out, "\r\033[K") }
		} else {
			// Pipes and CI logs get one plain line per file, as before,
			// plus a tenth-of-the-way line through embedding — that phase
			// names no files, so it used to print nothing at all.
			var ms milestoneReporter
			report = func(p ingest.Progress) {
				if p.Phase == ingest.PhaseEmbedding {
					if line := ms.step(&p); line != "" {
						fmt.Fprintln(out, line)
					}
					return
				}
				if p.Path != "" {
					fmt.Fprintf(out, "  %s\n", p.Path)
				}
			}
		}
	}
	stats, err = ingest.IndexFolder(st, path, tag, emb, report)
	if clearLine != nil {
		clearLine()
	}
	if err != nil {
		return stats, "", err
	}
	if emb != nil {
		model = emb.Model()
	}
	return stats, model, nil
}

// liveProgress keeps a folder-sized index pass from looking hung. The
// daemon owns the work and answers one blocking request, so its own
// growing counts are the only honest signal: poll them. A terminal gets
// a single rewritten line with a fraction and an ETA; pipes, CI logs and
// backgrounded runs cannot rewrite, so they get one line per tenth of a
// phase instead of the silence they used to get. Returns the stop
// function, which clears the line so the summary lands on a clean row.
func liveProgress(cmd *cobra.Command, c *daemon.Client, path string) func() {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Indexing %s…\n", path)
	tty := canRewrite(out)

	// Redraws are cheap on a terminal and worth doing often. Milestones
	// fire at most ten times a phase, so polling that fast buys nothing.
	interval := 700 * time.Millisecond
	if !tty {
		interval = 2 * time.Second
	}

	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		var eta etaTracker
		var ms milestoneReporter
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-done:
				if tty {
					fmt.Fprint(out, "\r\033[K")
				}
				return
			case <-tick.C:
				s, err := c.Status()
				if err != nil {
					continue
				}
				if !tty {
					if line := ms.step(s.Indexing); line != "" {
						fmt.Fprintln(out, line)
					}
					continue
				}
				if s.Indexing != nil {
					fmt.Fprintf(out, "\r\033[K  %s", progressLine(&eta, *s.Indexing, time.Now()))
					continue
				}
				// The pass has not registered yet (or an old daemon is
				// answering): global counts are still an honest pulse.
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
