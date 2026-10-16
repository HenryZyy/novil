package api

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// deleteBook deletes a book's file from its library folder and the book
// from the database, with everyone's progress and notes in it. Only the
// file goes, so the next scan doesn't bring it back; an emptied series
// folder goes with it.
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
	// Scans only find files inside the library, but a moved library folder
	// could leave old paths behind: never delete anything outside it.
	rel, err := filepath.Rel(lib.Path, b.Path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		writeError(w, http.StatusConflict, "the book's file is outside its library folder; rescan the library")
		return
	}
	if err := os.Remove(b.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		writeFileError(w, err)
		return
	}
	// Removing a folder only works when it's empty, which is what's wanted.
	if dir := filepath.Dir(b.Path); dir != filepath.Clean(lib.Path) {
		os.Remove(dir)
	}
	if err := s.store.DeleteBook(b.ID); err != nil {
		respond(w, nil, err)
		return
	}
	s.audit(r, "book_deleted", actor(r), "", b.SeriesName+" / "+filepath.Base(b.Path)+" from "+lib.Name)
	w.WriteHeader(http.StatusNoContent)
}
