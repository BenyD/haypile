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

// parseSafetensors decodes every F32 tensor in the file. Tensors of other
// dtypes are rejected — the fetch script pins the fp32 export.
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
	for name, rawMeta := range header {
		if name == "__metadata__" {
			continue
		}
		var m tensorMeta
		if err := json.Unmarshal(rawMeta, &m); err != nil {
			return nil, fmt.Errorf("safetensors: tensor %q: %w", name, err)
		}
		if m.Dtype != "F32" {
			// Checkpoints carry non-weight buffers (e.g. I64 position_ids);
			// every weight the encoder needs is F32, so skip the rest.
			continue
		}
		if m.Offsets[0] < 0 || m.Offsets[1] > len(body) || m.Offsets[0] > m.Offsets[1] {
			return nil, fmt.Errorf("safetensors: tensor %q has offsets out of range", name)
		}

		n := 1
		for _, d := range m.Shape {
			n *= d
		}
		buf := body[m.Offsets[0]:m.Offsets[1]]
		if len(buf) != n*4 {
			return nil, fmt.Errorf("safetensors: tensor %q: %d bytes for %d floats", name, len(buf), n)
		}

		data := make([]float32, n)
		for i := range data {
			data[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
		}

		t := tensor{rows: 1, cols: n, data: data}
		if len(m.Shape) == 2 {
			t.rows, t.cols = m.Shape[0], m.Shape[1]
		} else if len(m.Shape) == 1 {
			t.rows, t.cols = 1, m.Shape[0]
		} else if len(m.Shape) != 0 {
			return nil, fmt.Errorf("safetensors: tensor %q has %d dims, want ≤2", name, len(m.Shape))
		}
		out[name] = t
	}
	return out, nil
}
