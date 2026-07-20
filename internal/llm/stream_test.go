package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeStreamLLM answers /chat/completions with an OpenAI-style SSE
// stream that emits each word of answer as one delta.
func fakeStreamLLM(t *testing.T, answer string, status int) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Stream {
			t.Errorf("expected a stream:true chat request, got %+v (err %v)", req, err)
		}
		if status != http.StatusOK {
			http.Error(w, "model exploded", status)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, word := range strings.SplitAfter(answer, " ") {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", word)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestChatStreamDeliversTokens(t *testing.T) {
	ts := fakeStreamLLM(t, "sixty days notice", http.StatusOK)
	c := newClient(ts.URL, "fake-chat")

	var got []string
	err := c.ChatStream(context.Background(), "sys", "user", func(tok string) error {
		got = append(got, tok)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if joined := strings.Join(got, ""); joined != "sixty days notice" {
		t.Errorf("tokens joined = %q, want the full answer", joined)
	}
	if len(got) < 2 {
		t.Errorf("got %d tokens, want the answer split across several", len(got))
	}
}

func TestChatStreamSurfacesHTTPError(t *testing.T) {
	ts := fakeStreamLLM(t, "", http.StatusInternalServerError)
	c := newClient(ts.URL, "fake-chat")
	err := c.ChatStream(context.Background(), "sys", "user", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want a 500 error, got %v", err)
	}
}

func TestChatStreamStopsOnCallbackError(t *testing.T) {
	ts := fakeStreamLLM(t, "one two three", http.StatusOK)
	c := newClient(ts.URL, "fake-chat")
	calls := 0
	err := c.ChatStream(context.Background(), "sys", "user", func(string) error {
		calls++
		return fmt.Errorf("stop")
	})
	if err == nil || !strings.Contains(err.Error(), "stop") {
		t.Fatalf("want the callback error back, got %v", err)
	}
	if calls != 1 {
		t.Errorf("callback ran %d times after erroring, want 1", calls)
	}
}
