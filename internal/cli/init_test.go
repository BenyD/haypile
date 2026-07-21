package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestInitUnattended(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())
	docs := t.TempDir()
	os.MkdirAll(filepath.Join(docs, "drafts"), 0o755)
	os.WriteFile(filepath.Join(docs, "final.md"), []byte("the walrus accord is signed"), 0o644)
	os.WriteFile(filepath.Join(docs, "drafts", "wip.md"), []byte("walrus accord draft"), 0o644)

	out, err := run(t, "init", docs, "--yes", "--tag", "deals", "--exclude", "drafts/**")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	for _, want := range []string{".haypile.yml", ".mcp.json", "Indexed 1 file"} {
		if !strings.Contains(out, want) {
			t.Errorf("init output missing %q:\n%s", want, out)
		}
	}

	// Config landed with the right contents.
	cfgData, err := os.ReadFile(filepath.Join(docs, ".haypile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfgData), "tag: deals") || !strings.Contains(string(cfgData), "drafts/**") {
		t.Errorf("config contents unexpected:\n%s", cfgData)
	}

	// .mcp.json points Claude Code at the daemon.
	var mcp struct {
		Servers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	mcpData, err := os.ReadFile(filepath.Join(docs, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatal(err)
	}
	if mcp.Servers["haypile"].URL != "http://localhost:11500/mcp" {
		t.Errorf(".mcp.json unexpected: %s", mcpData)
	}

	// The excluded draft is not searchable; the tag is live.
	if out, _ := run(t, "search", "walrus accord", "--tag", "deals"); !strings.Contains(out, "final.md") ||
		strings.Contains(out, "wip.md") {
		t.Errorf("search after init unexpected:\n%s", out)
	}
}

func TestInitInteractiveDefaults(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())
	docs := filepath.Join(t.TempDir(), "acme-cases")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "a.md"), []byte("meerkat memo"), 0o644)

	// Enter, Enter (accept defaults), n (no mcp), then llm prompt may not
	// appear (skipped when an endpoint is detected). Feed a trailing n.
	out, err := runWithInput(t, "\n\nn\nn\n", "init", docs)
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	cfg, err := os.ReadFile(filepath.Join(docs, ".haypile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "tag: acme-cases") {
		t.Errorf("default tag should be the folder name:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(docs, ".mcp.json")); err == nil {
		t.Error("answered n to MCP but .mcp.json was written")
	}
}

func TestInitPreservesExistingMCPServers(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())
	docs := t.TempDir()
	os.WriteFile(filepath.Join(docs, "a.md"), []byte("ibex invoice"), 0o644)
	existing := `{"mcpServers": {"other": {"type": "stdio", "command": "other-tool"}}}`
	os.WriteFile(filepath.Join(docs, ".mcp.json"), []byte(existing), 0o644)

	if out, err := run(t, "init", docs, "--yes"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	data, _ := os.ReadFile(filepath.Join(docs, ".mcp.json"))
	if !strings.Contains(string(data), "other-tool") || !strings.Contains(string(data), "haypile") {
		t.Errorf("merge lost a server:\n%s", data)
	}
}

func TestInitRejectsFile(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())
	f := filepath.Join(t.TempDir(), "doc.md")
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := run(t, "init", f, "--yes"); err == nil {
		t.Fatal("init on a file must error and point at hay add")
	}
}
