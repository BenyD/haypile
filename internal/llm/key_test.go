package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// keyRecordingLLM is an OpenAI-compatible stub that records the
// Authorization header of every request it serves.
func keyRecordingLLM(t *testing.T, gotAuth *[]string) *httptest.Server {
	t.Helper()
	record := func(r *http.Request) { *gotAuth = append(*gotAuth, r.Header.Get("Authorization")) }
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "test-chat"}},
		})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	})
	return httptest.NewServer(mux)
}

// Every request an authenticated client makes must carry the key: model
// discovery and chat alike, because cloud endpoints 401 either one.
func TestKeySentAsBearerEverywhere(t *testing.T) {
	var auth []string
	srv := keyRecordingLLM(t, &auth)
	defer srv.Close()

	c, err := Detect(context.Background(), srv.URL+"/v1", "", "sk-test")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if _, err := c.Chat(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if len(auth) < 2 {
		t.Fatalf("expected at least 2 recorded requests, got %d", len(auth))
	}
	for i, a := range auth {
		if a != "Bearer sk-test" {
			t.Errorf("request %d: Authorization = %q, want Bearer sk-test", i, a)
		}
	}
}

// Without a key there must be no Authorization header at all; local
// servers do not expect one and some reject unknown auth outright.
func TestNoKeyNoHeader(t *testing.T) {
	var auth []string
	srv := keyRecordingLLM(t, &auth)
	defer srv.Close()

	c, err := Detect(context.Background(), srv.URL+"/v1", "", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if _, err := c.Chat(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	for i, a := range auth {
		if a != "" {
			t.Errorf("request %d: unexpected Authorization %q", i, a)
		}
	}
}

// The env var is the flagless path; it must reach the header too.
func TestKeyFromEnv(t *testing.T) {
	var auth []string
	srv := keyRecordingLLM(t, &auth)
	defer srv.Close()

	t.Setenv("HAYPILE_LLM_API_KEY", "sk-env")
	c, err := Detect(context.Background(), srv.URL+"/v1", "", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if _, err := c.Chat(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	for i, a := range auth {
		if a != "Bearer sk-env" {
			t.Errorf("request %d: Authorization = %q, want Bearer sk-env", i, a)
		}
	}
}

// A key over cleartext http to another machine would hand the secret to
// the network; Detect must refuse before any request is made. Loopback
// stays allowed because local servers speak plain http by design.
func TestKeyRefusedOverPlainHTTP(t *testing.T) {
	_, err := Detect(context.Background(), "http://example.com/v1", "m", "sk-test")
	if err == nil {
		t.Fatal("Detect accepted a key over plain http to a remote host")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("error %q does not point at the fix", err)
	}
}
