package ingest

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// PDF extraction runs PDFium (Chrome's PDF engine) compiled to WebAssembly
// under wazero: production-grade text extraction with zero CGo, so plain
// GOOS/GOARCH cross-compilation keeps working.
//
// The runtime instantiates lazily — commands that never touch a PDF don't
// pay the WASM startup cost — and is shared for the process lifetime.
var (
	pdfOnce sync.Once
	pdfPool pdfium.Pool
	pdfErr  error
)

func pdfInstance() (pdfium.Pdfium, error) {
	pdfOnce.Do(func() {
		// One warm instance for today's serial walk; room to grow for the
		// M3 worker pool without another instantiation stampede.
		pdfPool, pdfErr = webassembly.Init(webassembly.Config{
			MinIdle:  1,
			MaxIdle:  2,
			MaxTotal: runtime.GOMAXPROCS(0),
		})
	})
	if pdfErr != nil {
		return nil, fmt.Errorf("starting PDF engine: %w", pdfErr)
	}
	return pdfPool.GetInstance(30 * time.Second)
}

// ocrPage, when set, transcribes a rendered page image. It is the bridge
// to whatever vision-capable LLM the user runs; ingest itself stays free
// of LLM dependencies. Set it before indexing starts.
var ocrPage func(pngImage []byte) (string, error)

// ocrUnavailable detects "there is no vision endpoint" errors from the
// hook by shape, keeping ingest free of an llm import.
func ocrUnavailable(err error) bool {
	var u interface{ OCRUnavailable() bool }
	return errors.As(err, &u) && u.OCRUnavailable()
}

// SetOCR installs the transcriber used for pages with no extractable
// text (scanned PDFs). A nil fn — or never calling this — means such
// pages index empty, exactly as before.
func SetOCR(fn func(pngImage []byte) (string, error)) { ocrPage = fn }

// ocrDPI keeps a US-letter page around 1300×1700 px: enough for a vision
// model to read body text, small enough to encode and send quickly.
const ocrDPI = 150

// extractPDF pulls text page by page — the page number is the citation.
// Layout-aware assembly rebuilds paragraphs from the page geometry;
// pages with no text at all fall through to OCR when it is configured.
func extractPDF(path string) ([]Section, error) {
	inst, err := pdfInstance()
	if err != nil {
		return nil, err
	}
	defer inst.Close()

	// The document goes in as bytes, never as a path: the WASM sandbox
	// has its own filesystem view and cannot resolve OS paths (Windows
	// volumes in particular). Reading here keeps extraction portable.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening PDF: %w", err)
	}
	doc, err := inst.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return nil, fmt.Errorf("opening PDF: %w", err)
	}
	defer inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	count, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, fmt.Errorf("counting pages: %w", err)
	}

	var sections []Section
	for i := 0; i < count.PageCount; i++ {
		page := requests.Page{
			ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i},
		}

		text, err := pageText(inst, page)
		if err != nil {
			return nil, fmt.Errorf("reading page %d: %w", i+1, err)
		}
		if strings.TrimSpace(text) == "" && pageHasImage(inst, page) {
			// No text but pixels: a scanned page. Best effort, and the
			// misses are marked so the caller can say "these pages
			// indexed empty" instead of staying silent about it.
			if ocrPage == nil {
				sections = append(sections, Section{Page: i + 1, ScanSkipped: true})
				continue
			}
			t, err := transcribePage(inst, page)
			switch {
			case err != nil && ocrUnavailable(err):
				sections = append(sections, Section{Page: i + 1, ScanSkipped: true})
				continue
			case err != nil || strings.TrimSpace(t) == "":
				sections = append(sections, Section{Page: i + 1, ScanFailed: true})
				continue
			}
			text = t
		}
		sections = append(sections, Section{Text: text, Page: i + 1})
	}
	return sections, nil
}

// pageText extracts one page's text, structured when possible. The rect
// walk can fail on exotic PDFs where plain extraction still works, so
// the flat text is the fallback, not an error.
func pageText(inst pdfium.Pdfium, page requests.Page) (string, error) {
	structured, serr := inst.GetPageTextStructured(&requests.GetPageTextStructured{
		Page:                   page,
		Mode:                   requests.GetPageTextStructuredModeRects,
		CollectFontInformation: true,
	})
	if serr == nil {
		runs := make([]textRun, 0, len(structured.Rects))
		for _, r := range structured.Rects {
			if r == nil {
				continue
			}
			run := textRun{
				text:   r.Text,
				left:   r.PointPosition.Left,
				right:  r.PointPosition.Right,
				top:    r.PointPosition.Top,
				bottom: r.PointPosition.Bottom,
			}
			if fi := r.FontInformation; fi != nil {
				run.fontName = fi.Name
				run.fontSize = fi.RenderedSize
				if run.fontSize == 0 {
					run.fontSize = fi.Size
				}
			}
			runs = append(runs, run)
		}
		if text := assemblePage(runs); strings.TrimSpace(text) != "" {
			return text, nil
		}
	}

	plain, perr := inst.GetPageText(&requests.GetPageText{Page: page})
	if perr != nil {
		if serr != nil {
			return "", serr
		}
		return "", perr
	}
	return plain.Text, nil
}

// pageHasImage reports whether the page places at least one image — the
// cheap test that separates a scanned page from a genuinely blank one
// before spending an OCR call.
func pageHasImage(inst pdfium.Pdfium, page requests.Page) bool {
	count, err := inst.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: page})
	if err != nil {
		return false
	}
	for i := 0; i < count.Count; i++ {
		obj, err := inst.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: page, Index: i})
		if err != nil {
			continue
		}
		typ, err := inst.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: obj.PageObject})
		if err == nil && typ.Type == enums.FPDF_PAGEOBJ_IMAGE {
			return true
		}
	}
	return false
}

// transcribePage renders the page and hands the image to the configured
// OCR transcriber.
func transcribePage(inst pdfium.Pdfium, page requests.Page) (string, error) {
	rendered, err := inst.RenderPageInDPI(&requests.RenderPageInDPI{Page: page, DPI: ocrDPI})
	if err != nil {
		return "", err
	}
	defer rendered.Cleanup()

	var buf bytes.Buffer
	if err := png.Encode(&buf, rendered.Result.Image); err != nil {
		return "", err
	}
	return ocrPage(buf.Bytes())
}
