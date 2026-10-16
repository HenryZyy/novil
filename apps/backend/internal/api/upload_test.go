package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func cbzBytes(t *testing.T, pages ...string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.cbz")
	testutil.WriteCBZ(t, path, pages...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func epubBytes(t *testing.T, mimetype string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("mimetype")
	w.Write([]byte(mimetype))
	zw.Close()
	return buf.Bytes()
}

func upload(c *client, series, filename string, body []byte) *http.Response {
	c.t.Helper()
	q := url.Values{"series": {series}, "filename": {filename}}
	resp := c.request("POST", "/api/libraries/1/books?"+q.Encode(), string(body))
	resp.Body.Close()
	return resp
}

func TestUpload(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 2.cbz"), "01.jpg")
	c.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	c.do("PUT", "/api/books/1/progress", `{"page":1}`, http.StatusOK, nil)

	if got := upload(c, "Berserk", "Vol 1.cbz", cbzBytes(t, "01.jpg")).StatusCode; got != http.StatusForbidden {
		t.Fatalf("upload while disabled: status %d, want 403", got)
	}

	var lib store.Library
	c.do("PATCH", "/api/libraries/1", `{"allowUploads":true}`, http.StatusOK, &lib)
	if !lib.AllowUploads {
		t.Fatalf("library %+v, want uploads allowed", lib)
	}

	if got := upload(c, "Berserk", "Vol 1.cbz", cbzBytes(t, "01.jpg", "02.jpg")).StatusCode; got != http.StatusCreated {
		t.Fatalf("upload: status %d, want 201", got)
	}
	var books []store.Book
	c.do("GET", "/api/series/1/books", "", http.StatusOK, &books)
	if len(books) != 2 || books[0].Title != "Vol 1" || books[0].PageCount != 2 || books[1].Title != "Vol 2" {
		t.Fatalf("books after upload %+v", books)
	}
	// The existing book kept its ID and progress.
	if books[1].ID != 1 || books[1].ReadPage != 1 {
		t.Errorf("existing book changed: %+v", books[1])
	}

	if got := upload(c, "New Series", "Dune.epub", epubBytes(t, "application/epub+zip")).StatusCode; got != http.StatusCreated {
		t.Errorf("epub upload: status %d, want 201", got)
	}
	if got := upload(c, "Papers", "a.pdf", []byte("%PDF-1.7 ...")).StatusCode; got != http.StatusCreated {
		t.Errorf("pdf upload: status %d, want 201", got)
	}
	var series []store.Series
	c.do("GET", "/api/series", "", http.StatusOK, &series)
	if len(series) != 3 {
		t.Errorf("series after uploads %+v", series)
	}

	for _, tc := range []struct {
		name, series, filename string
		body                   []byte
		want                   int
	}{
		{"duplicate", "Berserk", "Vol 1.cbz", cbzBytes(t, "01.jpg"), http.StatusConflict},
		{"not a zip", "Berserk", "Vol 3.cbz", []byte("hello"), http.StatusBadRequest},
		{"zip without images", "Berserk", "Vol 3.cbz", cbzBytes(t, "notes.txt"), http.StatusBadRequest},
		{"bad epub", "Berserk", "x.epub", epubBytes(t, "text/plain"), http.StatusBadRequest},
		{"bad pdf", "Berserk", "x.pdf", []byte("nope"), http.StatusBadRequest},
		{"wrong type", "Berserk", "virus.exe", []byte("MZ"), http.StatusBadRequest},
		{"too large", "Berserk", "big.pdf", append([]byte("%PDF-"), make([]byte, 2<<20)...), http.StatusRequestEntityTooLarge},
		{"parent folder", "..", "Vol 3.cbz", cbzBytes(t, "01.jpg"), http.StatusBadRequest},
		{"nested series", "Berserk/../../x", "Vol 3.cbz", cbzBytes(t, "01.jpg"), http.StatusBadRequest},
		{"path in filename", "Berserk", "../Vol 3.cbz", cbzBytes(t, "01.jpg"), http.StatusBadRequest},
		{"hidden file", "Berserk", ".Vol 3.cbz", cbzBytes(t, "01.jpg"), http.StatusBadRequest},
		{"empty series", " ", "Vol 3.cbz", cbzBytes(t, "01.jpg"), http.StatusBadRequest},
	} {
		if got := upload(c, tc.series, tc.filename, tc.body).StatusCode; got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
	}

	// Rejected uploads leave nothing behind.
	entries, err := os.ReadDir(filepath.Join(root, "Berserk"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, ", "); got != "Vol 1.cbz, Vol 2.cbz" {
		t.Errorf("files in series folder: %s", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "x")); err == nil {
		t.Error("upload escaped the library folder")
	}
}
