package embed

import (
	"errors"
	"os"

	"github.com/BenyD/haypile/internal/embed/bundled"
)

// FromEnv returns the configured embedder. The bundled in-binary model is
// the no-configuration default; an OpenAI-compatible endpoint is the
// opt-in upgrade path for bigger models and GPU throughput:
//
//	HAYPILE_EMBED_ENDPOINT  base URL of an OpenAI-compatible server,
//	                        e.g. http://localhost:11434/v1
//	HAYPILE_EMBED_MODEL     model name to request from that endpoint
//
// It returns nil only when no backend is usable (a development build
// without weights) — keyword-only mode, always valid.
func FromEnv() (Embedder, error) {
	if url := os.Getenv("HAYPILE_EMBED_ENDPOINT"); url != "" {
		model := os.Getenv("HAYPILE_EMBED_MODEL")
		if model == "" {
			return nil, errors.New("HAYPILE_EMBED_ENDPOINT is set but HAYPILE_EMBED_MODEL is not")
		}
		return NewEndpoint(url, model), nil
	}
	if !bundled.Available() {
		return nil, nil
	}
	return &bundled.Lazy{}, nil
}
