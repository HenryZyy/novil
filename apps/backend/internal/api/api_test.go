package api

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novil/backend/internal/auth"
	"novil/backend/internal/db"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

type testEnv struct {
	srv   *httptest.Server
	store *store.Store
	// admin is logged in as an admin who has set their password.
	admin *client
}

func newTestServer(t *testing.T) *testEnv {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	st := store.New(database)
	tokens := auth.NewTokens([]byte(strings.Repeat("s", 32)), time.Hour)
	srv := httptest.NewServer(New(st, tokens, []string{"http://localhost:5173"}))
	t.Cleanup(srv.Close)

	env := &testEnv{srv: srv, store: st}
	hash, err := auth.HashPassword("admin-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("admin", hash, store.RoleAdmin, false); err != nil {
		t.Fatal(err)
	}
	env.admin = env.login(t, "admin", "admin-password")
	return env
}

// anon returns a client without a token.
func (e *testEnv) anon(t *testing.T) *client {
	return &client{t: t, srv: e.srv}
}

func (e *testEnv) login(t *testing.T, username, password string) *client {
	t.Helper()
	c := e.anon(t)
	var s session
	c.do("POST", "/api/auth/login", toJSON(t, map[string]string{"username": username, "password": password}), http.StatusOK, &s)
	c.token = s.Token
	return c
}

// client sends requests with its token, failing the test on an unexpected
// status.
type client struct {
	t     *testing.T
	srv   *httptest.Server
	token string
}

func (c *client) request(method, path, body string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.srv.URL+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp
}

func (c *client) do(method, path, body string, wantStatus int, out any) {
	c.t.Helper()
	resp := c.request(method, path, body)
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, wantStatus, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
}

// toJSON encodes a request body. Building JSON by hand breaks on Windows
// paths, whose backslashes must be escaped.
func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func libraryJSON(t *testing.T, name, path string) string {
	return toJSON(t, map[string]string{"name": name, "path": path})
}

func TestLibraryFlow(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Solo Leveling", "Chapter 1.cbz"), "002.jpg", "001.jpg")

	var lib store.Library
	c.do("POST", "/api/libraries", libraryJSON(t, "Manhwa", root), http.StatusCreated, &lib)
	c.do("POST", "/api/libraries", libraryJSON(t, "Dup", root), http.StatusConflict, nil)

	var result store.ScanResult
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, &result)
	if result.Series != 1 || result.Books != 1 {
		t.Fatalf("scan result %+v", result)
	}

	var series []store.Series
	c.do("GET", "/api/series?library=1", "", http.StatusOK, &series)
	if len(series) != 1 || series[0].Name != "Solo Leveling" || series[0].BookCount != 1 {
		t.Fatalf("series %+v", series)
	}

	var books []store.Book
	c.do("GET", "/api/series/1/books", "", http.StatusOK, &books)
	if len(books) != 1 || books[0].PageCount != 2 {
		t.Fatalf("books %+v", books)
	}

	resp := c.request("GET", "/api/books/1/pages/1", "")
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(page) != "001.jpg" || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("page 1 = %q (%s), want 001.jpg (image/jpeg)", page, resp.Header.Get("Content-Type"))
	}
	c.do("GET", "/api/books/1/pages/3", "", http.StatusNotFound, nil)

	var p store.Progress
	c.do("GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 0 {
		t.Errorf("initial progress %+v", p)
	}
	c.do("PUT", "/api/books/1/progress", `{"page":2,"completed":true}`, http.StatusOK, nil)
	c.do("PUT", "/api/books/1/progress", `{"page":9}`, http.StatusBadRequest, nil)

	// A rescan keeps the book's ID, so progress survives.
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	c.do("GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 2 || !p.Completed {
		t.Errorf("progress after rescan %+v", p)
	}

	// Deleting the file and rescanning removes the book and series.
	if err := os.RemoveAll(filepath.Join(root, "Solo Leveling")); err != nil {
		t.Fatal(err)
	}
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	c.do("GET", "/api/books/1", "", http.StatusNotFound, nil)
	c.do("GET", "/api/series", "", http.StatusOK, &series)
	if len(series) != 0 {
		t.Errorf("series after delete %+v", series)
	}
}

func TestDownloadNonASCIIFilename(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "ワンピース", "第1巻.cbz"), "01.jpg")
	c.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	resp := c.request("GET", "/api/books/1/file", "")
	resp.Body.Close()
	// Raw UTF-8 in headers is garbled by browsers, so it must be percent-encoded.
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "filename*=utf-8''") {
		t.Errorf("Content-Disposition %q is not percent-encoded", cd)
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil || params["filename"] != "第1巻.cbz" {
		t.Errorf("Content-Disposition %q: filename %q, err %v", cd, params["filename"], err)
	}
}
