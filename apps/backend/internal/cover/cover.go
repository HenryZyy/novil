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
	// claim to be huge images and would use gigabytes of memory. Avatars,
	// which every user can upload, get less.
	maxPixels       = 50_000_000
	maxAvatarPixels = 16_000_000
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

// The size PDF pages are read at: sharp on most screens, and small enough
// to send quickly.
const (
	PageWidth  = 1600
	PageHeight = 2400
)

// PDFPage renders page n (from 1) of the PDF at path for reading, as a JPEG.
func (g *Generator) PDFPage(path string, n int) ([]byte, error) {
	img, err := g.pdf.page(path, n-1, PageWidth, PageHeight)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	return scaleJPEG(img, b, b.Dx(), b.Dy())
}

// PDFPageCount returns how many pages the PDF at path has.
func (g *Generator) PDFPageCount(path string) (int, error) {
	return g.pdf.pageCount(path)
}

// FromBook makes a cover from the book file at path: the first page of a
// CBZ or PDF, or an EPUB's cover image. It returns ErrNoCover if the file
// has nothing usable.
func (g *Generator) FromBook(path, format string) ([]byte, error) {
	var img image.Image
	var err error
	switch format {
	case "cbz":
		defer decoding()()
		img, err = cbzCover(path)
	case "epub":
		defer decoding()()
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
	defer decoding()()
	img, err := decode(r, maxPixels)
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
	return decode(page, maxPixels)
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
	return decode(f, maxPixels)
}

// decode reads an image, refusing ones too large to decode safely.
// slots allows two images in memory at once, however many requests decode
// them: a large image can take hundreds of MB once decoded.
var slots = make(chan struct{}, 2)

// decoding waits for a slot and returns the function that frees it.
func decoding() func() {
	slots <- struct{}{}
	return func() { <-slots }
}

func decode(r io.Reader, limit int) (image.Image, error) {
	// DecodeConfig only reads the header, so keep a copy of what it reads
	// for the real decode.
	var head bytes.Buffer
	cfg, _, err := image.DecodeConfig(io.TeeReader(r, &head))
	if err != nil {
		return nil, ErrNotAnImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > limit {
		return nil, fmt.Errorf("image is %dx%d, too large", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(io.MultiReader(&head, r))
	if err != nil {
		return nil, ErrNotAnImage
	}
	return img, nil
}

// encode scales img down to fit MaxWidth x MaxHeight and returns it as a
// JPEG.
func encode(img image.Image) ([]byte, error) {
	b := img.Bounds()
	w, h := fit(b.Dx(), b.Dy(), MaxWidth, MaxHeight)
	return scaleJPEG(img, b, w, h)
}

// AvatarSize is the width and height of profile pictures.
const AvatarSize = 256

// AvatarFromUpload checks that r is an image and turns it into a square
// profile picture: the middle of the image, at most AvatarSize wide.
func AvatarFromUpload(r io.Reader) ([]byte, error) {
	defer decoding()()
	img, err := decode(r, maxAvatarPixels)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x, y := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	size := min(side, AvatarSize)
	return scaleJPEG(img, image.Rect(x, y, x+side, y+side), size, size)
}

// scaleJPEG scales the part src of img to w x h and returns it as a JPEG.
// Transparent parts become white, since JPEG has no transparency.
func scaleJPEG(img image.Image, src image.Rectangle, w, h int) ([]byte, error) {
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(out, out.Bounds(), img, src, draw.Over, nil)
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
