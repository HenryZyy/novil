package api

import (
	"bytes"
	"image/jpeg"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestReadPDF(t *testing.T) {
	if testing.Short() {
		t.Skip("starting PDFium takes a few seconds")
	}
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "Docs", "report.pdf"), testutil.PDFWithPages(3))
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Docs", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	var b store.Book
	env.admin.do("GET", "/api/books/1", "", http.StatusOK, &b)
	if b.PageCount != 3 {
		t.Fatalf("page count %d, want 3", b.PageCount)
	}
	// Pages come as JPEGs at reading size, keeping the page's shape.
	for n, wantRed := range map[string]bool{"1": true, "2": false, "3": true} {
		resp := env.admin.request("GET", "/api/books/1/pages/"+n, "")
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("page %s: %d %s", n, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 1600 || h != 2400 {
			t.Errorf("page %s is %dx%d, want 1600x2400", n, w, h)
		}
		r, _, bl, _ := img.At(800, 1200).RGBA()
		if red := r > bl; red != wantRed {
			t.Errorf("page %s: red %v, want %v", n, red, wantRed)
		}
	}
	env.admin.do("GET", "/api/books/1/pages/4", "", http.StatusNotFound, nil)

	// Read by page, like comics, with bookmarks.
	env.admin.do("PUT", "/api/books/1/progress", `{"page":3}`, http.StatusOK, nil)
	env.admin.do("PUT", "/api/books/1/progress", `{"page":4}`, http.StatusBadRequest, nil)
	env.admin.do("POST", "/api/books/1/annotations", `{"kind":"bookmark","page":2}`, http.StatusCreated, nil)
	env.admin.do("POST", "/api/books/1/annotations", `{"kind":"highlight","page":2,"position":"x","color":"yellow"}`, http.StatusBadRequest, nil)

	// A rescan keeps the count of the unchanged file.
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	var books []store.Book
	env.admin.do("GET", "/api/series/1/books", "", http.StatusOK, &books)
	if books[0].PageCount != 3 {
		t.Errorf("after rescan %d pages", books[0].PageCount)
	}
}
