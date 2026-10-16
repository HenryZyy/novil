package api

import (
	"errors"
	"log/slog"
	"net/http"

	"novil/backend/internal/metadata"
)

// Only plain image formats: an SVG cover could carry scripts.
var coverTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	"image/avif": true,
}

// getCover serves a book's cover: the first page of a comic, or the cover
// image inside an EPUB.
func (s *Server) getCover(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	switch b.Format {
	case "cbz":
		if b.PageCount == 0 {
			writeError(w, http.StatusNotFound, "this book has no cover")
			return
		}
		writeCBZPage(w, b, 1)
	case "epub":
		data, mediaType, err := metadata.EPUBCover(b.Path)
		// A broken file has no cover to show either; the book list still works.
		if err != nil && !errors.Is(err, metadata.ErrNoCover) {
			slog.Warn("can't read epub cover", "path", b.Path, "err", err)
		}
		if err != nil || !coverTypes[mediaType] {
			writeError(w, http.StatusNotFound, "this book has no cover")
			return
		}
		w.Header().Set("Content-Type", mediaType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=86400")
		w.Write(data)
	default:
		writeError(w, http.StatusNotFound, "this book has no cover")
	}
}
