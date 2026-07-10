// Package ingest turns documents on disk into indexed chunks: it walks
// folders, extracts text, and splits it for the index. PDF and docx
// extraction arrive at M2; M0 handles plain-text formats.
package ingest

import (
	"path/filepath"
	"strings"
)

var supportedExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
}

// Supported reports whether the file at path is a format Haypile can index.
func Supported(path string) bool {
	return supportedExts[strings.ToLower(filepath.Ext(path))]
}
