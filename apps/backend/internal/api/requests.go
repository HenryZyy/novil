package api

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"novil/backend/internal/isbn"
	"novil/backend/internal/store"
)

// limitLookups keeps ISBN lookups, which call other services, to a few
// per IP.
func (s *Server) limitLookups(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d := s.lookupLimiter.wait(clientKey(clientIP(r, s.trustedProxies))); d > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.Seconds()))))
			writeCodedError(w, http.StatusTooManyRequests, "too_many_requests", "too many lookups, try again in a moment")
			return
		}
		next(w, r)
	}
}

// findBook returns what is known about an ISBN: the request for it, if
// someone asked already, or else what the lookup finds. It writes the
// error response itself and returns false then.
func (s *Server) findBook(w http.ResponseWriter, r *http.Request, raw string) (string, *store.BookRequest, isbn.Info, bool) {
	code, err := isbn.Normalize(raw)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "invalid_isbn", "this isn't a valid ISBN; check for typos")
		return "", nil, isbn.Info{}, false
	}
	req, err := s.store.GetRequestByISBN(currentUser(r).ID, code)
	if err == nil {
		return code, &req, isbn.Info{}, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		internalError(w, err)
		return "", nil, isbn.Info{}, false
	}
	if s.opts.LookupISBN == nil {
		writeCodedError(w, http.StatusNotFound, "isbn_not_found", isbn.ErrNotFound.Error())
		return "", nil, isbn.Info{}, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	info, err := s.opts.LookupISBN(ctx, code)
	switch {
	case errors.Is(err, isbn.ErrNotFound):
		writeCodedError(w, http.StatusNotFound, "isbn_not_found", "no book found for this ISBN")
		return "", nil, isbn.Info{}, false
	case err != nil:
		slog.Warn("ISBN lookup failed", "isbn", code, "err", err)
		writeCodedError(w, http.StatusBadGateway, "lookup_failed", "the book databases can't be reached right now, try again later")
		return "", nil, isbn.Info{}, false
	}
	return code, nil, info, true
}

// lookupISBN shows what a request for an ISBN would be:
// {"book": {isbn, title, …}, "request": {…} or null}.
func (s *Server) lookupISBN(w http.ResponseWriter, r *http.Request) {
	code, req, info, ok := s.findBook(w, r, r.PathValue("isbn"))
	if !ok {
		return
	}
	if req != nil {
		info = isbn.Info{ISBN: code, Title: req.Title, Authors: req.Authors, Published: req.Published, CoverURL: req.CoverURL}
	}
	writeJSON(w, http.StatusOK, map[string]any{"book": info, "request": s.forViewer(r, req)})
}

// requestBook asks for a book by ISBN: {"isbn": "978-…"}. Asking for a
// book someone else asked for joins their request.
func (s *Server) requestBook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ISBN string `json:"isbn"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	code, existing, info, ok := s.findBook(w, r, body.ISBN)
	if !ok {
		return
	}
	if existing != nil {
		switch {
		case existing.RequestedByMe:
			writeCodedError(w, http.StatusConflict, "already_requested", "you already requested this book")
			return
		case existing.Status == "added":
			writeCodedError(w, http.StatusConflict, "already_added", "this book is already in the library")
			return
		}
	}
	req := store.BookRequest{ISBN: code, Title: info.Title, Authors: info.Authors, Published: info.Published, CoverURL: info.CoverURL}
	created, err := s.store.Request(currentUser(r).ID, req)
	if errors.Is(err, store.ErrAlreadyRequested) {
		writeCodedError(w, http.StatusConflict, "already_requested", "you already requested this book")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	status := http.StatusCreated
	if existing != nil {
		status = http.StatusOK
	}
	writeJSON(w, status, s.forViewer(r, &created))
}

// forViewer leaves out who else asked, which only admins see.
func (s *Server) forViewer(r *http.Request, req *store.BookRequest) *store.BookRequest {
	if req == nil || currentUser(r).Role == store.RoleAdmin {
		return req
	}
	shown := *req
	shown.Requesters = nil
	return &shown
}

func (s *Server) myRequests(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.MyRequests(currentUser(r).ID)
	for i := range list {
		list[i] = *s.forViewer(r, &list[i])
	}
	respond(w, list, err)
}

func (s *Server) withdrawRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.Withdraw(currentUser(r).ID, id); err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	switch status {
	case "", "open", "added", "declined":
	default:
		writeError(w, http.StatusBadRequest, "status must be open, added or declined")
		return
	}
	list, err := s.store.ListRequests(currentUser(r).ID, status)
	respond(w, list, err)
}

// updateRequest lets admins answer a request: {"status": "added",
// "bookId": 12}, {"status": "declined"} or back to {"status": "open"}.
func (s *Server) updateRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
		BookID *int64 `json:"bookId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	switch body.Status {
	case "open", "declined":
		body.BookID = nil
	case "added":
		if body.BookID != nil {
			if _, err := s.store.GetBook(currentUser(r).ID, *body.BookID); err != nil {
				writeError(w, http.StatusBadRequest, "bookId is not a book")
				return
			}
		}
	default:
		writeError(w, http.StatusBadRequest, "status must be open, added or declined")
		return
	}
	if err := s.store.SetRequestStatus(id, body.Status, body.BookID); err != nil {
		respond(w, nil, err)
		return
	}
	req, err := s.store.GetRequest(currentUser(r).ID, id)
	if err == nil {
		s.audit(r, "request_updated", actor(r), "", req.Title+": "+req.Status)
	}
	respond(w, req, err)
}

func (s *Server) deleteRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteRequest(id); err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
