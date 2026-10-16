package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"novil/backend/internal/store"
)

// Highlight colours; the frontend picks how each looks.
var annotationColors = []string{"yellow", "green", "blue", "pink", "purple"}

const (
	maxAnnotationText = 2000
	maxNoteLength     = 10000
)

func (s *Server) setListed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	list := r.PathValue("list")
	if !slices.Contains(store.Lists, list) {
		writeError(w, http.StatusNotFound, "list must be favorites or want")
		return
	}
	userID := currentUser(r).ID
	if _, err := s.store.GetSeries(userID, id); err != nil {
		respond(w, nil, err)
		return
	}
	if err := s.store.SetListed(userID, id, list, r.Method == http.MethodPut); err != nil {
		internalError(w, err)
		return
	}
	sr, err := s.store.GetSeries(userID, id)
	respond(w, sr, err)
}

func (s *Server) listAnnotations(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	list, err := s.store.ListAnnotations(currentUser(r).ID, b.ID)
	respond(w, list, err)
}

// createAnnotation adds a bookmark or highlight to a book:
//
//	{"kind": "highlight", "page": 12, "position": "epubcfi(…)", "text": "…", "note": "", "color": "yellow"}
//
// Comics only get bookmarks, on a page; EPUBs need a position.
func (s *Server) createAnnotation(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	var a store.Annotation
	if err := decodeJSON(w, r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a.BookID = b.ID
	a.Text = strings.TrimSpace(a.Text)
	// The text is a quote shown in lists; a long one is cut, not refused.
	if utf8.RuneCountInString(a.Text) > maxAnnotationText {
		a.Text = string([]rune(a.Text)[:maxAnnotationText]) + "…"
	}
	if err := checkAnnotation(b, a); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.CreateAnnotation(currentUser(r).ID, a)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func checkAnnotation(b store.Book, a store.Annotation) error {
	epub := b.Format == "epub"
	maxPage := b.PageCount
	if epub {
		maxPage = 100
	}
	switch {
	case !epub && b.PageCount == 0:
		return errors.New("this book can't be annotated")
	case a.Kind != "bookmark" && a.Kind != "highlight":
		return errors.New("kind must be bookmark or highlight")
	case a.Kind == "highlight" && !epub:
		return errors.New("only EPUBs have highlights")
	case a.Page < 0 || a.Page > maxPage || (!epub && a.Page == 0):
		return errors.New("page out of range")
	case epub && a.Position == "":
		return errors.New("position is required for EPUBs")
	case len(a.Position) > 2000:
		return errors.New("position is too long")
	case utf8.RuneCountInString(a.Note) > maxNoteLength:
		return errors.New("note is too long")
	case a.Color != "" && !slices.Contains(annotationColors, a.Color):
		return errors.New("color must be " + strings.Join(annotationColors, ", ") + " or empty")
	}
	return nil
}

// updateAnnotation changes the note or colour: {"note": "…", "color": "blue"}.
func (s *Server) updateAnnotation(w http.ResponseWriter, r *http.Request) {
	a, ok := s.annotation(w, r)
	if !ok {
		return
	}
	var req struct {
		Note  *string `json:"note"`
		Color *string `json:"color"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Note == nil && req.Color == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if req.Note != nil && utf8.RuneCountInString(*req.Note) > maxNoteLength {
		writeError(w, http.StatusBadRequest, "note is too long")
		return
	}
	if req.Color != nil && *req.Color != "" && !slices.Contains(annotationColors, *req.Color) {
		writeError(w, http.StatusBadRequest, "color must be "+strings.Join(annotationColors, ", ")+" or empty")
		return
	}
	a, err := s.store.UpdateAnnotation(currentUser(r).ID, a.ID, req.Note, req.Color)
	respond(w, a, err)
}

func (s *Server) deleteAnnotation(w http.ResponseWriter, r *http.Request) {
	a, ok := s.annotation(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAnnotation(currentUser(r).ID, a.ID); err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// annotation returns the user's annotation from the path, if its book is
// still visible to them.
func (s *Server) annotation(w http.ResponseWriter, r *http.Request) (store.Annotation, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return store.Annotation{}, false
	}
	userID := currentUser(r).ID
	a, err := s.store.GetAnnotation(userID, id)
	if err == nil {
		_, err = s.store.GetBook(userID, a.BookID)
	}
	if err != nil {
		respond(w, nil, err)
		return store.Annotation{}, false
	}
	return a, true
}
