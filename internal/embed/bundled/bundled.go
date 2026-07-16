package bundled

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// ModelName identifies the bundled checkpoint. It is recorded in every
// index this backend builds; vectors from different models never mix.
const ModelName = "bundled/all-MiniLM-L6-v2"

//go:embed vocab.txt
var vocabData string

// Bundled embeds text with the in-binary MiniLM model. It is safe for
// concurrent use; the weights are read-only after load.
type Bundled struct {
	tok *tokenizer
	m   *model
}

// New loads the bundled model. Weight bytes come from the binary when it
// was built with -tags bundled, else from $HAYPILE_MODEL_PATH (a
// model.safetensors file) for development builds.
func New() (*Bundled, error) {
	raw := embeddedWeights
	if len(raw) == 0 {
		path := os.Getenv("HAYPILE_MODEL_PATH")
		if path == "" {
			return nil, errors.New(
				"no bundled model in this build: rebuild with -tags bundled " +
					"(after hack/fetch-model.sh) or set HAYPILE_MODEL_PATH")
		}
		var err error
		if raw, err = os.ReadFile(path); err != nil {
			return nil, fmt.Errorf("HAYPILE_MODEL_PATH: %w", err)
		}
	}

	tensors, err := parseSafetensors(raw)
	if err != nil {
		return nil, err
	}
	m, err := newModel(tensors)
	if err != nil {
		return nil, err
	}
	tok, err := newTokenizer(strings.NewReader(vocabData))
	if err != nil {
		return nil, err
	}
	return &Bundled{tok: tok, m: m}, nil
}

func (b *Bundled) Model() string { return ModelName }

// Available reports whether New can succeed: weights are in the binary or
// reachable via HAYPILE_MODEL_PATH. It lets callers choose keyword-only
// mode up front instead of erroring at embed time.
func Available() bool {
	if len(embeddedWeights) > 0 {
		return true
	}
	path := os.Getenv("HAYPILE_MODEL_PATH")
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// Lazy defers weight parsing until the first Embed call, so commands that
// never embed (list, remove, keyword-only paths) skip the load entirely.
type Lazy struct {
	once sync.Once
	b    *Bundled
	err  error
}

func (l *Lazy) Model() string { return ModelName }

func (l *Lazy) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	l.once.Do(func() { l.b, l.err = New() })
	if l.err != nil {
		return nil, l.err
	}
	return l.b.Embed(ctx, texts)
}

// Embed runs the model over each text, fanned out across CPUs — indexing
// throughput is the whole game for a local-first tool.
func (b *Bundled) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	if len(texts) == 0 {
		return out, nil
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > len(texts) {
		workers = len(texts)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = b.m.forward(b.tok.encode(texts[i]))
			}
		}()
	}

	var err error
feed:
	for i := range texts {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break feed
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	return out, err
}
