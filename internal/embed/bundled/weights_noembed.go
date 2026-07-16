//go:build !bundled

package bundled

// Development builds compile without the 90MB weight file so a fresh
// clone builds with plain `go build`. New() falls back to
// $HAYPILE_MODEL_PATH in this mode.
var embeddedWeights []byte
