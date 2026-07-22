package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BenyD/haypile/internal/ingest"
	"github.com/BenyD/haypile/internal/llm"
)

func newInitCmd() *cobra.Command {
	var tag string
	var excludes []string
	var mcp bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "init [folder]",
		Short: "Set up this folder for Haypile: config, indexing, editor integration",
		Long: `Writes a .haypile.yml (tag + exclude patterns), indexes the folder, and
optionally wires it up for MCP clients (.mcp.json for Claude Code) and a
local LLM. Interactive by default; every question has a flag so
--yes runs unattended. Editing .haypile.yml later re-syncs the index
automatically while the daemon runs.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			folder := "."
			if len(args) == 1 {
				folder = args[0]
			}
			return runInit(cmd, folder, tag, excludes, mcp, cmd.Flags().Changed("mcp"), yes)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "tag for filtered search (default: folder name)")
	cmd.Flags().StringSliceVar(&excludes, "exclude", nil, `exclude patterns, e.g. --exclude "drafts/**,*.bak"`)
	cmd.Flags().BoolVar(&mcp, "mcp", true, "write .mcp.json so Claude Code/Cursor can search these docs")
	cmd.Flags().BoolVar(&yes, "yes", false, "accept all defaults (unattended)")
	return cmd
}

func runInit(cmd *cobra.Command, folder, tag string, excludes []string, mcp, mcpSet, yes bool) error {
	out := cmd.OutOrStdout()

	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder. hay init configures folders (hay add handles single files)", abs)
	}

	// An existing config seeds the defaults, so rerunning init is an edit,
	// not a reset.
	existing, err := ingest.LoadConfig(abs)
	if err != nil {
		return err
	}
	if tag == "" {
		tag = existing.Tag
	}
	if tag == "" {
		tag = filepath.Base(abs)
	}
	if excludes == nil {
		excludes = existing.Exclude
	}

	fmt.Fprintf(out, "Setting up %s\n", abs)
	interactive := !yes
	p := newPrompter(cmd)
	if interactive {
		tag = p.line(fmt.Sprintf("Tag for filtered search [%s]:", tag), tag)
		def := strings.Join(excludes, ", ")
		if def == "" {
			def = "none"
		}
		raw := p.line(fmt.Sprintf("Exclude patterns, comma-separated [%s]:", def), strings.Join(excludes, ", "))
		excludes = nil
		for _, pat := range strings.Split(raw, ",") {
			if pat = strings.TrimSpace(pat); pat != "" {
				excludes = append(excludes, pat)
			}
		}
	}

	cfg := ingest.Config{Tag: tag, Exclude: excludes}
	if err := cfg.Save(abs); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s\n", filepath.Join(abs, ingest.ConfigName))

	// MCP wiring: a project-level .mcp.json is how Claude Code (and
	// compatible editors) discover per-folder servers.
	writeMCP := mcp
	if interactive && !mcpSet {
		writeMCP = p.yesNo("Make these docs available to AI tools here (Claude Code, Cursor)? [Y/n]", true)
	}
	if writeMCP {
		if err := writeMCPConfig(out, abs); err != nil {
			warnf(out, "could not write .mcp.json: %v", err)
		}
	}

	// Index (and watch, via the daemon) with the configured tag.
	stats, model, err := indexSource(cmd, abs, tag, true)
	if err != nil {
		return err
	}
	printIndexStats(out, stats, model)

	// Offer the LLM path only when it's actually missing — and never
	// auto-download under --yes; hay llm setup owns those consents.
	if interactive {
		if _, err := llm.Detect(cmd.Context(), "", ""); err != nil {
			if p.yesNo("Set up a local LLM for `hay ask`? Search works without it. [y/N]", false) {
				if err := runLLMSetup(cmd, p, recommendedModel, false); err != nil {
					fmt.Fprintf(out, "LLM setup did not finish: %v\nRerun anytime: hay llm setup\n", err)
				}
			}
		}
	}

	fmt.Fprintf(out, "\nDone. Try: hay search \"something in %s\"\n", filepath.Base(abs))
	return nil
}

// writeMCPConfig adds a haypile entry to the folder's .mcp.json, creating
// the file if needed and leaving existing servers untouched.
func writeMCPConfig(out interface{ Write([]byte) (int, error) }, dir string) error {
	path := filepath.Join(dir, ".mcp.json")
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("existing .mcp.json is not valid JSON, leaving it alone: %w", err)
		}
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers["haypile"]; exists {
		fmt.Fprintf(out, ".mcp.json already lists haypile, left as is\n")
		return nil
	}
	servers["haypile"] = map[string]any{
		"type": "http",
		"url":  "http://localhost:11500/mcp",
	}
	doc["mcpServers"] = servers

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s (Claude Code will pick it up in this folder)\n", path)
	return nil
}
