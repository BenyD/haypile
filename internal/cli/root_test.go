package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestRootRegistersAllCommands(t *testing.T) {
	want := []string{"add", "search", "ask", "list", "remove", "status", "serve"}

	root := newRootCmd()
	got := make(map[string]bool)
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Errorf("command %q is not registered on root", name)
		}
	}
}

func TestHelpRunsWithoutError(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(out, "hay") {
		t.Errorf("help output does not mention the binary name; got:\n%s", out)
	}
}

func TestUnimplementedCommandsFailLoudly(t *testing.T) {
	// Until their milestones land, stubs must return an error rather than
	// silently succeed — a silent no-op would look like data loss to a user.
	for _, args := range [][]string{
		{"ask", "anything"},
		{"status"},
		{"serve"},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: expected not-implemented error, got nil", args)
		}
	}
}

func TestAddSearchListRemoveEndToEnd(t *testing.T) {
	t.Setenv("HAYPILE_DIR", t.TempDir())

	docs := t.TempDir()
	err := os.WriteFile(filepath.Join(docs, "contract.md"),
		[]byte("# Services Agreement\n\nEither party may terminate with sixty days written notice.\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "add", docs)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Indexed 1 files (1 chunks)") {
		t.Errorf("add output unexpected:\n%s", out)
	}

	out, err = run(t, "search", "terminate notice")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "contract.md") {
		t.Errorf("search did not cite contract.md:\n%s", out)
	}

	// Re-adding without changes must skip, not re-index.
	out, _ = run(t, "add", docs)
	if !strings.Contains(out, "1 unchanged") {
		t.Errorf("re-add did not skip unchanged file:\n%s", out)
	}

	out, err = run(t, "list")
	if err != nil || !strings.Contains(out, docs) {
		t.Errorf("list missing source (err=%v):\n%s", err, out)
	}

	if out, err = run(t, "remove", docs); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	out, _ = run(t, "search", "terminate notice")
	if strings.Contains(out, "contract.md") {
		t.Errorf("removed folder still searchable:\n%s", out)
	}
}
