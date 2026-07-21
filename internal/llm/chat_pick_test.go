package llm

import "testing"

// Installing a vision model for OCR must never become the default
// voice of hay ask: text questions go to a plain chat model whenever
// one is loaded.
func TestChatModelID(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		want string
	}{
		{"empty", nil, ""},
		{"plain chat", []string{"llama3.2:3b"}, "llama3.2:3b"},
		{"vision listed first still loses", []string{"llava:latest", "llama3.2:3b"}, "llama3.2:3b"},
		{"qwen vl loses to chat", []string{"qwen3-vl", "llama3.2:3b"}, "llama3.2:3b"},
		{"only vision: better than nothing", []string{"llava:latest"}, "llava:latest"},
		{"embedders never chat", []string{"nomic-embed-text", "all-minilm"}, ""},
		{"embedder then chat", []string{"nomic-embed-text", "qwen2.5:7b"}, "qwen2.5:7b"},
	}
	for _, c := range cases {
		if got := ChatModelID(c.ids); got != c.want {
			t.Errorf("%s: ChatModelID(%v) = %q, want %q", c.name, c.ids, got, c.want)
		}
	}
}
