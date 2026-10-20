package api

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"novil/backend/internal/store"
)

var errOutsideLibrary = errors.New("the book's file is outside its library folder; rescan the library")

// insideLibrary reports whether a book's file is in its library's folder.
// Scans only find files there, but a moved library folder could leave old
// paths behind, and a linked folder could point elsewhere: nothing outside
// it is ever deleted. A file already gone counts as inside.
func insideLibrary(lib store.Library, b store.Book) bool {
	rel, err := filepath.Rel(lib.Path, b.Path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return false
	}
	root, err := filepath.EvalSymlinks(lib.Path)
	if err != nil {
		return false
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(b.Path))
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	rel, err = filepath.Rel(root, dir)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

// removeBook deletes a book's file from its library folder and the book
// from the database, with everyone's progress and notes in it. Only the
// file goes, so the next scan doesn't bring it back; an emptied series
// folder goes with it.
func (s *Server) removeBook(lib store.Library, b store.Book) error {
	if !insideLibrary(lib, b) {
		return errOutsideLibrary
	}
	if err := os.Remove(b.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// Removing a folder only works when it's empty, which is what's wanted.
	if dir := filepath.Dir(b.Path); dir != filepath.Clean(lib.Path) {
		os.Remove(dir)
	}
	return s.store.DeleteBook(b.ID)
}

func (s *Server) deleteBook(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	lib, err := s.store.BookLibrary(b.ID)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if err := s.removeBook(lib, b); err != nil {
		writeDeleteError(w, err)
		return
	}
	s.audit(r, "book_deleted", actor(r), "", b.SeriesName+" / "+filepath.Base(b.Path)+" from "+lib.Name)
	w.WriteHeader(http.StatusNoContent)
}

// deleteSeries deletes every book of a series like deleteBook, and with
// the last one the series.
func (s *Server) deleteSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID := currentUser(r).ID
	sr, err := s.store.GetSeries(userID, id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	lib, err := s.store.GetLibrary(sr.LibraryID)
	if err != nil {
		respond(w, nil, err)
		return
	}
	books, err := s.store.ListBooks(userID, id)
	if err != nil {
		internalError(w, err)
		return
	}
	// Checked for all first, so a problem leaves the series whole.
	for _, b := range books {
		if !insideLibrary(lib, b) {
			writeDeleteError(w, errOutsideLibrary)
			return
		}
	}
	for i, b := range books {
		if err := s.removeBook(lib, b); err != nil {
			if i > 0 {
				s.audit(r, "series_deleted", actor(r), "", fmt.Sprintf("%s from %s, only %d of %d books", sr.Name, lib.Name, i, len(books)))
			}
			writeDeleteError(w, err)
			return
		}
	}
	s.audit(r, "series_deleted", actor(r), "", fmt.Sprintf("%s (%d books) from %s", sr.Name, len(books), lib.Name))
	w.WriteHeader(http.StatusNoContent)
}

func writeDeleteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errOutsideLibrary):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeFileError(w, err)
	}
}
