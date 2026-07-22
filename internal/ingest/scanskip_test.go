package ingest

import (
	"errors"
	"path/filepath"
	"testing"
)

// ocrDown mimics the hook's "no vision endpoint at all" error shape.
type ocrDown struct{}

func (ocrDown) Error() string        { return "no endpoint" }
func (ocrDown) OCRUnavailable() bool { return true }

// A scanned page nobody can read must be marked, not silently empty:
// the marks are what let hay add explain itself, and the two marks
// differ because "install a model" and "retry" are different advice.
func TestScanSkippedWithoutOCRHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || !secs[0].ScanSkipped || secs[0].ScanFailed {
		t.Errorf("page without any OCR hook must carry ScanSkipped, got %+v", secs)
	}
}

func TestScanMarksFollowOCROutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)
	t.Cleanup(func() { SetOCR(nil) })

	// No endpoint behind the hook: same advice as no hook.
	SetOCR(func([]byte) (string, error) { return "", ocrDown{} })
	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || !secs[0].ScanSkipped || secs[0].ScanFailed {
		t.Errorf("unavailable OCR must mark ScanSkipped, got %+v", secs)
	}

	// A model that errors, or reads nothing, is a failure to retry.
	SetOCR(func([]byte) (string, error) { return "", errors.New("boom") })
	if secs, _ = Extract(path); len(secs) != 1 || !secs[0].ScanFailed || secs[0].ScanSkipped {
		t.Errorf("erroring OCR must mark ScanFailed, got %+v", secs)
	}
	SetOCR(func([]byte) (string, error) { return "   ", nil })
	if secs, _ = Extract(path); len(secs) != 1 || !secs[0].ScanFailed {
		t.Errorf("blank transcription must mark ScanFailed, got %+v", secs)
	}

	// A working model clears both marks and carries text.
	SetOCR(func([]byte) (string, error) { return "SECURITY DEPOSIT $2,000", nil })
	secs, err = Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || secs[0].ScanSkipped || secs[0].ScanFailed || secs[0].Text == "" {
		t.Errorf("successful OCR must clear the marks and carry text, got %+v", secs)
	}
}
