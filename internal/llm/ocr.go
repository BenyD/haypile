package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// OCR rides the same delegation rule as answering: Haypile ships no
// model, so scanned pages are transcribed by whatever vision-capable
// server the user already runs. No server, no OCR — the page indexes
// empty, exactly as if the feature didn't exist.

const ocrPrompt = "Transcribe all text in this page image, top to bottom, in reading order. " +
	"Separate paragraphs with blank lines. Output only the transcribed text — " +
	"no commentary, no markdown fences. If the page has no text, output nothing."

// visionRequest is a chat request whose user content is the typed-parts
// form OpenAI-compatible servers require for images.
type visionRequest struct {
	Model    string          `json:"model"`
	Messages []visionMessage `json:"messages"`
}

type visionMessage struct {
	Role    string       `json:"role"`
	Content []visionPart `json:"content"`
}

type visionPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// httpError carries the status code so callers can tell "this model
// cannot do vision" (4xx, permanent) from a transient failure.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("LLM endpoint returned %d: %s", e.status, e.msg)
}

// OCRPage sends one page image and returns its transcription.
func (c *Client) OCRPage(ctx context.Context, pngImage []byte) (string, error) {
	body, err := json.Marshal(visionRequest{
		Model: c.Model,
		Messages: []visionMessage{{
			Role: "user",
			Content: []visionPart{
				{Type: "text", Text: ocrPrompt},
				{Type: "image_url", ImageURL: &imageURL{
					URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngImage),
				}},
			},
		}},
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
		return "", &httpError{status: resp.StatusCode, msg: string(msg)}
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

// OCRHook returns the transcriber ingest.SetOCR wants, or nil when OCR
// is switched off. The endpoint is detected lazily on the first scanned
// page — indexing a folder of text never probes anything — and a 4xx
// reply (model can't take images) disables the hook for the rest of the
// process instead of failing page after page.
//
//	HAYPILE_OCR=off        disable OCR entirely
//	HAYPILE_OCR_MODEL      vision model to request (default: first chat model)
func OCRHook() func(pngImage []byte) (string, error) {
	if os.Getenv("HAYPILE_OCR") == "off" {
		return nil
	}

	var (
		mu       sync.Mutex
		once     sync.Once
		client   *Client
		disabled bool
	)
	return func(pngImage []byte) (string, error) {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, err := Detect(ctx, "", os.Getenv("HAYPILE_OCR_MODEL"))
			if err == nil {
				client = c
			}
		})
		mu.Lock()
		dead := disabled || client == nil
		mu.Unlock()
		if dead {
			return "", ErrNoEndpoint
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		text, err := client.OCRPage(ctx, pngImage)
		var herr *httpError
		if errors.As(err, &herr) && herr.status >= 400 && herr.status < 500 {
			mu.Lock()
			disabled = true
			mu.Unlock()
		}
		return text, err
	}
}
