package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Indexing a large folder runs synchronously and can take an hour. It must
// not share a deadline with the ordinary calls, which should still fail
// fast when the daemon is unreachable.
func TestAddSourceOutlivesTheOrdinaryCallTimeout(t *testing.T) {
	const slow = 150 * time.Millisecond

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(slow)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"indexed": 1, "chunks": 7})
	}))
	defer ts.Close()

	c := &Client{
		base: ts.URL,
		http: &http.Client{Timeout: slow / 5}, // stands in for callTimeout
		long: &http.Client{},
	}

	stats, err := c.AddSource(t.TempDir(), "")
	if err != nil {
		t.Fatalf("AddSource cut off a slow pass: %v", err)
	}
	if stats.Chunks != 7 {
		t.Fatalf("chunks = %d, want 7", stats.Chunks)
	}

	// The same slowness on an ordinary call is a daemon that is not
	// answering, and should surface rather than hang.
	if _, err := c.Query("anything", "", 3); err == nil {
		t.Fatal("Query waited past its timeout")
	}
}

// Discover wires the two clients; a regression that points AddSource back
// at the bounded one would reintroduce the cutoff.
func TestClientTimeoutsAreDistinct(t *testing.T) {
	c := &Client{
		http: &http.Client{Timeout: callTimeout},
		long: &http.Client{},
	}
	if c.http.Timeout != callTimeout {
		t.Errorf("ordinary calls: timeout %v, want %v", c.http.Timeout, callTimeout)
	}
	if c.long.Timeout != 0 {
		t.Errorf("indexing: timeout %v, want none", c.long.Timeout)
	}
}
