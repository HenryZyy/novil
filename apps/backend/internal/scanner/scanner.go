// Package scanner finds books in a library. Each folder is a series.
package scanner

import (
	"errors"
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

func Scan(root string) ([]Series, error) {
	byPath, err := walk(root, root)
	if err != nil {
		return nil, err
	}
	out := make([]Series, 0, len(byPath))
	for _, s := range byPath {
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Series) int { return natsort.Compare(a.Name, b.Name) })
	return out, nil
}

func ScanSeries(root, dir string) (Series, error) {
	byPath, err := walk(root, dir)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return Series{}, err
	}
	if s := byPath[dir]; s != nil {
		return *s, nil
	}
	return Series{Name: filepath.Base(dir), Path: dir}, nil
}

func walk(root, start string) (map[string]*Series, error) {
	byPath := map[string]*Series{}
	err := filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == start {
				return err
			}
			slog.Warn("skipping unreadable path", "path", p, "err", err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if p != start && strings.HasPrefix(d.Name(), ".") {
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
	for _, s := range byPath {
		slices.SortFunc(s.Books, func(a, b Book) int { return natsort.Compare(a.Path, b.Path) })
	}
	return byPath, err
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
