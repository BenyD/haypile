// Package cli defines the hay command-line interface. Commands are thin:
// they parse flags and delegate to the engine packages under internal/.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
)

// version is set at build time via -ldflags "-X ...cli.version=v1.2.3".
var version = "dev"

func init() {
	// Lets daemon discovery retire a daemon left over from before an
	// upgrade instead of quietly using its old code.
	daemon.CurrentVersion = version
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "hay",
		Short:   "Local, private search and Q&A over your documents",
		Long:    "Haypile watches your folders, indexes every document, and answers questions about them. Fully local, fully private.",
		Version: version,
		// Silence cobra's own error printing; main.go handles errors once.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newSearchCmd(),
		newAskCmd(),
		newListCmd(),
		newRemoveCmd(),
		newStatusCmd(),
		newServeCmd(),
		newWebCmd(),
		newMCPStdioCmd(),
		newLLMCmd(),
	)
	return root
}

// Execute runs the CLI. It is the only entry point main() needs.
func Execute() error {
	return newRootCmd().Execute()
}
