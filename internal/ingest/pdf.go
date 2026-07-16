package ingest

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
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

// extractPDF pulls text page by page — the page number is the citation.
func extractPDF(path string) ([]Section, error) {
	inst, err := pdfInstance()
	if err != nil {
		return nil, err
	}
	defer inst.Close()

	doc, err := inst.OpenDocument(&requests.OpenDocument{FilePath: &path})
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
		text, err := inst.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("reading page %d: %w", i+1, err)
		}
		sections = append(sections, Section{Text: text.Text, Page: i + 1})
	}
	return sections, nil
}
