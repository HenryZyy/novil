// Package cover makes cover images: from a book's first page or embedded
// cover, or from an image an admin uploads. Every cover comes out as a JPEG
// of at most MaxWidth x MaxHeight.
package cover

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"

	// Register the formats image.Decode understands.
	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"golang.org/x/image/draw"

	"novil/backend/internal/archive"
	"novil/backend/internal/metadata"
)

const (
	MaxWidth  = 800
	MaxHeight = 1200
	quality   = 85
	// maxPixels guards against "decompression bombs": tiny files that
	// claim to be huge images and would use gigabytes of memory.
	maxPixels = 50_000_000
)

var (
	ErrNoCover    = errors.New("no cover")
	ErrNotAnImage = errors.New("not a JPEG, PNG, GIF or WebP image")
	// ErrUnavailable means the cover can't be made right now, e.g. because
	// the PDF renderer is busy. Unlike other errors it says nothing about
	// the file, so trying again later may work.
	ErrUnavailable = errors.New("cover renderer unavailable")
)

// Generator makes covers from book files.
type Generator struct {
	pdf *pdfRenderer
}

func NewGenerator() *Generator {
	return &Generator{pdf: &pdfRenderer{}}
}

// Close frees the PDF renderer, if it was started.
func (g *Generator) Close() error {
	return g.pdf.close()
}

// FromBook makes a cover from the book file at path: the first page of a
// CBZ or PDF, or an EPUB's cover image. It returns ErrNoCover if the file
// has nothing usable.
func (g *Generator) FromBook(path, format string) ([]byte, error) {
	var img image.Image
	var err error
	switch format {
	case "cbz":
		img, err = cbzCover(path)
	case "epub":
		img, err = epubCover(path)
	case "pdf":
		img, err = g.pdf.firstPage(path)
	default:
		return nil, ErrNoCover
	}
	if err != nil {
		return nil, err
	}
	return encode(img)
}

// FromUpload checks that r is an image and turns it into a cover.
// Re-encoding also drops anything else the file might carry.
func FromUpload(r io.Reader) ([]byte, error) {
	img, err := decode(r)
	if err != nil {
		return nil, err
	}
	return encode(img)
}

func cbzCover(path string) (image.Image, error) {
	c, err := archive.OpenCBZ(path)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if c.PageCount() == 0 {
		return nil, ErrNoCover
	}
	page, _, err := c.Page(1)
	if err != nil {
		return nil, err
	}
	defer page.Close()
	return decode(page)
}

func epubCover(path string) (image.Image, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	name, err := metadata.EPUBCover(&zr.Reader)
	if errors.Is(err, metadata.ErrNoCover) {
		return nil, ErrNoCover
	}
	if err != nil {
		return nil, err
	}
	f, err := zr.Open(name)
	if err != nil {
		return nil, fmt.Errorf("cover %s: %w", name, err)
	}
	defer f.Close()
	return decode(f)
}

// decode reads an image, refusing ones too large to decode safely.
func decode(r io.Reader) (image.Image, error) {
	// DecodeConfig only reads the header, so keep a copy of what it reads
	// for the real decode.
	var head bytes.Buffer
	cfg, _, err := image.DecodeConfig(io.TeeReader(r, &head))
	if err != nil {
		return nil, ErrNotAnImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("image is %dx%d, too large", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(io.MultiReader(&head, r))
	if err != nil {
		return nil, ErrNotAnImage
	}
	return img, nil
}

// encode scales img down to fit MaxWidth x MaxHeight and returns it as a
// JPEG. Transparent parts become white, since JPEG has no transparency.
func encode(img image.Image) ([]byte, error) {
	b := img.Bounds()
	w, h := fit(b.Dx(), b.Dy(), MaxWidth, MaxHeight)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fit returns w x h scaled down, keeping its aspect ratio, to fit in
// maxW x maxH. Smaller images keep their size.
func fit(w, h, maxW, maxH int) (int, int) {
	if w <= maxW && h <= maxH {
		return w, h
	}
	if w*maxH > h*maxW {
		return maxW, max(1, h*maxW/w)
	}
	return max(1, w*maxH/h), maxH
}
