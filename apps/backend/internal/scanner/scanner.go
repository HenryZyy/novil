// Package scanner finds the series and books in a library folder.
// Each folder in the library is a series, and the files inside it are its
// books. A file directly in the library folder is a series on its own.
package scanner

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"novil/backend/internal/archive"
	"novil/backend/internal/natsort"
)

var Formats = map[string]string{
	".cbz":  "cbz",
	".epub": "epub",
	".pdf":  "pdf",
}

type Book struct {
	Path      string
	Title     string
	Format    string
	Size      int64
	ModTime   time.Time
	PageCount int
}

type Series struct {
	Name  string
	Path  string
	Books []Book
}

// Scan returns the series in root, sorted by name.
// Files that can't be read get skipped
func Scan(root string) ([]Series, error) {
	byPath := map[string]*Series{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			slog.Warn("skipping unreadable path", "path", p, "err", err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		format, ok := Formats[strings.ToLower(filepath.Ext(p))]
		if !ok {
			return nil
		}
		book, err := readBook(p, d, format)
		if err != nil {
			slog.Warn("skipping unreadable book", "path", p, "err", err)
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		top, _, nested := strings.Cut(rel, string(filepath.Separator))
		seriesPath, seriesName := p, book.Title
		if nested {
			seriesPath, seriesName = filepath.Join(root, top), top
		}
		s := byPath[seriesPath]
		if s == nil {
			s = &Series{Name: seriesName, Path: seriesPath}
			byPath[seriesPath] = s
		}
		s.Books = append(s.Books, book)
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]Series, 0, len(byPath))
	for _, s := range byPath {
		slices.SortFunc(s.Books, func(a, b Book) int { return natsort.Compare(a.Path, b.Path) })
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Series) int { return natsort.Compare(a.Name, b.Name) })
	return out, nil
}

func readBook(p string, d fs.DirEntry, format string) (Book, error) {
	info, err := d.Info()
	if err != nil {
		return Book{}, err
	}
	b := Book{
		Path:    p,
		Title:   strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
		Format:  format,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if format == "cbz" {
		c, err := archive.OpenCBZ(p)
		if err != nil {
			return Book{}, err
		}
		b.PageCount = c.PageCount()
		c.Close()
	}
	return b, nil
}
