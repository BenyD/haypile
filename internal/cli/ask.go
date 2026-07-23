package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/llm"
	"github.com/BenyD/haypile/internal/query"
)

func newAskCmd() *cobra.Command {
	var endpoint, model, tag string
	var limit int

	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Answer a question from your documents, with cited sources (uses your local LLM)",
		Long: `Retrieves the most relevant passages and asks a local LLM to answer from
them, citing sources. Generation needs any OpenAI-compatible server you
already run (Ollama, LM Studio, llama.cpp, Jan, ...) — auto-detected on
common ports. Haypile itself never talks to the network.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			question := args[0]

			// Retrieval first: it works with or without an LLM.
			var results []index.Result
			if c := daemon.Discover(); c != nil {
				var err error
				if results, err = c.Query(question, tag, limit); err != nil {
					return err
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
				if results, err = query.HybridForAnswer(cmd.Context(), st, emb, question, tag, limit); err != nil {
					return err
				}
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "Nothing relevant indexed. Have you added a folder? (hay add <folder>)")
				return nil
			}

			client, err := llm.Detect(cmd.Context(), endpoint, model)
			if err != nil {
				// No LLM is a degraded mode, not a failure: show what
				// search found and how to enable answers.
				if errors.Is(err, llm.ErrNoEndpoint) {
					warnf(out, "no local LLM endpoint found (tried Ollama :11434, LM Studio :1234, llama.cpp :8080, Jan :1337).")
					hintf(out, "hay llm setup gets one going; or point hay at yours: hay ask --endpoint http://localhost:PORT/v1")
				} else {
					warnf(out, "%v", err)
				}
				fmt.Fprint(out, "\nTop passages for your question meanwhile:\n\n")
				for i, r := range results {
					fmt.Fprintf(out, "%2d. %s\n    %s\n", i+1, citation(r), oneLine(r.Snippet))
				}
				return nil
			}

			fmt.Fprintf(out, "Answering with %s (%s)…\n\n", client.Model, client.BaseURL)
			answer, err := llm.Answer(cmd.Context(), client, question, results)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, answer)
			fmt.Fprintln(out, "\nSources:")
			for i, r := range results {
				fmt.Fprintf(out, "  [%d] %s\n", i+1, citation(r))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "OpenAI-compatible base URL (default: auto-detect local servers)")
	cmd.Flags().StringVar(&model, "model", "", "model name to request (default: first chat model the endpoint lists)")
	cmd.Flags().StringVar(&tag, "tag", "", "restrict retrieval to folders indexed with this tag")
	cmd.Flags().IntVar(&limit, "limit", 6, "how many passages to retrieve as context")
	return cmd
}
