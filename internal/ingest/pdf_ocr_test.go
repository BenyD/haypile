package ingest

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// writeImageOnlyPDF builds the smallest honest scanned PDF: one page,
// one JPEG, zero text objects. Generated instead of checked in so the
// fixture never rots.
func writeImageOnlyPDF(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = 0xEE
	}
	img.Set(4, 4, color.Black)
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}

	content := "q 200 0 0 200 0 0 cm /Im0 Do Q"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] " +
			"/Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 16 /Height 16 "+
			"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode "+
			"/Length %d >>\nstream\n%s\nendstream", jpg.Len(), jpg.String()),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objects)+1, xref)

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractPDFOCRsImageOnlyPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)

	var gotImage []byte
	SetOCR(func(pngImage []byte) (string, error) {
		gotImage = pngImage
		return "Scanned deed of sale", nil
	})
	t.Cleanup(func() { SetOCR(nil) })

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 {
		t.Fatalf("got %d sections, want 1", len(secs))
	}
	if secs[0].Text != "Scanned deed of sale" || secs[0].Page != 1 {
		t.Errorf("section = %+v, want the transcription on page 1", secs[0])
	}
	if !bytes.HasPrefix(gotImage, []byte("\x89PNG")) {
		t.Errorf("transcriber did not receive a PNG (got %d bytes)", len(gotImage))
	}
}

func TestExtractPDFImagePageWithoutOCRStaysEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.pdf")
	writeImageOnlyPDF(t, path)

	secs, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(secs) != 1 || secs[0].Text != "" {
		t.Errorf("without OCR the page must index empty, got %+v", secs)
	}
}
