// Package cli defines the hay command-line interface. Commands are thin:
// they parse flags and delegate to the engine packages under internal/.
package cli

import (
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X ...cli.version=v1.2.3".
var version = "dev"

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
		newMCPStdioCmd(),
		newLLMCmd(),
	)
	return root
}

// Execute runs the CLI. It is the only entry point main() needs.
func Execute() error {
	return newRootCmd().Execute()
}
