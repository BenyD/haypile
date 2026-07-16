package bundled

import (
	"fmt"
	"math"
	"strings"
)

// MiniLM-L6 architecture constants. These describe the bundled checkpoint;
// they are not tunables.
const (
	hiddenDim    = 384
	numLayers    = 6
	numHeads     = 12
	headDim      = hiddenDim / numHeads
	intermediate = 1536
	epsLayerNorm = 1e-12
)

// linear is a dense layer with HF convention: weight is [out, in],
// y = xWᵀ + b.
type linear struct {
	w tensor
	b []float32
}

type layerNorm struct {
	gamma, beta []float32
}

type encoderLayer struct {
	query, key, value linear
	attnOut           linear
	attnNorm          layerNorm
	ffnUp, ffnDown    linear
	ffnNorm           layerNorm
}

// model holds the full MiniLM encoder weights.
type model struct {
	wordEmb, posEmb, typeEmb tensor
	embNorm                  layerNorm
	layers                   [numLayers]encoderLayer
}

// newModel wires parsed safetensors into the encoder. Weight names follow
// the HF BertModel export, with or without a "bert." prefix.
func newModel(tensors map[string]tensor) (*model, error) {
	prefix := ""
	if _, ok := tensors["bert.embeddings.word_embeddings.weight"]; ok {
		prefix = "bert."
	}

	var missing []string
	get := func(name string) tensor {
		t, ok := tensors[prefix+name]
		if !ok {
			missing = append(missing, name)
		}
		return t
	}
	lin := func(name string) linear {
		return linear{w: get(name + ".weight"), b: get(name + ".bias").data}
	}
	ln := func(name string) layerNorm {
		return layerNorm{gamma: get(name + ".weight").data, beta: get(name + ".bias").data}
	}

	m := &model{
		wordEmb: get("embeddings.word_embeddings.weight"),
		posEmb:  get("embeddings.position_embeddings.weight"),
		typeEmb: get("embeddings.token_type_embeddings.weight"),
		embNorm: ln("embeddings.LayerNorm"),
	}
	for i := range m.layers {
		p := fmt.Sprintf("encoder.layer.%d.", i)
		m.layers[i] = encoderLayer{
			query:    lin(p + "attention.self.query"),
			key:      lin(p + "attention.self.key"),
			value:    lin(p + "attention.self.value"),
			attnOut:  lin(p + "attention.output.dense"),
			attnNorm: ln(p + "attention.output.LayerNorm"),
			ffnUp:    lin(p + "intermediate.dense"),
			ffnDown:  lin(p + "output.dense"),
			ffnNorm:  ln(p + "output.LayerNorm"),
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("model weights missing tensors: %s", strings.Join(missing, ", "))
	}
	if m.wordEmb.cols != hiddenDim {
		return nil, fmt.Errorf("model hidden size is %d, want %d", m.wordEmb.cols, hiddenDim)
	}
	return m, nil
}

// forward runs the encoder over one tokenized sequence and returns the
// mean-pooled, L2-normalized sentence embedding.
func (m *model) forward(ids []int32) []float32 {
	seq := len(ids)
	hidden := make([]float32, seq*hiddenDim)

	// Embedding sum: word + position + token type (always 0), then LayerNorm.
	for i, id := range ids {
		w := m.wordEmb.data[int(id)*hiddenDim:]
		p := m.posEmb.data[i*hiddenDim:]
		t := m.typeEmb.data[:hiddenDim]
		row := hidden[i*hiddenDim : (i+1)*hiddenDim]
		for j := range row {
			row[j] = w[j] + p[j] + t[j]
		}
		applyLayerNorm(row, m.embNorm)
	}

	scores := make([]float32, seq*seq) // attention score scratch
	q := make([]float32, seq*hiddenDim)
	k := make([]float32, seq*hiddenDim)
	v := make([]float32, seq*hiddenDim)
	ctxOut := make([]float32, seq*hiddenDim)
	ffnMid := make([]float32, seq*intermediate)
	residual := make([]float32, seq*hiddenDim)

	for _, layer := range m.layers {
		// Self-attention. No mask needed: sequences are unpadded.
		matmulT(q, hidden, layer.query, seq)
		matmulT(k, hidden, layer.key, seq)
		matmulT(v, hidden, layer.value, seq)

		scale := float32(1 / math.Sqrt(headDim))
		for h := 0; h < numHeads; h++ {
			off := h * headDim
			for i := 0; i < seq; i++ {
				qi := q[i*hiddenDim+off : i*hiddenDim+off+headDim]
				row := scores[i*seq : (i+1)*seq]
				for j := 0; j < seq; j++ {
					kj := k[j*hiddenDim+off : j*hiddenDim+off+headDim]
					row[j] = dot(qi, kj) * scale
				}
				softmax(row)
				out := ctxOut[i*hiddenDim+off : i*hiddenDim+off+headDim]
				for d := range out {
					out[d] = 0
				}
				for j := 0; j < seq; j++ {
					vj := v[j*hiddenDim+off : j*hiddenDim+off+headDim]
					w := row[j]
					for d := 0; d < headDim; d++ {
						out[d] += w * vj[d]
					}
				}
			}
		}

		copy(residual, hidden)
		matmulT(hidden, ctxOut, layer.attnOut, seq)
		addAndNorm(hidden, residual, layer.attnNorm)

		// Feed-forward: up-project, GELU, down-project.
		matmulTWide(ffnMid, hidden, layer.ffnUp, seq)
		for i := range ffnMid {
			ffnMid[i] = gelu(ffnMid[i])
		}
		copy(residual, hidden)
		matmulTNarrow(hidden, ffnMid, layer.ffnDown, seq)
		addAndNorm(hidden, residual, layer.ffnNorm)
	}

	// Mean pooling over all tokens, then L2 normalize.
	emb := make([]float32, hiddenDim)
	for i := 0; i < seq; i++ {
		row := hidden[i*hiddenDim:]
		for j := 0; j < hiddenDim; j++ {
			emb[j] += row[j]
		}
	}
	var sum float64
	for j := range emb {
		emb[j] /= float32(seq)
		sum += float64(emb[j]) * float64(emb[j])
	}
	if sum > 0 {
		inv := float32(1 / math.Sqrt(sum))
		for j := range emb {
			emb[j] *= inv
		}
	}
	return emb
}

// matmulT computes out = in·Wᵀ + b for a square [hidden→hidden] layer,
// row per token.
func matmulT(out, in []float32, l linear, seq int) {
	mm(out, in, l, seq, hiddenDim, hiddenDim)
}

// matmulTWide is the FFN up-projection [hidden→intermediate].
func matmulTWide(out, in []float32, l linear, seq int) {
	mm(out, in, l, seq, hiddenDim, intermediate)
}

// matmulTNarrow is the FFN down-projection [intermediate→hidden].
func matmulTNarrow(out, in []float32, l linear, seq int) {
	mm(out, in, l, seq, intermediate, hiddenDim)
}

func mm(out, in []float32, l linear, seq, inDim, outDim int) {
	for i := 0; i < seq; i++ {
		x := in[i*inDim : (i+1)*inDim]
		row := out[i*outDim : (i+1)*outDim]
		for o := 0; o < outDim; o++ {
			row[o] = dot(x, l.w.data[o*inDim:(o+1)*inDim]) + l.b[o]
		}
	}
}

// dot uses four independent accumulators: breaking the loop-carried
// dependency is worth ~3x — this loop is where indexing time goes.
func dot(x, w []float32) float32 {
	var d0, d1, d2, d3 float32
	n := len(x) &^ 3
	w = w[:len(x)]
	for j := 0; j < n; j += 4 {
		d0 += x[j] * w[j]
		d1 += x[j+1] * w[j+1]
		d2 += x[j+2] * w[j+2]
		d3 += x[j+3] * w[j+3]
	}
	for j := n; j < len(x); j++ {
		d0 += x[j] * w[j]
	}
	return d0 + d1 + d2 + d3
}

func addAndNorm(hidden, residual []float32, ln layerNorm) {
	for i := range hidden {
		hidden[i] += residual[i]
	}
	for i := 0; i < len(hidden); i += hiddenDim {
		applyLayerNorm(hidden[i:i+hiddenDim], ln)
	}
}

func applyLayerNorm(row []float32, ln layerNorm) {
	var mean float64
	for _, x := range row {
		mean += float64(x)
	}
	mean /= float64(len(row))
	var variance float64
	for _, x := range row {
		d := float64(x) - mean
		variance += d * d
	}
	variance /= float64(len(row))
	inv := 1 / math.Sqrt(variance+epsLayerNorm)
	for i, x := range row {
		row[i] = float32((float64(x)-mean)*inv)*ln.gamma[i] + ln.beta[i]
	}
}

func softmax(row []float32) {
	max := row[0]
	for _, x := range row[1:] {
		if x > max {
			max = x
		}
	}
	var sum float32
	for i, x := range row {
		e := float32(math.Exp(float64(x - max)))
		row[i] = e
		sum += e
	}
	for i := range row {
		row[i] /= sum
	}
}

// gelu is the exact (erf) variant BERT was trained with.
func gelu(x float32) float32 {
	return float32(0.5 * float64(x) * (1 + math.Erf(float64(x)/math.Sqrt2)))
}
