package daemon

import "testing"

const netstatSample = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1044
  TCP    127.0.0.1:11500        0.0.0.0:0              LISTENING       4212
  TCP    127.0.0.1:11500        127.0.0.1:52310        ESTABLISHED     4212
  TCP    10.0.0.5:52034         93.184.216.34:443      ESTABLISHED     4212
  TCP    10.0.0.5:52099         93.184.216.34:443      TIME_WAIT       0
  TCP    10.0.0.5:53000         52.84.10.9:443         ESTABLISHED     9999
  TCP    [::1]:11500            [::]:0                 LISTENING       4212
  TCP    [::1]:11500            [::1]:52401            ESTABLISHED     4212
`

func TestParseNetstatOutbound(t *testing.T) {
	// For pid 4212: two listeners skipped, two loopback peers skipped,
	// exactly one real outbound connection remains.
	if got := parseNetstatOutbound(netstatSample, 4212); got != 1 {
		t.Errorf("pid 4212: got %d outbound, want 1", got)
	}
	// Another process's connections never count.
	if got := parseNetstatOutbound(netstatSample, 1044); got != 0 {
		t.Errorf("pid 1044: got %d outbound, want 0", got)
	}
	if got := parseNetstatOutbound("", 4212); got != 0 {
		t.Errorf("empty output: got %d, want 0", got)
	}
}
