package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir on this system")
	}
	cases := map[string]string{
		"~":              home,
		"~/Documents":    filepath.Join(home, "Documents"),
		`~\Documents`:    filepath.Join(home, "Documents"),
		"~someone/else":  "~someone/else", // another user's home is not ours to guess
		"/absolute/path": "/absolute/path",
		"relative":       "relative",
	}
	for in, want := range cases {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}
