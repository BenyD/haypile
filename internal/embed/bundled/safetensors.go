package bundled

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

// safetensors is the Hugging Face weight format: an 8-byte little-endian
// header length, a JSON header mapping tensor names to dtype/shape/offsets,
// then the raw tensor bytes. Parsing it directly means no conversion step
// between "download from the hub" and "ship in the binary".

type tensorMeta struct {
	Dtype   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets [2]int `json:"data_offsets"`
}

// tensor is a dense row-major float32 matrix (vectors have cols == len).
type tensor struct {
	rows, cols int
	data       []float32
}

// parseSafetensors decodes the model weights. F32 tensors load directly.
// I8 tensors are weight-only quantized (per-row symmetric, produced by
// hack/fetch-model.sh) and are dequantized here using their companion
// "<name>.scale" F32 tensor — the binary ships small, inference runs at
// full fp32 speed. Other dtypes (e.g. the checkpoint's I64 position_ids
// buffer) are skipped.
func parseSafetensors(raw []byte) (map[string]tensor, error) {
	if len(raw) < 8 {
		return nil, fmt.Errorf("safetensors: file too short (%d bytes)", len(raw))
	}
	headerLen := binary.LittleEndian.Uint64(raw[:8])
	if headerLen > uint64(len(raw)-8) {
		return nil, fmt.Errorf("safetensors: header length %d exceeds file size", headerLen)
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:8+headerLen], &header); err != nil {
		return nil, fmt.Errorf("safetensors: bad header: %w", err)
	}
	body := raw[8+headerLen:]

	out := make(map[string]tensor, len(header))
	quantized := make(map[string][]int8)
	shapes := make(map[string]tensorMeta)

	for name, rawMeta := range header {
		if name == "__metadata__" {
			continue
		}
		var m tensorMeta
		if err := json.Unmarshal(rawMeta, &m); err != nil {
			return nil, fmt.Errorf("safetensors: tensor %q: %w", name, err)
		}
		if m.Offsets[0] < 0 || m.Offsets[1] > len(body) || m.Offsets[0] > m.Offsets[1] {
			return nil, fmt.Errorf("safetensors: tensor %q has offsets out of range", name)
		}
		n := 1
		for _, d := range m.Shape {
			n *= d
		}
		buf := body[m.Offsets[0]:m.Offsets[1]]

		switch m.Dtype {
		case "F32":
			if len(buf) != n*4 {
				return nil, fmt.Errorf("safetensors: tensor %q: %d bytes for %d floats", name, len(buf), n)
			}
			data := make([]float32, n)
			for i := range data {
				data[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
			}
			t, err := shapedTensor(name, m.Shape, data)
			if err != nil {
				return nil, err
			}
			out[name] = t
		case "I8":
			if len(buf) != n {
				return nil, fmt.Errorf("safetensors: tensor %q: %d bytes for %d int8s", name, len(buf), n)
			}
			q := make([]int8, n)
			for i, b := range buf {
				q[i] = int8(b)
			}
			quantized[name] = q
			shapes[name] = m
		default:
			continue
		}
	}

	for name, q := range quantized {
		m := shapes[name]
		if len(m.Shape) != 2 {
			return nil, fmt.Errorf("safetensors: quantized tensor %q must be 2-D", name)
		}
		rows, cols := m.Shape[0], m.Shape[1]
		scale, ok := out[name+".scale"]
		if !ok || scale.rows != 1 || scale.cols != rows {
			return nil, fmt.Errorf("safetensors: quantized tensor %q needs a [%d] F32 %q tensor", name, rows, name+".scale")
		}
		data := make([]float32, len(q))
		for r := 0; r < rows; r++ {
			s := scale.data[r]
			row := q[r*cols : (r+1)*cols]
			for c, v := range row {
				data[r*cols+c] = float32(v) * s
			}
		}
		delete(out, name+".scale")
		out[name] = tensor{rows: rows, cols: cols, data: data}
	}
	return out, nil
}

func shapedTensor(name string, shape []int, data []float32) (tensor, error) {
	t := tensor{rows: 1, cols: len(data), data: data}
	switch len(shape) {
	case 0, 1:
	case 2:
		t.rows, t.cols = shape[0], shape[1]
	default:
		return tensor{}, fmt.Errorf("safetensors: tensor %q has %d dims, want ≤2", name, len(shape))
	}
	return t, nil
}
