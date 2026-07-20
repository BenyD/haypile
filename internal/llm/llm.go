// Package llm speaks to whatever OpenAI-compatible server the user
// already runs (Ollama, LM Studio, llama.cpp, Jan, …). Haypile ships no
// LLM and never will: generation is delegated, search never depends on
// it, and no request ever leaves this machine unless the user points
// --endpoint somewhere else on purpose.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// wellKnownEndpoints are probed in order when nothing is configured.
var wellKnownEndpoints = []struct{ url, name string }{
	{"http://localhost:11434/v1", "ollama"},
	{"http://localhost:1234/v1", "lm-studio"},
	{"http://localhost:8080/v1", "llama.cpp"},
	{"http://localhost:1337/v1", "jan"},
}

// ErrNoEndpoint means no local LLM server was found. Callers degrade to
// search with an explanation — never a hard failure.
var ErrNoEndpoint = errors.New(
	"no local LLM endpoint found (tried Ollama :11434, LM Studio :1234, llama.cpp :8080, Jan :1337);\n" +
		"run `hay llm setup` to get one going, or point hay at yours: hay ask --endpoint http://localhost:PORT/v1")

// Client talks to one chat-completions endpoint.
type Client struct {
	BaseURL string
	Model   string
	http    *http.Client
}

// Detect returns a client for the configured or first responding local
// endpoint. Explicit configuration (flag > env) is trusted verbatim;
// probing is only for the zero-config path.
//
//	HAYPILE_LLM_ENDPOINT  base URL, e.g. http://localhost:11434/v1
//	HAYPILE_LLM_MODEL     model name to request
func Detect(ctx context.Context, endpoint, model string) (*Client, error) {
	if endpoint == "" {
		endpoint = os.Getenv("HAYPILE_LLM_ENDPOINT")
	}
	if model == "" {
		model = os.Getenv("HAYPILE_LLM_MODEL")
	}

	if endpoint != "" {
		c := newClient(endpoint, model)
		if c.Model == "" {
			if err := c.pickModel(ctx); err != nil {
				return nil, fmt.Errorf("%s: %w", endpoint, err)
			}
		}
		return c, nil
	}

	for _, ep := range wellKnownEndpoints {
		c := newClient(ep.url, model)
		probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := c.pickModel(probeCtx)
		cancel()
		if err == nil {
			return c, nil
		}
	}
	return nil, ErrNoEndpoint
}

// Ping reports whether an OpenAI-compatible server answers at baseURL.
// It distinguishes "server down" from "server up but no chat model" —
// two states that need different fixes.
func Ping(ctx context.Context, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return false
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func newClient(baseURL, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		// Local models on modest hardware are slow; generous ceiling.
		http: &http.Client{Timeout: 5 * time.Minute},
	}
}

// listModels asks /models and returns the ids in server order.
func (c *Client) listModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned %s", resp.Status)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// pickModel keeps the first listed entry unless one is already chosen.
// Embedding-only models are skipped by name heuristic — asking an
// embedder to chat produces a confusing server error.
func (c *Client) pickModel(ctx context.Context) error {
	ids, err := c.listModels(ctx)
	if err != nil {
		return err
	}
	if c.Model != "" {
		return nil // endpoint is alive and the model was chosen explicitly
	}
	for _, m := range ids {
		if id := strings.ToLower(m); strings.Contains(id, "embed") || strings.Contains(id, "minilm") {
			continue
		}
		c.Model = m
		return nil
	}
	return fmt.Errorf("endpoint has no chat model loaded (hay ask --model <name> to force one)")
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Chat sends one system+user exchange and returns the reply text.
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("LLM endpoint returned %s: %s", resp.Status, msg)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("LLM endpoint: bad response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("LLM endpoint returned no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
