package cover

import (
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"golang.org/x/image/draw"
)

// pdfRenderer draws PDF pages with PDFium, the PDF engine of Chrome. It runs
// as WebAssembly inside the Go program, so it needs no installed libraries
// and works in the static Docker image. Starting it takes a moment, so that
// happens on the first PDF, not at server start.
type pdfRenderer struct {
	once sync.Once
	pool pdfium.Pool
	err  error
}

func (p *pdfRenderer) start() error {
	p.once.Do(func() {
		// One worker: covers are made once and cached, and each worker can
		// use a lot of memory on large PDFs.
		p.pool, p.err = webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	})
	return p.err
}

// firstPage renders page 1 of a PDF, fitting it in MaxWidth x MaxHeight.
func (p *pdfRenderer) firstPage(path string) (image.Image, error) {
	if err := p.start(); err != nil {
		return nil, fmt.Errorf("%w: start pdfium: %v", ErrUnavailable, err)
	}
	// Waits while another PDF is being rendered.
	instance, err := p.pool.GetInstance(time.Minute)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer instance.Close()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// A reader instead of the whole file, so large PDFs aren't read into
	// memory.
	doc, err := instance.OpenDocument(&requests.OpenDocument{FileReader: f, FileReaderSize: info.Size()})
	if err != nil {
		return nil, err
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	render, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{
		Page:   requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0}},
		Width:  MaxWidth,
		Height: MaxHeight,
	})
	if err != nil {
		return nil, err
	}
	// The pixels live in the WebAssembly memory until Cleanup, so copy them
	// out first.
	defer render.Cleanup()
	src := render.Result.RenderedImage
	out := image.NewRGBA(src.Bounds())
	draw.Draw(out, out.Bounds(), src, src.Bounds().Min, draw.Src)
	return out, nil
}

func (p *pdfRenderer) close() error {
	if p.pool == nil {
		return nil
	}
	return p.pool.Close()
}
