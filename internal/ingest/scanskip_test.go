package ingest

import (
	"errors"
	"path/filepath"
	"testing"
)

// A scanned page nobody can read must be marked, not silently empty:
// the mark is what lets hay add explain itself.
func TestScanSkippedMarkedWithoutOCR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || !secs[0].ScanSkipped {
		t.Errorf("page without OCR must carry ScanSkipped, got %+v", secs)
	}
}

// The same page with a failing transcriber (vision server down) counts
// as skipped too; a working transcriber clears the mark.
func TestScanSkippedFollowsOCROutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)

	SetOCR(func([]byte) (string, error) { return "", errors.New("no endpoint") })
	t.Cleanup(func() { SetOCR(nil) })
	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || !secs[0].ScanSkipped {
		t.Errorf("failed OCR must mark ScanSkipped, got %+v", secs)
	}

	SetOCR(func([]byte) (string, error) { return "SECURITY DEPOSIT $2,000", nil })
	secs, err = Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || secs[0].ScanSkipped || secs[0].Text == "" {
		t.Errorf("successful OCR must clear ScanSkipped and carry text, got %+v", secs)
	}
}
