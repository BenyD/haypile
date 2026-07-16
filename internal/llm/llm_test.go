package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BenyD/haypile/internal/index"
)

// fakeLLM is an OpenAI-compatible stub: /models lists ids, /chat/completions
// echoes a canned answer and records the prompt it was given.
func fakeLLM(t *testing.T, models []string, answer string, lastPrompt *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		var data []map[string]string
		for _, m := range models {
			data = append(data, map[string]string{"id": m})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if lastPrompt != nil && len(req.Messages) > 0 {
			*lastPrompt = req.Messages[len(req.Messages)-1].Content
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": answer}}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDetectPicksFirstChatModel(t *testing.T) {
	srv := fakeLLM(t, []string{"nomic-embed-text", "all-minilm", "llama3.2"}, "", nil)
	c, err := Detect(context.Background(), srv.URL+"/v1", "")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if c.Model != "llama3.2" {
		t.Fatalf("picked %q, want llama3.2 (embedding models must be skipped)", c.Model)
	}
}

func TestDetectHonorsExplicitModel(t *testing.T) {
	srv := fakeLLM(t, []string{"llama3.2"}, "", nil)
	c, err := Detect(context.Background(), srv.URL+"/v1", "mistral")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if c.Model != "mistral" {
		t.Fatalf("picked %q, want the explicit mistral", c.Model)
	}
}

func TestDetectNoEndpoint(t *testing.T) {
	// Probing must fail fast and report ErrNoEndpoint when nothing local
	// answers. (Guard: a real Ollama on this machine would answer — skip.)
	if _, err := http.Get("http://localhost:11434/v1/models"); err == nil {
		t.Skip("a real local LLM server is running")
	}
	_, err := Detect(context.Background(), "", "")
	if !errors.Is(err, ErrNoEndpoint) {
		t.Fatalf("err = %v, want ErrNoEndpoint", err)
	}
}

func TestDetectErrorsWhenOnlyEmbeddingModels(t *testing.T) {
	srv := fakeLLM(t, []string{"nomic-embed-text"}, "", nil)
	if _, err := Detect(context.Background(), srv.URL+"/v1", ""); err == nil {
		t.Fatal("want an error when the endpoint has no chat model")
	}
}

func TestPing(t *testing.T) {
	srv := fakeLLM(t, []string{"llama3.2"}, "", nil)
	if !Ping(context.Background(), srv.URL+"/v1") {
		t.Error("Ping must be true for a live server")
	}
	srv.Close()
	if Ping(context.Background(), srv.URL+"/v1") {
		t.Error("Ping must be false for a closed server")
	}
}

func TestAnswerBuildsCitedPrompt(t *testing.T) {
	var prompt string
	srv := fakeLLM(t, []string{"llama3.2"}, "Sixty days notice is required [1].", &prompt)

	c, err := Detect(context.Background(), srv.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	results := []index.Result{
		{Path: "/docs/contract.pdf", Page: 2, Snippet: "termination requires sixty days notice"},
		{Path: "/docs/notes.md", Seq: 3, Snippet: "renewal is automatic"},
	}
	answer, err := Answer(context.Background(), c, "what notice period applies?", results)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if !strings.Contains(answer, "[1]") {
		t.Errorf("answer lost its citation: %q", answer)
	}
	for _, want := range []string{
		"[1] contract.pdf, page 2",
		"termination requires sixty days notice",
		"[2] notes.md, section 4",
		"what notice period applies?",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
