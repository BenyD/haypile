package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
)

func newMCPStdioCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-stdio",
		Short: "MCP stdio transport (proxies to the daemon's /mcp endpoint)",
		Long: `Bridges stdio MCP clients to the daemon's Streamable HTTP endpoint:
newline-delimited JSON-RPC on stdin/stdout, forwarded to /mcp. Configure
clients that launch a process ("command": "hay", "args": ["mcp-stdio"]);
clients that support HTTP can skip this and use http://localhost:11500/mcp.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := daemon.AutoStart()
			if err != nil {
				return err
			}
			if c == nil {
				return errors.New("no daemon available (is HAYPILE_NO_DAEMON set?)")
			}
			return proxyStdio(cmd.InOrStdin(), cmd.OutOrStdout(), c.MCPURL())
		},
	}
}

// proxyStdio forwards each stdin line (one JSON-RPC message) to the HTTP
// endpoint and writes each response as one stdout line. Notifications
// return 202 with no body and produce no output line, per the spec.
func proxyStdio(in io.Reader, out io.Writer, url string) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20) // documents can make big tool results

	w := bufio.NewWriter(out)
	defer w.Flush()

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		resp, err := http.Post(url, "application/json", bytes.NewReader(line))
		if err != nil {
			return fmt.Errorf("daemon unreachable: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusAccepted || len(bytes.TrimSpace(body)) == 0 {
			continue // notification: nothing to write back
		}
		w.Write(bytes.TrimSpace(body))
		w.WriteByte('\n')
		if err := w.Flush(); err != nil {
			return err
		}
	}
	err := sc.Err()
	if err != nil && strings.Contains(err.Error(), "token too long") {
		fmt.Fprintln(os.Stderr, "mcp-stdio: message exceeded 16MB limit")
	}
	return err
}
