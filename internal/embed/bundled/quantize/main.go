// Command quantize converts fp32 safetensors weights to weight-only int8:
// every large 2-D tensor becomes an I8 tensor plus a per-row F32
// "<name>.scale" companion. The loader dequantizes at startup, so this
// trades ~3x binary size for a one-time load cost and a ~0.3% weight
// rounding error — retrieval quality is guarded by the eval set.
//
// Run via hack/fetch-model.sh, or directly:
//
//	go run ./internal/embed/bundled/quantize -in model.safetensors -out model.q8.safetensors
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
)

// quantizeThreshold is the element count below which a tensor stays F32:
// biases, LayerNorms, and the tiny embedding tables are not worth the
// rounding error.
const quantizeThreshold = 100_000

type meta struct {
	Dtype   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets [2]int `json:"data_offsets"`
}

type outTensor struct {
	dtype string
	shape []int
	data  []byte
}

func main() {
	in := flag.String("in", "internal/embed/bundled/model.safetensors", "fp32 safetensors input")
	out := flag.String("out", "internal/embed/bundled/model.q8.safetensors", "quantized output")
	flag.Parse()

	raw, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	tensors, order, err := parse(raw)
	if err != nil {
		log.Fatalf("parsing %s: %v", *in, err)
	}

	result := make(map[string]outTensor)
	var names []string
	quantizedCount := 0
	for _, name := range order {
		t := tensors[name]
		n := 1
		for _, d := range t.meta.Shape {
			n *= d
		}
		if t.meta.Dtype != "F32" || len(t.meta.Shape) != 2 || n < quantizeThreshold {
			// Pass through untouched (including non-F32 buffers).
			result[name] = outTensor{dtype: t.meta.Dtype, shape: t.meta.Shape, data: t.data}
			names = append(names, name)
			continue
		}

		rows, cols := t.meta.Shape[0], t.meta.Shape[1]
		q := make([]byte, rows*cols)
		scales := make([]byte, rows*4)
		for r := 0; r < rows; r++ {
			var maxAbs float64
			for c := 0; c < cols; c++ {
				v := math.Abs(float64(f32At(t.data, r*cols+c)))
				if v > maxAbs {
					maxAbs = v
				}
			}
			scale := maxAbs / 127
			if scale == 0 {
				scale = 1
			}
			binary.LittleEndian.PutUint32(scales[r*4:], math.Float32bits(float32(scale)))
			for c := 0; c < cols; c++ {
				v := math.RoundToEven(float64(f32At(t.data, r*cols+c)) / scale)
				q[r*cols+c] = byte(int8(v))
			}
		}
		result[name] = outTensor{dtype: "I8", shape: t.meta.Shape, data: q}
		result[name+".scale"] = outTensor{dtype: "F32", shape: []int{rows}, data: scales}
		names = append(names, name, name+".scale")
		quantizedCount++
	}

	if err := write(*out, result, names); err != nil {
		log.Fatal(err)
	}
	fi, _ := os.Stat(*out)
	fmt.Printf("quantized %d tensors: %s (%.1f MB) -> %s (%.1f MB)\n",
		quantizedCount, *in, float64(len(raw))/1e6, *out, float64(fi.Size())/1e6)
}

type parsed struct {
	meta meta
	data []byte
}

func parse(raw []byte) (map[string]parsed, []string, error) {
	if len(raw) < 8 {
		return nil, nil, fmt.Errorf("file too short")
	}
	headerLen := binary.LittleEndian.Uint64(raw[:8])
	if headerLen > uint64(len(raw)-8) {
		return nil, nil, fmt.Errorf("header length out of range")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:8+headerLen], &header); err != nil {
		return nil, nil, err
	}
	body := raw[8+headerLen:]

	out := make(map[string]parsed)
	var order []string
	for name, rawMeta := range header {
		if name == "__metadata__" {
			continue
		}
		var m meta
		if err := json.Unmarshal(rawMeta, &m); err != nil {
			return nil, nil, fmt.Errorf("tensor %q: %w", name, err)
		}
		if m.Offsets[0] < 0 || m.Offsets[1] > len(body) || m.Offsets[0] > m.Offsets[1] {
			return nil, nil, fmt.Errorf("tensor %q: offsets out of range", name)
		}
		out[name] = parsed{meta: m, data: body[m.Offsets[0]:m.Offsets[1]]}
		order = append(order, name)
	}
	sort.Strings(order)
	return out, order, nil
}

func write(path string, tensors map[string]outTensor, names []string) error {
	sort.Strings(names)
	header := make(map[string]meta, len(names))
	offset := 0
	for _, name := range names {
		t := tensors[name]
		header[name] = meta{Dtype: t.dtype, Shape: t.shape, Offsets: [2]int{offset, offset + len(t.data)}}
		offset += len(t.data)
	}
	hdr, err := json.Marshal(header)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(hdr)))
	for _, b := range [][]byte{lenBuf[:], hdr} {
		if _, err := f.Write(b); err != nil {
			return err
		}
	}
	for _, name := range names {
		if _, err := f.Write(tensors[name].data); err != nil {
			return err
		}
	}
	return f.Close()
}

func f32At(b []byte, i int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
}
