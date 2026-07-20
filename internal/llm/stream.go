package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// streamChunk is one `data:` line of an OpenAI-compatible stream.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ChatStream sends one system+user exchange with stream:true and invokes
// onToken for each content delta as it arrives. It returns once the
// stream ends ([DONE] or EOF) or ctx is cancelled.
func (c *Client) ChatStream(ctx context.Context, system, user string, onToken func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream: true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("LLM endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("LLM endpoint returned %s: %s", resp.Status, msg)
	}

	sc := bufio.NewScanner(resp.Body)
	// Deltas are small, but a model can emit a long line; 1MB of headroom.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // blank separators, comments, event: lines
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return fmt.Errorf("LLM endpoint: bad stream chunk: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if t := chunk.Choices[0].Delta.Content; t != "" {
			if err := onToken(t); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("LLM endpoint: stream: %w", err)
	}
	return nil
}
