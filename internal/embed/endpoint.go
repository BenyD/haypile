package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Endpoint embeds via any OpenAI-compatible server's /embeddings route
// (Ollama, LM Studio, llama.cpp server, …). It exists so power users can
// trade the bundled model for a bigger one with GPU throughput.
type Endpoint struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewEndpoint returns an embedder for an OpenAI-compatible base URL, e.g.
// http://localhost:11434/v1 for Ollama.
func NewEndpoint(baseURL, model string) *Endpoint {
	return &Endpoint{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (e *Endpoint) Model() string { return "endpoint/" + e.model }

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *Endpoint) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(embeddingRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embedding endpoint returned %s: %s", resp.Status, msg)
	}

	var parsed embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("embedding endpoint: bad response: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding endpoint: got %d vectors for %d texts",
			len(parsed.Data), len(texts))
	}

	sort.Slice(parsed.Data, func(i, j int) bool {
		return parsed.Data[i].Index < parsed.Data[j].Index
	})
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		normalize(d.Embedding)
		out[i] = d.Embedding
	}
	return out, nil
}
