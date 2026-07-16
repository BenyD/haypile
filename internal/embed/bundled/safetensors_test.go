package bundled

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// buildSafetensors assembles a file from (dtype, shape, raw bytes) specs.
func buildSafetensors(t *testing.T, specs map[string]struct {
	dtype string
	shape []int
	data  []byte
}) []byte {
	t.Helper()
	header := make(map[string]any)
	var body []byte
	for name, s := range specs {
		header[name] = map[string]any{
			"dtype":        s.dtype,
			"shape":        s.shape,
			"data_offsets": []int{len(body), len(body) + len(s.data)},
		}
		body = append(body, s.data...)
	}
	hdr, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 8, 8+len(hdr)+len(body))
	binary.LittleEndian.PutUint64(out, uint64(len(hdr)))
	return append(append(out, hdr...), body...)
}

func f32bytes(vals ...float32) []byte {
	b := make([]byte, 4*len(vals))
	for i, v := range vals {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(v))
	}
	return b
}

func TestParseSafetensorsDequantizesI8(t *testing.T) {
	// 2x3 int8 tensor with per-row scales 0.5 and 2.
	raw := buildSafetensors(t, map[string]struct {
		dtype string
		shape []int
		data  []byte
	}{
		"w":       {"I8", []int{2, 3}, []byte{0x81 /* -127 */, 0, 64, 1, 0xFE /* -2 */, 127}},
		"w.scale": {"F32", []int{2}, f32bytes(0.5, 2)},
		"b":       {"F32", []int{3}, f32bytes(1, 2, 3)},
	})

	tensors, err := parseSafetensors(raw)
	if err != nil {
		t.Fatalf("parseSafetensors: %v", err)
	}
	if _, ok := tensors["w.scale"]; ok {
		t.Error("scale tensor must be consumed, not exposed")
	}
	w := tensors["w"]
	want := []float32{-63.5, 0, 32, 2, -4, 254}
	if w.rows != 2 || w.cols != 3 {
		t.Fatalf("w is %dx%d, want 2x3", w.rows, w.cols)
	}
	for i, x := range want {
		if w.data[i] != x {
			t.Errorf("w[%d] = %g, want %g", i, w.data[i], x)
		}
	}
	if b := tensors["b"]; b.cols != 3 || b.data[2] != 3 {
		t.Errorf("plain F32 tensor mangled: %+v", b)
	}
}

func TestParseSafetensorsI8MissingScale(t *testing.T) {
	raw := buildSafetensors(t, map[string]struct {
		dtype string
		shape []int
		data  []byte
	}{
		"w": {"I8", []int{1, 2}, []byte{1, 2}},
	})
	if _, err := parseSafetensors(raw); err == nil {
		t.Fatal("want an error for quantized tensor without scales")
	}
}

// TestQuantizedEmbedQuality gates the shipped (quantized) weights: close
// to the fp32 reference, and far above the noise floor for retrieval.
// Skips when model.q8.safetensors hasn't been generated.
func TestQuantizedEmbedQuality(t *testing.T) {
	if _, err := loadQuantized(t); err != nil {
		t.Skipf("quantized model not present (run hack/fetch-model.sh): %v", err)
	}
	b, _ := loadQuantized(t)
	g := loadGoldens(t)

	texts := make([]string, len(g.Cases))
	for i, c := range g.Cases {
		texts[i] = c.Text
	}
	got, err := b.Embed(t.Context(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	for i, c := range g.Cases {
		var cos float64
		for j := range c.Embedding {
			cos += float64(got[i][j]) * float64(c.Embedding[j])
		}
		// Weight-only int8 keeps embeddings nearly identical; 0.99 cosine
		// against fp32 reference is far beyond what ranking needs.
		if cos < 0.99 {
			t.Errorf("case %d (%.40q): cosine vs fp32 reference = %.4f, want ≥ 0.99", i, c.Text, cos)
		}
	}
}

func loadQuantized(t *testing.T) (*Bundled, error) {
	t.Helper()
	if len(embeddedWeights) > 0 {
		return New()
	}
	if _, err := os.Stat("model.q8.safetensors"); err != nil {
		return nil, err
	}
	t.Setenv("HAYPILE_MODEL_PATH", "model.q8.safetensors")
	return New()
}
