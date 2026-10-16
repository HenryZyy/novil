// Package api is the HTTP API.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"novil/backend/internal/archive"
	"novil/backend/internal/auth"
	"novil/backend/internal/cover"
	"novil/backend/internal/isbn"
	"novil/backend/internal/scanner"
	"novil/backend/internal/store"
)

type Server struct {
	store          *store.Store
	tokens         *auth.Tokens
	loginLimiter   *limiter
	lookupLimiter  *limiter
	trustedProxies []netip.Prefix
	opts           Options
}

// Options are the optional API settings.
type Options struct {
	// WebDir is the built frontend, served for every path outside /api.
	// Nothing is served when it's empty.
	WebDir         string
	MaxUploadBytes int64
	// CORSOrigins are the browser origins allowed to call the API; see cors.
	CORSOrigins []string
	// TrustedProxies are reverse proxies whose X-Forwarded-For header tells
	// the client's real IP; see clientIP.
	TrustedProxies []netip.Prefix
	// Each IP may try LoginBurst passwords at once, then one more every
	// LoginInterval. Zero means 5 and 12s, so 5 per minute in the long run.
	LoginBurst    int
	LoginInterval time.Duration
	// ISBN lookups call other services, so each IP may make LookupBurst at
	// once, then one every 3 seconds. Zero means 10.
	LookupBurst int
	// Covers makes covers from book files. Nil means a new one.
	Covers *cover.Generator
	// LookupISBN finds the book of an ISBN for requests. Nil means none
	// is ever found, so books can't be requested.
	LookupISBN isbn.Lookup
}

