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

	"novil/backend/internal/db"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func newTestServer(t *testing.T) *httptest.Server {
	return newTestServerWithWeb(t, "")
}

func newTestServerWithWeb(t *testing.T, webDir string) *httptest.Server {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	srv := httptest.NewServer(New(store.New(database), Options{WebDir: webDir, MaxUploadBytes: 1 << 20}))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path, body string, wantStatus int, out any) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, wantStatus, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
}

// libraryJSON builds a create-library request body. The path must be
// JSON-encoded because Windows paths contain backslashes.
func libraryJSON(t *testing.T, name, path string) string {
	t.Helper()
	b, err := json.Marshal(map[string]string{"name": name, "path": path})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLibraryFlow(t *testing.T) {
	srv := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Solo Leveling", "Chapter 1.cbz"), "002.jpg", "001.jpg")

	var lib store.Library
	do(t, srv, "POST", "/api/libraries", libraryJSON(t, "Manhwa", root), http.StatusCreated, &lib)
	do(t, srv, "POST", "/api/libraries", libraryJSON(t, "Dup", root), http.StatusConflict, nil)

	var result store.ScanResult
	do(t, srv, "POST", "/api/libraries/1/scan", "", http.StatusOK, &result)
	if result.Series != 1 || result.Books != 1 {
		t.Fatalf("scan result %+v", result)
	}

	var series []store.Series
	do(t, srv, "GET", "/api/series?library=1", "", http.StatusOK, &series)
	if len(series) != 1 || series[0].Name != "Solo Leveling" || series[0].BookCount != 1 ||
		series[0].CoverBookID == nil || *series[0].CoverBookID != 1 {
		t.Fatalf("series %+v", series)
	}

	var books []store.Book
	do(t, srv, "GET", "/api/series/1/books", "", http.StatusOK, &books)
	if len(books) != 1 || books[0].PageCount != 2 {
		t.Fatalf("books %+v", books)
	}

	resp, err := http.Get(srv.URL + "/api/books/1/pages/1")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(page) != "001.jpg" || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("page 1 = %q (%s), want 001.jpg (image/jpeg)", page, resp.Header.Get("Content-Type"))
	}
	do(t, srv, "GET", "/api/books/1/pages/3", "", http.StatusNotFound, nil)

	var p store.Progress
	do(t, srv, "GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 0 {
		t.Errorf("initial progress %+v", p)
	}
	do(t, srv, "PUT", "/api/books/1/progress", `{"page":2,"completed":true}`, http.StatusOK, nil)
	do(t, srv, "PUT", "/api/books/1/progress", `{"page":9}`, http.StatusBadRequest, nil)

	// A rescan keeps the book's ID, so progress survives.
	do(t, srv, "POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	do(t, srv, "GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 2 || !p.Completed {
		t.Errorf("progress after rescan %+v", p)
	}
	do(t, srv, "GET", "/api/series/1/books", "", http.StatusOK, &books)
	if books[0].ReadPage != 2 || !books[0].Completed {
		t.Errorf("book progress after rescan %+v", books[0])
	}

	// Deleting the file and rescanning removes the book and series.
	if err := os.RemoveAll(filepath.Join(root, "Solo Leveling")); err != nil {
		t.Fatal(err)
	}
	do(t, srv, "POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	do(t, srv, "GET", "/api/books/1", "", http.StatusNotFound, nil)
	do(t, srv, "GET", "/api/series", "", http.StatusOK, &series)
	if len(series) != 0 {
		t.Errorf("series after delete %+v", series)
	}
}

func TestDownloadNonASCIIFilename(t *testing.T) {
	srv := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "ワンピース", "第1巻.cbz"), "01.jpg")
	do(t, srv, "POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	do(t, srv, "POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	resp, err := http.Get(srv.URL + "/api/books/1/file")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "filename*=utf-8''") {
		t.Errorf("Content-Disposition %q is not percent-encoded", cd)
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil || params["filename"] != "第1巻.cbz" {
		t.Errorf("Content-Disposition %q: filename %q, err %v", cd, params["filename"], err)
	}
}

func TestServesFrontend(t *testing.T) {
	web := t.TempDir()
	testutil.WriteFile(t, filepath.Join(web, "index.html"), "index")
	testutil.WriteFile(t, filepath.Join(web, "assets", "app-1a2b.js"), "js")
	srv := newTestServerWithWeb(t, web)

	for path, want := range map[string]string{
		"/":                   "index",
		"/series/5":           "index",
		"/assets/app-1a2b.js": "js",
		"/../../etc/passwd":   "index",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != want {
			t.Errorf("GET %s = %q, want %q", path, body, want)
		}
	}
	do(t, srv, "GET", "/api/unknown", "", http.StatusNotFound, nil)
}
