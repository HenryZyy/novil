package api

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"novil/backend/internal/cover"
	"novil/backend/internal/store"
)

// maxCoverUploadBytes limits uploaded cover images.
const maxCoverUploadBytes = 20 << 20

// getCover sends a book's cover as a JPEG: the one an admin uploaded, or else
// one made from the file (first page of a CBZ or PDF, an EPUB's cover image).
func (s *Server) getCover(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	data, etag, err := s.coverFor(b)
	if errors.Is(err, cover.ErrNoCover) {
		writeError(w, http.StatusNotFound, "this book has no cover")
		return
	}
	if errors.Is(err, cover.ErrUnavailable) {
		slog.Warn("can't make cover now", "book", b.ID, "err", err)
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "the cover isn't ready yet, try again shortly")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	// The browser may keep the image but must ask whether it's still
	// current; the ETag changes with the cover, so an unchanged one costs
	// only a 304 Not Modified.
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// coverFor returns a book's cover and an ETag for it.
func (s *Server) coverFor(b store.Book) ([]byte, string, error) {
	if b.CustomCover {
		data, updated, err := s.store.CustomCover(b.ID)
		if err == nil {
			return data, `"custom-` + updated + `"`, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return nil, "", err
		}
	}
	etag := `"file-` + b.FileKey + `"`
	data, found, err := s.store.CachedCover(b.ID, b.FileKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		data, err = s.opts.Covers.FromBook(b.Path, b.Format)
		if errors.Is(err, cover.ErrUnavailable) {
			return nil, "", err
		}
		if err != nil {
			// Remember that this file has no usable cover, until it changes.
			if !errors.Is(err, cover.ErrNoCover) {
				slog.Warn("can't make cover from book file", "book", b.ID, "path", b.Path, "err", err)
			}
			data = nil
		}
		if err := s.store.SetCachedCover(b.ID, b.FileKey, data); err != nil {
			return nil, "", err
		}
	}
	if data == nil {
		return nil, "", cover.ErrNoCover
	}
	return data, etag, nil
}

// setCover replaces a book's cover with the uploaded image (the raw file as
// the request body). JPEG, PNG, GIF and WebP work.
func (s *Server) setCover(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	data, err := cover.FromUpload(http.MaxBytesReader(w, r.Body, maxCoverUploadBytes))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "the image is too large")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, "the cover must be a JPEG, PNG, GIF or WebP image of at most 50 megapixels")
		return
	}
	if err := s.store.SetCustomCover(b.ID, data); err != nil {
		internalError(w, err)
		return
	}
	b, err = s.store.GetBook(currentUser(r).ID, b.ID)
	respond(w, b, err)
}

// deleteCover goes back to the cover made from the book's file.
func (s *Server) deleteCover(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteCustomCover(b.ID); err != nil {
		internalError(w, err)
		return
	}
	b, err := s.store.GetBook(currentUser(r).ID, b.ID)
	respond(w, b, err)
}
