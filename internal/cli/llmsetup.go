package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/daemon"
	"github.com/BenyD/haypile/internal/llm"
)

// recommendedModel balances answer quality against download size. The 1B
// models hallucinate too readily to be a default; 3B is the floor where
// cited answers hold together.
const recommendedModel = "llama3.2:3b"

func newLLMCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Manage the local LLM that answers hay ask",
	}
	cmd.AddCommand(newLLMSetupCmd())
	return cmd
}

func newLLMSetupCmd() *cobra.Command {
	var model string
	var yes bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Get a local LLM running for hay ask (one guided command)",
		Long: `Checks for a running LLM server, and when there isn't one, walks through
the shortest path to it: install Ollama (with your confirmation), start
it, download a recommended model (with your confirmation — this is the
only large download), and verify with a real question.

Nothing is installed or downloaded without asking first. If you already
run LM Studio, llama.cpp, or Jan, this command just confirms it works.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLLMSetup(cmd, model, yes)
		},
	}

	cmd.Flags().StringVar(&model, "model", recommendedModel, "chat model to download if none is present")
	cmd.Flags().BoolVar(&yes, "yes", false, "answer yes to all prompts (unattended setup)")
	return cmd
}

func runLLMSetup(cmd *cobra.Command, model string, yes bool) error {
	out := cmd.OutOrStdout()
	ctx := cmd.Context()

	// Step 0: maybe everything already works.
	if c, err := llm.Detect(ctx, "", ""); err == nil {
		fmt.Fprintf(out, "Found a running LLM server: %s (model: %s)\n", c.BaseURL, c.Model)
		return verifyLLM(ctx, out, c)
	}
	fmt.Fprintln(out, "No running LLM server found — setting one up with Ollama.")

	// Step 1: ensure the ollama binary exists.
	if _, err := exec.LookPath("ollama"); err != nil {
		if runtime.GOOS == "darwin" {
			if _, err := exec.LookPath("brew"); err == nil {
				if !confirm(cmd, yes, "Install Ollama via Homebrew? (downloads ~30MB)") {
					fmt.Fprintln(out, "Skipped. Install it yourself from https://ollama.com/download and rerun: hay llm setup")
					return nil
				}
				fmt.Fprintln(out, "Running: brew install ollama")
				install := exec.CommandContext(ctx, "brew", "install", "ollama")
				install.Stdout, install.Stderr = out, cmd.ErrOrStderr()
				if err := install.Run(); err != nil {
					return fmt.Errorf("brew install ollama: %w", err)
				}
			} else {
				fmt.Fprintln(out, "Install Ollama from https://ollama.com/download and rerun: hay llm setup")
				return nil
			}
		} else {
			fmt.Fprintln(out, "Install Ollama from https://ollama.com/download (Linux: curl -fsSL https://ollama.com/install.sh | sh) and rerun: hay llm setup")
			return nil
		}
	}

	// Step 2: ensure the server is running.
	if !ollamaUp(ctx) {
		fmt.Fprintln(out, "Starting ollama serve in the background…")
		serve := exec.Command("ollama", "serve")
		daemon.Detach(serve)
		if err := serve.Start(); err != nil {
			return fmt.Errorf("starting ollama: %w", err)
		}
		serve.Process.Release()
		if err := waitFor(ctx, 15*time.Second, func() bool { return ollamaUp(ctx) }); err != nil {
			return fmt.Errorf("ollama did not become ready: %w", err)
		}
	}

	// Step 3: ensure a chat model is present (the one big download).
	if _, err := llm.Detect(ctx, "http://localhost:11434/v1", ""); err != nil {
		if !confirm(cmd, yes, fmt.Sprintf("Download the %s model? (about 2GB, one time)", model)) {
			fmt.Fprintf(out, "Skipped. Pull one yourself (ollama pull %s) and hay ask will find it.\n", model)
			return nil
		}
		fmt.Fprintf(out, "Running: ollama pull %s\n", model)
		pull := exec.CommandContext(ctx, "ollama", "pull", model)
		pull.Stdout, pull.Stderr = out, cmd.ErrOrStderr()
		if err := pull.Run(); err != nil {
			return fmt.Errorf("ollama pull: %w", err)
		}
	}

	// Step 4: prove the whole path with a real round trip.
	c, err := llm.Detect(ctx, "", "")
	if err != nil {
		return fmt.Errorf("setup finished but no endpoint answers: %w", err)
	}
	return verifyLLM(ctx, out, c)
}

func verifyLLM(ctx context.Context, out io.Writer, c *llm.Client) error {
	fmt.Fprint(out, "Verifying with a test question… ")
	reply, err := c.Chat(ctx, "You are a health check. Reply with exactly: OK", "Are you ready?")
	if err != nil {
		return fmt.Errorf("the endpoint answered discovery but not chat: %w", err)
	}
	_ = reply // any completed round trip is a pass; models phrase "OK" freely
	fmt.Fprintf(out, "works.\n\nhay ask is ready (%s · %s). Try:\n", c.BaseURL, c.Model)
	fmt.Fprintln(out, `  hay ask "what do my documents say about …?"`)
	return nil
}

// confirm asks y/N on the command's own streams so tests can script it.
func confirm(cmd *cobra.Command, yes bool, prompt string) bool {
	if yes {
		return true
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", prompt)
	sc := bufio.NewScanner(cmd.InOrStdin())
	if !sc.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(sc.Text()))
	return answer == "y" || answer == "yes"
}

func ollamaUp(ctx context.Context) bool {
	probe, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return llm.Ping(probe, "http://localhost:11434/v1")
}

func waitFor(ctx context.Context, max time.Duration, ready func() bool) error {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if ready() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out after %s", max)
}
