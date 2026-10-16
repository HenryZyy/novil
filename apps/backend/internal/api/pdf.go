package api

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"novil/backend/internal/cover"
	"novil/backend/internal/store"
)

// pdfCounter counts the pages of PDFs, which scans leave to it because it
// takes PDFium. One run at a time; a run started meanwhile waits.
type pdfCounter struct {
	mu sync.Mutex
}

// countPDFPages counts the pages of every PDF not counted yet, in the
// background, after the server starts and after scans and uploads.
func (s *Server) countPDFPages() {
	go func() {
		s.pdfs.mu.Lock()
		defer s.pdfs.mu.Unlock()
		pdfs, err := s.store.PDFsWithoutPageCount()
		if err != nil {
			slog.Error("can't list PDFs to count", "err", err)
			return
		}
		for id, path := range pdfs {
			s.countPages(id, path)
		}
	}()
}

// countPages counts and stores the pages of one PDF; 0 if that fails.
func (s *Server) countPages(id int64, path string) int {
	n, err := s.opts.Covers.PDFPageCount(path)
	if err != nil {
		slog.Warn("can't count the pages of a PDF", "book", id, "path", path, "err", err)
		return 0
	}
	if err := s.store.SetPageCount(id, n); err != nil {
		slog.Error("can't store a PDF's page count", "book", id, "err", err)
	}
	return n
}

// withPages makes sure a PDF has its page count, counting it now if the
// background count hasn't got to it yet.
func (s *Server) withPages(b store.Book) store.Book {
	if b.Format == "pdf" && b.PageCount == 0 {
		b.PageCount = s.countPages(b.ID, b.Path)
	}
	return b
}

func (s *Server) writePDFPage(w http.ResponseWriter, b store.Book, n int) {
	page, err := s.opts.Covers.PDFPage(b.Path, n)
	if errors.Is(err, cover.ErrUnavailable) {
		w.Header().Set("Retry-After", "2")
		writeError(w, http.StatusServiceUnavailable, "the PDF renderer is busy, try again")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(page)
}
