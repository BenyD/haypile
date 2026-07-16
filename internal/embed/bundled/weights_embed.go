//go:build bundled

package bundled

import _ "embed"

// The release build carries the model inside the binary — that is the
// product promise: install, index, search, zero downloads. The int8
// quantized weights keep the binary ~3x smaller; the loader dequantizes
// once at startup.
//
//go:embed model.q8.safetensors
var embeddedWeights []byte
