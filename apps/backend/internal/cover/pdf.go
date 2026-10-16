package cover

import (
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
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
		// Two workers, so a cover being made doesn't hold up someone
		// reading; each can use a lot of memory on large PDFs.
		p.pool, p.err = webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 2})
	})
	return p.err
}

// withDocument opens the PDF at path in a PDFium instance for fn.
func (p *pdfRenderer) withDocument(path string, fn func(pdfium.Pdfium, *responses.OpenDocument) error) error {
	if err := p.start(); err != nil {
		return fmt.Errorf("%w: start pdfium: %v", ErrUnavailable, err)
	}
	// Waits while other PDFs are being rendered.
	instance, err := p.pool.GetInstance(time.Minute)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer instance.Close()

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// A reader instead of the whole file, so large PDFs aren't read into
	// memory.
	doc, err := instance.OpenDocument(&requests.OpenDocument{FileReader: f, FileReaderSize: info.Size()})
	if err != nil {
		return err
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	return fn(instance, doc)
}

// page renders page index (from 0) of a PDF, fitting it in maxW x maxH.
func (p *pdfRenderer) page(path string, index, maxW, maxH int) (image.Image, error) {
	var out *image.RGBA
	err := p.withDocument(path, func(instance pdfium.Pdfium, doc *responses.OpenDocument) error {
		render, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{
			Page:   requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: index}},
			Width:  maxW,
			Height: maxH,
		})
		if err != nil {
			return err
		}
		// The pixels live in the WebAssembly memory until Cleanup, so copy
		// them out first.
		defer render.Cleanup()
		src := render.Result.RenderedImage
		out = image.NewRGBA(src.Bounds())
		draw.Draw(out, out.Bounds(), src, src.Bounds().Min, draw.Src)
		return nil
	})
	return out, err
}

func (p *pdfRenderer) firstPage(path string) (image.Image, error) {
	return p.page(path, 0, MaxWidth, MaxHeight)
}

func (p *pdfRenderer) pageCount(path string) (int, error) {
	var n int
	err := p.withDocument(path, func(instance pdfium.Pdfium, doc *responses.OpenDocument) error {
		res, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
		if err == nil {
			n = res.PageCount
		}
		return err
	})
	return n, err
}

func (p *pdfRenderer) close() error {
	if p.pool == nil {
		return nil
	}
	return p.pool.Close()
}
