package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestLLMSetupAlreadyConfigured: with a working endpoint, setup verifies
// and reports ready without installing or downloading anything.
func TestLLMSetupAlreadyConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "llama3.2"}}})
		case "/v1/chat/completions":
			json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "OK"}},
			}})
		}
	}))
	defer srv.Close()

	t.Setenv("HAYPILE_LLM_ENDPOINT", srv.URL+"/v1")
	out, err := run(t, "llm", "setup")
	if err != nil {
		t.Fatalf("llm setup: %v\n%s", err, out)
	}
	for _, want := range []string{"Found a running LLM server", "works", "hay ask"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrompter(t *testing.T) {
	tests := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true},
		{"Y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"\n", false, false},     // enter = default
		{"\n", true, true},       // enter = default
		{"", false, false},       // closed stdin = default
		{"what\n", false, false}, // unrecognized = default
	}
	for _, tt := range tests {
		cmd := &cobra.Command{}
		cmd.SetIn(strings.NewReader(tt.input))
		cmd.SetOut(&bytes.Buffer{})
		if got := newPrompter(cmd).yesNo("?", tt.def); got != tt.want {
			t.Errorf("yesNo(input=%q, def=%v) = %v, want %v", tt.input, tt.def, got, tt.want)
		}
	}

	// One prompter serves several questions off the same reader — the
	// bug this type exists to prevent.
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("alpha\n\ny\n"))
	cmd.SetOut(&bytes.Buffer{})
	p := newPrompter(cmd)
	if got := p.line("?", "d1"); got != "alpha" {
		t.Errorf("first line = %q", got)
	}
	if got := p.line("?", "d2"); got != "d2" {
		t.Errorf("second line (empty) = %q, want default", got)
	}
	if !p.yesNo("?", false) {
		t.Error("third prompt should read the y")
	}
}