// New returns the API handler.
func New(st *store.Store, tokens *auth.Tokens, opts Options) http.Handler {
	if opts.LoginBurst == 0 {
		opts.LoginBurst = 5
	}
	if opts.LoginInterval == 0 {
		opts.LoginInterval = 12 * time.Second
	}
	if opts.LookupBurst == 0 {
		opts.LookupBurst = 10
	}
	if opts.Covers == nil {
		opts.Covers = cover.NewGenerator()
	}
	s := &Server{
		store:          st,
		tokens:         tokens,
		loginLimiter:   newLimiter(opts.LoginInterval, opts.LoginBurst),
		lookupLimiter:  newLimiter(3*time.Second, opts.LookupBurst),
		trustedProxies: opts.TrustedProxies,
		opts:           opts,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("POST /api/auth/login", s.limitAttempts(s.login))
	mux.HandleFunc("POST /api/auth/logout", s.guard(pendingPassword, s.logout))
	mux.HandleFunc("GET /api/me", s.guard(pendingPassword, s.me))
	mux.HandleFunc("PUT /api/me/password", s.guard(pendingPassword, s.limitAttempts(s.changePassword)))

	mux.HandleFunc("GET /api/users", s.guard(adminOnly, s.listUsers))
	mux.HandleFunc("POST /api/users", s.guard(adminOnly, s.createUser))
	mux.HandleFunc("PUT /api/me/avatar", s.guard(member, s.setMyAvatar))
	mux.HandleFunc("DELETE /api/me/avatar", s.guard(member, s.deleteMyAvatar))
	mux.HandleFunc("GET /api/users/{id}/avatar", s.guard(member, s.getAvatar))
	mux.HandleFunc("DELETE /api/users/{id}/avatar", s.guard(adminOnly, s.deleteUserAvatar))
	mux.HandleFunc("PATCH /api/users/{id}", s.guard(adminOnly, s.updateUser))
	mux.HandleFunc("PUT /api/users/{id}/password", s.guard(adminOnly, s.resetPassword))
	mux.HandleFunc("DELETE /api/users/{id}", s.guard(adminOnly, s.deleteUser))

	mux.HandleFunc("GET /api/libraries", s.guard(member, s.listLibraries))
	mux.HandleFunc("POST /api/libraries", s.guard(adminOnly, s.createLibrary))
	mux.HandleFunc("PATCH /api/libraries/{id}", s.guard(adminOnly, s.updateLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", s.guard(adminOnly, s.scanLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/books", s.guard(adminOnly, s.uploadBook))

	mux.HandleFunc("GET /api/series", s.guard(member, s.listSeries))
	mux.HandleFunc("GET /api/series/{id}", s.guard(member, s.getSeries))
	mux.HandleFunc("GET /api/series/{id}/books", s.guard(member, s.listBooks))
	mux.HandleFunc("PUT /api/series/{id}/read", s.guard(member, s.setSeriesRead))
	mux.HandleFunc("PATCH /api/series/{id}", s.guard(adminOnly, s.updateSeries))
	mux.HandleFunc("PUT /api/series/{id}/lists/{list}", s.guard(member, s.setListed))
	mux.HandleFunc("DELETE /api/series/{id}/lists/{list}", s.guard(member, s.setListed))
	mux.HandleFunc("GET /api/continue", s.guard(member, s.continueReading))
	mux.HandleFunc("GET /api/stats", s.guard(adminOnly, s.stats))

	mux.HandleFunc("GET /api/books", s.guard(adminOnly, s.searchBooks))
	mux.HandleFunc("GET /api/books/{id}", s.guard(member, s.getBook))
	mux.HandleFunc("PATCH /api/books/{id}", s.guard(adminOnly, s.updateBook))
	mux.HandleFunc("GET /api/books/{id}/cover", s.guard(member, s.getCover))
	mux.HandleFunc("PUT /api/books/{id}/cover", s.guard(adminOnly, s.setCover))
	mux.HandleFunc("DELETE /api/books/{id}/cover", s.guard(adminOnly, s.deleteCover))
	mux.HandleFunc("GET /api/books/{id}/file", s.guard(member, s.getBookFile))
	mux.HandleFunc("GET /api/books/{id}/pages/{page}", s.guard(member, s.getPage))
	mux.HandleFunc("GET /api/books/{id}/progress", s.guard(member, s.getProgress))
	mux.HandleFunc("PUT /api/books/{id}/progress", s.guard(member, s.setProgress))
	mux.HandleFunc("GET /api/books/{id}/annotations", s.guard(member, s.listAnnotations))
	mux.HandleFunc("POST /api/books/{id}/annotations", s.guard(member, s.createAnnotation))
	mux.HandleFunc("PATCH /api/annotations/{id}", s.guard(member, s.updateAnnotation))
	mux.HandleFunc("DELETE /api/annotations/{id}", s.guard(member, s.deleteAnnotation))

	mux.HandleFunc("GET /api/isbn/{isbn}", s.guard(member, s.limitLookups(s.lookupISBN)))
	mux.HandleFunc("POST /api/requests", s.guard(member, s.limitLookups(s.requestBook)))
	mux.HandleFunc("GET /api/me/requests", s.guard(member, s.myRequests))
	mux.HandleFunc("DELETE /api/me/requests/{id}", s.guard(member, s.withdrawRequest))
	mux.HandleFunc("GET /api/requests", s.guard(adminOnly, s.listRequests))
	mux.HandleFunc("PATCH /api/requests/{id}", s.guard(adminOnly, s.updateRequest))
	mux.HandleFunc("DELETE /api/requests/{id}", s.guard(adminOnly, s.deleteRequest))

	mux.HandleFunc("GET /api/logs", s.guard(adminOnly, s.listLog))
	mux.HandleFunc("PUT /api/me/reading", s.guard(member, s.reading))
	mux.HandleFunc("DELETE /api/me/reading", s.guard(member, s.stopReading))
	mux.HandleFunc("GET /api/reading", s.guard(adminOnly, s.listReading))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	if opts.WebDir != "" {
		mux.Handle("/", spa(opts.WebDir))
	}
	return cors(opts.CORSOrigins, mux)
}

func spa(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path.Clean("/"+r.URL.Path))))
		if err != nil || info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries()
	if currentUser(r).Role != store.RoleAdmin {
		for i := range libs {
			libs[i].Path = ""
		}
	}
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

