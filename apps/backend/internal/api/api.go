// Package api exposes the novil REST API over HTTP.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"novil/backend/internal/archive"
	"novil/backend/internal/auth"
	"novil/backend/internal/scanner"
	"novil/backend/internal/store"
)

type Server struct {
	store          *store.Store
	tokens         *auth.Tokens
	loginLimiter   *limiter
	trustedProxies []netip.Prefix
}

// Options are the optional API settings.
type Options struct {
	// CORSOrigins are the browser origins allowed to call the API; see cors.
	CORSOrigins []string
	// TrustedProxies are reverse proxies whose X-Forwarded-For header tells
	// the client's real IP; see clientIP.
	TrustedProxies []netip.Prefix
	// Each IP may try LoginBurst passwords at once, then one more every
	// LoginInterval. Zero means 5 and 12s, so 5 per minute in the long run.
	LoginBurst    int
	LoginInterval time.Duration
}

// New returns the API handler.
func New(st *store.Store, tokens *auth.Tokens, opts Options) http.Handler {
	if opts.LoginBurst == 0 {
		opts.LoginBurst = 5
	}
	if opts.LoginInterval == 0 {
		opts.LoginInterval = 12 * time.Second
	}
	s := &Server{
		store:          st,
		tokens:         tokens,
		loginLimiter:   newLimiter(opts.LoginInterval, opts.LoginBurst),
		trustedProxies: opts.TrustedProxies,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("POST /api/auth/login", s.limitAttempts(s.login))
	mux.HandleFunc("POST /api/auth/logout", s.guard(pendingPassword, s.logout))
	mux.HandleFunc("GET /api/me", s.guard(pendingPassword, s.me))
	mux.HandleFunc("PUT /api/me/password", s.guard(pendingPassword, s.limitAttempts(s.changePassword)))

	mux.HandleFunc("GET /api/users", s.guard(adminOnly, s.listUsers))
	mux.HandleFunc("POST /api/users", s.guard(adminOnly, s.createUser))
	mux.HandleFunc("PUT /api/users/{id}/password", s.guard(adminOnly, s.resetPassword))
	mux.HandleFunc("DELETE /api/users/{id}", s.guard(adminOnly, s.deleteUser))

	mux.HandleFunc("GET /api/libraries", s.guard(member, s.listLibraries))
	mux.HandleFunc("POST /api/libraries", s.guard(adminOnly, s.createLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", s.guard(adminOnly, s.scanLibrary))

	mux.HandleFunc("GET /api/series", s.guard(member, s.listSeries))
	mux.HandleFunc("GET /api/series/{id}", s.guard(member, s.getSeries))
	mux.HandleFunc("GET /api/series/{id}/books", s.guard(member, s.listBooks))

	mux.HandleFunc("GET /api/books/{id}", s.guard(member, s.getBook))
	mux.HandleFunc("GET /api/books/{id}/file", s.guard(member, s.getBookFile))
	mux.HandleFunc("GET /api/books/{id}/pages/{page}", s.guard(member, s.getPage))
	mux.HandleFunc("GET /api/books/{id}/progress", s.guard(member, s.getProgress))
	mux.HandleFunc("PUT /api/books/{id}/progress", s.guard(member, s.setProgress))
	return cors(opts.CORSOrigins, mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries()
	respond(w, libs, err)
}

func (s *Server) createLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Path == "" {
		writeError(w, http.StatusBadRequest, "name and path are required")
		return
	}
	path, err := filepath.Abs(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is not a readable directory")
		return
	}
	lib, err := s.store.CreateLibrary(req.Name, path)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		writeError(w, http.StatusConflict, "a library with this path already exists")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, lib)
}

func (s *Server) scanLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	lib, err := s.store.GetLibrary(id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	found, err := scanner.Scan(lib.Path)
	if err != nil {
		internalError(w, err)
		return
	}
	result, err := s.store.SyncLibrary(lib.ID, found)
	respond(w, result, err)
}

func (s *Server) listSeries(w http.ResponseWriter, r *http.Request) {
	var libraryID int64
	if v := r.URL.Query().Get("library"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid library id")
			return
		}
		libraryID = id
	}
	series, err := s.store.ListSeries(libraryID)
	respond(w, series, err)
}

func (s *Server) getSeries(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r, "id"); ok {
		sr, err := s.store.GetSeries(id)
		respond(w, sr, err)
	}
}

func (s *Server) listBooks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.store.GetSeries(id); err != nil {
		respond(w, nil, err)
		return
	}
	books, err := s.store.ListBooks(id)
	respond(w, books, err)
}

func (s *Server) getBook(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r, "id"); ok {
		b, err := s.store.GetBook(id)
		respond(w, b, err)
	}
}

func (s *Server) getBookFile(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(b.Path)}))
	http.ServeFile(w, r, b.Path)
}

func (s *Server) getPage(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	if b.Format != "cbz" {
		writeError(w, http.StatusBadRequest, "pages are only available for cbz books")
		return
	}
	n, err := strconv.Atoi(r.PathValue("page"))
	if err != nil || n < 1 || n > b.PageCount {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}
	c, err := archive.OpenCBZ(b.Path)
	if err != nil {
		internalError(w, err)
		return
	}
	defer c.Close()
	page, contentType, err := c.Page(n)
	if err != nil {
		internalError(w, err)
		return
	}
	defer page.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	io.Copy(w, page)
}

func (s *Server) getProgress(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	p, err := s.store.GetProgress(currentUser(r).ID, b.ID)
	respond(w, p, err)
}

func (s *Server) setProgress(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	var req struct {
		Page      int  `json:"page"`
		Completed bool `json:"completed"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Page < 0 || (b.PageCount > 0 && req.Page > b.PageCount) {
		writeError(w, http.StatusBadRequest, "page out of range")
		return
	}
	p, err := s.store.SetProgress(currentUser(r).ID, store.Progress{BookID: b.ID, Page: req.Page, Completed: req.Completed})
	respond(w, p, err)
}

// book loads the book named by the {id} path value, writing an error
// response and returning false if that fails
func (s *Server) book(w http.ResponseWriter, r *http.Request) (store.Book, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return store.Book{}, false
	}
	b, err := s.store.GetBook(id)
	if err != nil {
		respond(w, nil, err)
		return store.Book{}, false
	}
	return b, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v)
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

// respond writes v as JSON, or the matching error response if err is set.
func respond(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case err != nil:
		internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

func internalError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeCodedError adds a machine-readable code that clients can check
// instead of matching the message, e.g. "password_change_required".
func writeCodedError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
