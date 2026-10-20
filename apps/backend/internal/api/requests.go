package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"novil/backend/internal/cover"
	"novil/backend/internal/isbn"
	"novil/backend/internal/store"
)

// limitLookups keeps ISBN lookups, which call other services, to a few
// per IP.
func (s *Server) limitLookups(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d := s.lookupLimiter.wait(clientKey(clientIP(r, s.trustedProxies))); d > 0 {
			tooMany(w, d, "too many lookups, try again in a moment")
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
	info, err := s.lookup(ctx, code)
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
	user := currentUser(r)
	if user.Role != store.RoleAdmin {
		if wait, msg, err := s.requestCooldown(user.ID); err != nil {
			internalError(w, err)
			return
		} else if wait > 0 {
			tooMany(w, wait, msg)
			return
		}
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
	} else if created.CoverURL != "" {
		go s.saveRequestCover(created.ID, created.CoverURL)
	}
	writeJSON(w, status, s.forViewer(r, &created))
}

// forViewer leaves out who else asked, which only admins see, and the
// book it was added as if that's hidden from the viewer.
func (s *Server) forViewer(r *http.Request, req *store.BookRequest) *store.BookRequest {
	user := currentUser(r)
	if req == nil || user.Role == store.RoleAdmin {
		return req
	}
	shown := *req
	shown.Requesters = nil
	if shown.BookID != nil {
		if _, err := s.store.GetBook(user.ID, *shown.BookID); err != nil {
			shown.BookID = nil
		}
	}
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

// Lookups are remembered: found books for a month, "not found" for a day,
// in case the databases add the book.
const (
	lookupKeep  = 30 * 24 * time.Hour
	missingKeep = 24 * time.Hour
)

// lookup finds the book of an ISBN-13, from what an earlier lookup found
// if it's recent enough.
func (s *Server) lookup(ctx context.Context, code string) (isbn.Info, error) {
	now := time.Now()
	saved, found, fresh, err := s.store.CachedLookup(code, now.Add(-lookupKeep), now.Add(-missingKeep))
	if err != nil {
		return isbn.Info{}, err
	}
	if fresh {
		if !found {
			return isbn.Info{}, isbn.ErrNotFound
		}
		var info isbn.Info
		if err := json.Unmarshal([]byte(saved), &info); err == nil {
			return info, nil
		}
	}
	info, err := s.opts.LookupISBN(ctx, code)
	switch {
	case errors.Is(err, isbn.ErrNotFound):
		if err := s.store.SaveLookup(code, false, "{}"); err != nil {
			slog.Error("can't save an ISBN lookup", "err", err)
		}
	case err == nil:
		data, _ := json.Marshal(info)
		if err := s.store.SaveLookup(code, true, string(data)); err != nil {
			slog.Error("can't save an ISBN lookup", "err", err)
		}
	}
	return info, err
}

// requestCover downloads a request's cover and saves it, made into a JPEG
// like uploaded covers.
func (s *Server) requestCover(ctx context.Context, id int64, url string) ([]byte, error) {
	data, err := s.opts.FetchCover(ctx, url)
	if err != nil {
		return nil, err
	}
	jpeg, err := cover.FromUpload(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return jpeg, s.store.SetRequestCover(id, jpeg)
}

// saveRequestCover gets a new request's cover in the background, so asking
// for a book doesn't wait for it.
func (s *Server) saveRequestCover(id int64, url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := s.requestCover(ctx, id, url); err != nil {
		slog.Warn("can't save a request's cover", "request", id, "url", url, "err", err)
	}
}

// getRequestCover sends a request's saved cover. Requests saved before
// covers were get theirs on first view.
func (s *Server) getRequestCover(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// What others asked for is theirs to know.
	if user := currentUser(r); user.Role != store.RoleAdmin {
		if req, err := s.store.GetRequest(user.ID, id); err != nil || !req.RequestedByMe {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
	}
	data, url, err := s.store.RequestCover(id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if data == nil && url != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if data, err = s.requestCover(ctx, id, url); err != nil {
			slog.Warn("can't save a request's cover", "request", id, "url", url, "err", err)
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusBadGateway, "the cover couldn't be downloaded, try again later")
			return
		}
	}
	if data == nil {
		writeError(w, http.StatusNotFound, "this request has no cover")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	// Saved once, it doesn't change.
	w.Header().Set("Cache-Control", "private, max-age=604800")
	w.Write(data)
}

// requestCooldown says how long the user must wait before requesting
// another book, and why: RequestsPerDay a day, RequestGap apart.
func (s *Server) requestCooldown(userID int64) (time.Duration, string, error) {
	now := time.Now()
	times, err := s.store.RecentRequests(userID, now.Add(-24*time.Hour))
	if err != nil {
		return 0, "", err
	}
	if len(times) >= s.opts.RequestsPerDay {
		// Free again when the oldest of today's falls out of the day.
		oldest := times[len(times)-s.opts.RequestsPerDay]
		return oldest.Add(24 * time.Hour).Sub(now), fmt.Sprintf("you can request %d books a day; try again later", s.opts.RequestsPerDay), nil
	}
	if s.opts.RequestGap > 0 && len(times) > 0 {
		if wait := times[len(times)-1].Add(s.opts.RequestGap).Sub(now); wait > 0 {
			return wait, "wait a moment before requesting another book", nil
		}
	}
	return 0, "", nil
}