func (s *Server) updateLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		AllowUploads *bool `json:"allowUploads"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.AllowUploads == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	lib, err := s.store.SetAllowUploads(id, *req.AllowUploads)
	respond(w, lib, err)
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
	if err == nil {
		s.audit(r, "library_scanned", actor(r), "", fmt.Sprintf("%s: %d series, %d books", lib.Name, result.Series, result.Books))
	}
	respond(w, result, err)
}

func (s *Server) listSeries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := store.SeriesQuery{
		Search: strings.TrimSpace(query.Get("search")),
		Status: query.Get("status"),
		Sort:   query.Get("sort"),
		List:   query.Get("list"),
	}
	if v := query.Get("library"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid library id")
			return
		}
		q.LibraryID = id
	}
	switch q.Status {
	case "", "unread", "reading", "read":
	default:
		writeError(w, http.StatusBadRequest, "status must be unread, reading or read")
		return
	}
	if q.List != "" && !slices.Contains(store.Lists, q.List) {
		writeError(w, http.StatusBadRequest, "list must be favorites or want")
		return
	}
	if !store.ValidSort(q.Sort) {
		writeError(w, http.StatusBadRequest, "sort must be name-desc, added, added-asc, released, released-asc, read or listed")
		return
	}
	series, err := s.store.ListSeries(currentUser(r).ID, q)
	respond(w, series, err)
}

func (s *Server) getSeries(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r, "id"); ok {
		sr, err := s.store.GetSeries(currentUser(r).ID, id)
		respond(w, sr, err)
	}
}

func (s *Server) listBooks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID := currentUser(r).ID
	if _, err := s.store.GetSeries(userID, id); err != nil {
		respond(w, nil, err)
		return
	}
	books, err := s.store.ListBooks(userID, id)
	respond(w, books, err)
}

func (s *Server) setSeriesRead(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Read bool `json:"read"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	userID := currentUser(r).ID
	if _, err := s.store.GetSeries(userID, id); err != nil {
		respond(w, nil, err)
		return
	}
	if err := s.store.SetSeriesRead(userID, id, req.Read); err != nil {
		internalError(w, err)
		return
	}
	sr, err := s.store.GetSeries(userID, id)
	respond(w, sr, err)
}

// updateSeries hides or shows all books of a series: {"hidden": true}.
func (s *Server) updateSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Hidden *bool `json:"hidden"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Hidden == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	userID := currentUser(r).ID
	if _, err := s.store.GetSeries(userID, id); err != nil {
		respond(w, nil, err)
		return
	}
	if err := s.store.SetSeriesHidden(id, *req.Hidden); err != nil {
		internalError(w, err)
		return
	}
	sr, err := s.store.GetSeries(userID, id)
	respond(w, sr, err)
}

func (s *Server) searchBooks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := store.BookQuery{Search: strings.TrimSpace(query.Get("search")), Hidden: query.Get("hidden")}
	if v := query.Get("library"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid library id")
			return
		}
		q.LibraryID = id
	}
	if q.Hidden != "" && q.Hidden != "hidden" && q.Hidden != "visible" {
		writeError(w, http.StatusBadRequest, "hidden must be hidden or visible")
		return
	}
	books, err := s.store.SearchBooks(currentUser(r).ID, q)
	respond(w, books, err)
}

func (s *Server) continueReading(w http.ResponseWriter, r *http.Request) {
	books, err := s.store.ContinueReading(currentUser(r).ID, 20)
	respond(w, books, err)
}

func (s *Server) getBook(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r, "id"); ok {
		b, err := s.store.GetBook(currentUser(r).ID, id)
		respond(w, b, err)
	}
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Stats()
	respond(w, st, err)
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
	writeCBZPage(w, b, n)
}

func writeCBZPage(w http.ResponseWriter, b store.Book, n int) {
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
		Page      int    `json:"page"`
		Position  string `json:"position"`
		Completed bool   `json:"completed"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Books without pages (EPUBs) send a percentage instead.
	maxPage := b.PageCount
	if maxPage == 0 {
		maxPage = 100
	}
	if req.Page < 0 || req.Page > maxPage {
		writeError(w, http.StatusBadRequest, "page out of range")
		return
	}
	if len(req.Position) > 2000 {
		writeError(w, http.StatusBadRequest, "position is too long")
		return
	}
	p, err := s.store.SetProgress(currentUser(r).ID, store.Progress{
		BookID: b.ID, Page: req.Page, Position: req.Position, Completed: req.Completed,
	})
	respond(w, p, err)
}

func (s *Server) book(w http.ResponseWriter, r *http.Request) (store.Book, bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return store.Book{}, false
	}
	b, err := s.store.GetBook(currentUser(r).ID, id)
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
