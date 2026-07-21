package daemon

import "testing"

func TestStaleVersion(t *testing.T) {
	cases := []struct {
		daemon, cli string
		want        bool
	}{
		{"0.2.0", "0.2.1", true},  // the post-upgrade leftover
		{"0.2.1", "0.2.1", false}, // in step
		{"0.2.1", "dev", true},    // developer binary replaces a release daemon
		{"", "0.2.1", false},      // unknown daemon version: leave it be
		{"0.2.1", "", false},      // CLI without a version never kills anything
	}
	for _, c := range cases {
		if got := staleVersion(c.daemon, c.cli); got != c.want {
			t.Errorf("staleVersion(%q, %q) = %v, want %v", c.daemon, c.cli, got, c.want)
		}
	}
}
