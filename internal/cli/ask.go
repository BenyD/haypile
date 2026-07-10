package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newAskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask <question>",
		Short: "Answer a question from your documents, with cited sources (requires Ollama)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet — planned for M4 (see docs/PRD.md §11)")
		},
	}
}
