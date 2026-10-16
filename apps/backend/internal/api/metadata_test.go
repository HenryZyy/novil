package api

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestBookMetadata(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	root := t.TempDir()
	testutil.WriteEPUB(t, filepath.Join(root, "Klassiker", "dune.epub"), `
		<dc:title>Dune</dc:title>
		<dc:creator>Frank Herbert</dc:creator>
		<dc:language>en</dc:language>
		<dc:date>1965-08-01</dc:date>
		<dc:description>Spice.</dc:description>`)
	admin.do("POST", "/api/libraries", libraryJSON(t, "Bücher", root), http.StatusCreated, nil)
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	var b store.Book
	admin.do("GET", "/api/books/1", "", http.StatusOK, &b)
	if b.Title != "Dune" || b.Author != "Frank Herbert" || b.Language != "en" ||
		b.Released != "1965-08-01" || b.Description != "Spice." || len(b.Edited) != 0 || b.AddedAt == "" {
		t.Fatalf("book from file %+v", b)
	}

	// An admin corrects some fields.
	admin.do("PATCH", "/api/books/1", `{"title":"Dune (Deutsch)","language":"de","released":"1967"}`, http.StatusOK, &b)
	if b.Title != "Dune (Deutsch)" || b.Language != "de" || b.Released != "1967" || b.Author != "Frank Herbert" {
		t.Errorf("after edit %+v", b)
	}
	if !slices.Equal(b.Edited, []string{"title", "language", "released"}) {
		t.Errorf("edited %v", b.Edited)
	}

	// A rescan updates the file's values but keeps the admin's.
	testutil.WriteEPUB(t, filepath.Join(root, "Klassiker", "dune.epub"), `
		<dc:title>Dune</dc:title><dc:creator>F. Herbert</dc:creator><dc:language>en</dc:language>`)
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	admin.do("GET", "/api/books/1", "", http.StatusOK, &b)
	if b.Title != "Dune (Deutsch)" || b.Author != "F. Herbert" || b.Released != "1967" {
		t.Errorf("after rescan %+v", b)
	}

	// null goes back to the file's value.
	admin.do("PATCH", "/api/books/1", `{"title":null,"released":null,"language":null}`, http.StatusOK, &b)
	if b.Title != "Dune" || b.Released != "" || len(b.Edited) != 0 {
		t.Errorf("after reset %+v", b)
	}

	// An empty author is a valid override, e.g. for anonymous works.
	admin.do("PATCH", "/api/books/1", `{"author":""}`, http.StatusOK, &b)
	if b.Author != "" || !slices.Equal(b.Edited, []string{"author"}) {
		t.Errorf("empty author %+v", b)
	}

	for _, body := range []string{
		`{}`,
		`{"title":""}`,
		`{"released":"August 1965"}`,
		`{"pageCount":5}`,
		`{"author":42}`,
	} {
		admin.do("PATCH", "/api/books/1", body, http.StatusBadRequest, nil)
	}
	admin.do("PATCH", "/api/books/99", `{"title":"x"}`, http.StatusNotFound, nil)

	// Everyone sees the books, only admins edit them.
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, nil)
	anna := env.login(t, "anna", "start-password")
	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	anna.token = s.Token
	anna.do("GET", "/api/books/1", "", http.StatusOK, &b)
	anna.do("PATCH", "/api/books/1", `{"title":"Mine"}`, http.StatusForbidden, nil)
}

func TestFileNameIsTitleFallback(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "Papers", "Annual Report 2025.pdf"), "%PDF-1.7\n")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Docs", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	var b store.Book
	env.admin.do("GET", "/api/books/1", "", http.StatusOK, &b)
	if b.Title != "Annual Report 2025" || b.Format != "pdf" {
		t.Errorf("book %+v", b)
	}
}

func TestUploadReadsMetadata(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	c.do("POST", "/api/libraries", libraryJSON(t, "Lib", root), http.StatusCreated, nil)
	c.do("PATCH", "/api/libraries/1", `{"allowUploads":true}`, http.StatusOK, nil)

	src := filepath.Join(t.TempDir(), "x.epub")
	testutil.WriteEPUB(t, src, `<dc:title>Neuromancer</dc:title><dc:creator>William Gibson</dc:creator>`)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := upload(c, "Sprawl", "neuromancer.epub", data).StatusCode; got != http.StatusCreated {
		t.Fatalf("upload status %d", got)
	}
	var books []store.Book
	c.do("GET", "/api/series/1/books", "", http.StatusOK, &books)
	if len(books) != 1 || books[0].Title != "Neuromancer" || books[0].Author != "William Gibson" {
		t.Errorf("books %+v", books)
	}
}
