package cli

import (
	"bytes"
	"strings"
	"testing"
)

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
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(out.String(), "hay") {
		t.Errorf("help output does not mention the binary name; got:\n%s", out.String())
	}
}

func TestUnimplementedCommandsFailLoudly(t *testing.T) {
	// Until milestones land, every stub must return an error rather than
	// silently succeed — a silent no-op would look like data loss to a user.
	cases := [][]string{
		{"add", "/tmp/nowhere"},
		{"search", "anything"},
		{"ask", "anything"},
		{"list"},
		{"remove", "/tmp/nowhere"},
		{"status"},
		{"serve"},
	}

	for _, args := range cases {
		root := newRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Errorf("%v: expected not-implemented error, got nil", args)
		}
	}
}
