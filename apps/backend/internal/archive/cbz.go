// Package archive reads pages out of comic archives without extracting them.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"novil/backend/internal/natsort"
)

var imageTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".avif": "image/avif",
}

// CBZ is an open zip-based comic archive. Pages are the image entries,
// sorted in natural order.
type CBZ struct {
	zr    *zip.ReadCloser
	pages []*zip.File
}

func OpenCBZ(path string) (*CBZ, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	var pages []*zip.File
	for _, f := range zr.File {
		if isPage(f) {
			pages = append(pages, f)
		}
	}
	slices.SortFunc(pages, func(a, b *zip.File) int { return natsort.Compare(a.Name, b.Name) })
	return &CBZ{zr: zr, pages: pages}, nil
}

func (c *CBZ) PageCount() int { return len(c.pages) }

// Page opens page n (1-indexed) and returns its content and MIME type.
func (c *CBZ) Page(n int) (io.ReadCloser, string, error) {
	if n < 1 || n > len(c.pages) {
		return nil, "", fmt.Errorf("page %d out of range (1-%d)", n, len(c.pages))
	}
	f := c.pages[n-1]
	rc, err := f.Open()
	if err != nil {
		return nil, "", err
	}
	return rc, imageTypes[strings.ToLower(path.Ext(f.Name))], nil
}

func (c *CBZ) Close() error { return c.zr.Close() }

func isPage(f *zip.File) bool {
	if f.FileInfo().IsDir() {
		return false
	}
	for _, part := range strings.Split(f.Name, "/") {
		if strings.HasPrefix(part, ".") || part == "__MACOSX" {
			return false
		}
	}
	_, ok := imageTypes[strings.ToLower(path.Ext(f.Name))]
	return ok
}
