//go:build bundled

package bundled

import _ "embed"

// The release build carries the model inside the binary — that is the
// product promise: install, index, search, zero downloads.
//
//go:embed model.safetensors
var embeddedWeights []byte
