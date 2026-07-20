package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeVisionLLM stubs /chat/completions for the typed-parts request shape
// and records the payload it received.
func fakeVisionLLM(t *testing.T, answer string, status int, got *visionRequest) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "llava"}},
		})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			json.NewDecoder(r.Body).Decode(got)
		}
		if status != http.StatusOK {
			http.Error(w, "model does not support images", status)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": answer}}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestOCRPageSendsImageAndReturnsText(t *testing.T) {
	var got visionRequest
	srv := fakeVisionLLM(t, "Scanned invoice from ACME", http.StatusOK, &got)

	c := newClient(srv.URL+"/v1", "llava")
	text, err := c.OCRPage(context.Background(), []byte("png-bytes"))
	if err != nil {
		t.Fatalf("OCRPage: %v", err)
	}
	if text != "Scanned invoice from ACME" {
		t.Errorf("got %q", text)
	}

	if got.Model != "llava" {
		t.Errorf("request model = %q", got.Model)
	}
	if len(got.Messages) != 1 || len(got.Messages[0].Content) != 2 {
		t.Fatalf("want one message with two parts, got %+v", got.Messages)
	}
	img := got.Messages[0].Content[1]
	if img.Type != "image_url" || img.ImageURL == nil ||
		!strings.HasPrefix(img.ImageURL.URL, "data:image/png;base64,") {
		t.Errorf("second part is not a base64 png data URL: %+v", img)
	}
}

func TestOCRHookDisablesAfterClientError(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "text-only"}},
		})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "images not supported", http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("HAYPILE_LLM_ENDPOINT", srv.URL+"/v1")
	hook := OCRHook()
	if hook == nil {
		t.Fatal("hook is nil with an endpoint configured")
	}

	if _, err := hook([]byte("png")); err == nil {
		t.Fatal("want an error from a 400 reply")
	}
	if _, err := hook([]byte("png")); err == nil {
		t.Fatal("want the hook disabled after a client error")
	}
	if calls != 1 {
		t.Errorf("endpoint was called %d times, want 1 — a 4xx must disable the hook", calls)
	}
}

func TestOCRHookOffSwitch(t *testing.T) {
	t.Setenv("HAYPILE_OCR", "off")
	if OCRHook() != nil {
		t.Fatal("HAYPILE_OCR=off must return a nil hook")
	}
}
