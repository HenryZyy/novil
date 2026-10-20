package api

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"novil/backend/internal/store"
)

// logEvents are the events the log knows, for filtering.
var logEvents = []string{
	"login", "login_failed", "logout", "password_changed",
	"user_created", "user_updated", "user_deleted", "password_reset",
	"book_uploaded", "book_deleted", "series_deleted", "library_scanned", "request_updated",
}

// audit adds an entry to the activity log. user did it (nil for nobody
// known), detail says to what. A failed write only shows in the server log.
func (s *Server) audit(r *http.Request, event string, user *store.User, username, detail string) {
	e := store.LogEntry{Event: event, Username: username, Detail: detail, IP: clientIP(r, s.trustedProxies).String()}
	if user != nil {
		e.UserID, e.Username = &user.ID, user.Username
	}
	e.DeviceKind, e.Device = describeDevice(r.UserAgent())
	if err := s.store.AddLog(e); err != nil {
		slog.Error("can't write the activity log", "event", event, "err", err)
	}
}

// actor is the logged-in user, for audit.
func actor(r *http.Request) *store.User {
	u := currentUser(r)
	return &u
}

// listLog returns log entries, newest first, 100 at a time:
// ?event=login,login_failed&search=…&before=<id of the last one shown>.
func (s *Server) listLog(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := store.LogQuery{Search: strings.TrimSpace(query.Get("search")), Limit: 100}
	if v := query.Get("event"); v != "" {
		q.Events = strings.Split(v, ",")
		for _, e := range q.Events {
			if !slices.Contains(logEvents, e) {
				writeError(w, http.StatusBadRequest, "unknown event "+e+"; known are "+strings.Join(logEvents, ", "))
				return
			}
		}
	}
	if v := query.Get("before"); v != "" {
		before, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid before")
			return
		}
		q.Before = before
	}
	entries, err := s.store.ListLog(q)
	respond(w, entries, err)
}

// activeReaders are reports newer than this.
const activeReaders = 3 * time.Minute

// reading is sent every minute by an open reader: {"bookId": 12}.
func (s *Server) reading(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BookID int64 `json:"bookId"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user := currentUser(r)
	if _, err := s.store.GetBook(user.ID, req.BookID); err != nil {
		respond(w, nil, err)
		return
	}
	kind, device := describeDevice(r.UserAgent())
	ip := clientIP(r, s.trustedProxies).String()
	if err := s.store.TouchReading(user.ID, clientName(r), req.BookID, ip, kind, device); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) stopReading(w http.ResponseWriter, r *http.Request) {
	if err := s.store.EndReading(currentUser(r).ID, clientName(r)); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clientName tells a user's browsers and apps apart.
func clientName(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > 300 {
		ua = ua[:300]
	}
	return ua
}

func (s *Server) listReading(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.Reading(time.Now().Add(-activeReaders))
	respond(w, sessions, err)
}
