package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
	"github.com/BenyD/haypile/internal/index"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Daemon status, model info, and outbound connection count (target: 0)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			c := daemon.Discover()
			if c == nil {
				// No daemon: report on the index directly so status always
				// answers something useful.
				st, err := index.Open(index.DefaultPath())
				if err != nil {
					return err
				}
				defer st.Close()
				sources, err := st.Sources()
				if err != nil {
					return err
				}
				files, chunks := 0, 0
				for _, s := range sources {
					files += s.Files
					chunks += s.Chunks
				}
				fmt.Fprintln(out, "Daemon:   not running (starts automatically on hay add)")
				fmt.Fprintf(out, "Index:    %s\n", index.DefaultPath())
				fmt.Fprintf(out, "Indexed:  %d sources, %d files, %d chunks\n", len(sources), files, chunks)
				return nil
			}

			s, err := c.Status()
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Daemon:   running (%s, up %ds)\n", s.Version, s.UptimeSeconds)
			fmt.Fprintf(out, "Index:    %s\n", s.DB)
			fmt.Fprintf(out, "Indexed:  %d sources, %d files, %d chunks\n", len(s.Sources), s.Files, s.Chunks)
			if s.Model != "" {
				fmt.Fprintf(out, "Model:    %s\n", s.Model)
			} else {
				fmt.Fprintln(out, "Model:    none (keyword-only mode)")
			}
			if s.PendingJobs > 0 {
				fmt.Fprintf(out, "Indexing: %d changes queued\n", s.PendingJobs)
			}
			if s.OutboundNote != "" {
				fmt.Fprintf(out, "Outbound connections: %s\n", s.OutboundNote)
			} else {
				fmt.Fprintf(out, "Outbound connections: %d\n", s.OutboundConns)
			}
			return nil
		},
	}
}
