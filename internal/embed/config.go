package embed

import (
	"errors"
	"os"
)

// FromEnv returns the configured embedder, or nil when semantic search is
// not configured (keyword-only mode — always valid).
//
//	HAYPILE_EMBED_ENDPOINT  base URL of an OpenAI-compatible server,
//	                        e.g. http://localhost:11434/v1
//	HAYPILE_EMBED_MODEL     model name to request from that endpoint
//
// The bundled backend becomes the no-configuration default when it lands;
// these variables then act as the opt-in upgrade path.
func FromEnv() (Embedder, error) {
	url := os.Getenv("HAYPILE_EMBED_ENDPOINT")
	if url == "" {
		return nil, nil
	}
	model := os.Getenv("HAYPILE_EMBED_MODEL")
	if model == "" {
		return nil, errors.New("HAYPILE_EMBED_ENDPOINT is set but HAYPILE_EMBED_MODEL is not")
	}
	return NewEndpoint(url, model), nil
}
