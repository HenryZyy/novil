package api

import (
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"novil/backend/internal/auth"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func addReader(t *testing.T, env *testEnv) *client {
	t.Helper()
	hash, err := auth.HashPassword("reader-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.CreateUser("reader", hash, store.RoleUser, false); err != nil {
		t.Fatal(err)
	}
	return env.login(t, "reader", "reader-password")
}

func TestBooksArePerUser(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg", "02.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)

	env.admin.do("PUT", "/api/books/1/progress", `{"page":2,"completed":true}`, http.StatusOK, nil)
	reader.do("PUT", "/api/books/1/progress", `{"page":1}`, http.StatusOK, nil)

	var p store.Progress
	reader.do("GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 1 || p.Completed {
		t.Errorf("reader's progress %+v", p)
	}
	// With progress from two users, the book must still be listed once.
	var books []store.Book
	reader.do("GET", "/api/series/1/books", "", http.StatusOK, &books)
	if len(books) != 1 || books[0].ReadPage != 1 {
		t.Errorf("reader's books %+v", books)
	}
	var series store.Series
	env.admin.do("GET", "/api/series/1", "", http.StatusOK, &series)
	if series.BookCount != 1 || series.ReadCount != 1 {
		t.Errorf("admin's series %+v", series)
	}
}

func TestSeriesQueries(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg", "02.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 2.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Claymore", "Vol 1.cbz"), "01.jpg")
	c.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	check := func(query string, want ...string) {
		t.Helper()
		var series []store.Series
		c.do("GET", "/api/series"+query, "", http.StatusOK, &series)
		got := []string{}
		for _, s := range series {
			got = append(got, s.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", query, got, want)
		}
	}

	// Books are 1 Akira, 2 and 3 Berserk, 4 Claymore.
	c.do("PUT", "/api/books/2/progress", `{"page":1}`, http.StatusOK, nil)
	time.Sleep(5 * time.Millisecond)
	c.do("PUT", "/api/series/3/read", `{"read":true}`, http.StatusOK, nil)

	check("", "Akira", "Berserk", "Claymore")
	check("?search=ERS", "Berserk")
	check("?status=unread", "Akira")
	check("?status=reading", "Berserk")
	check("?status=read", "Claymore")
	check("?sort=read", "Claymore", "Berserk", "Akira")
	check("?sort=name-desc", "Claymore", "Berserk", "Akira")

	// Release dates, one from an admin edit; Akira has none and goes last.
	c.do("PATCH", "/api/books/2", `{"released":"1989-08"}`, http.StatusOK, nil)
	c.do("PATCH", "/api/books/4", `{"released":"2001"}`, http.StatusOK, nil)
	check("?sort=released", "Claymore", "Berserk", "Akira")
	check("?sort=released-asc", "Berserk", "Claymore", "Akira")
	c.do("GET", "/api/series?sort=bogus", "", http.StatusBadRequest, nil)
	c.do("GET", "/api/series?status=bogus", "", http.StatusBadRequest, nil)

	var continueReading []store.Book
	c.do("GET", "/api/continue", "", http.StatusOK, &continueReading)
	if len(continueReading) != 1 || continueReading[0].ID != 2 || continueReading[0].SeriesName != "Berserk" {
		t.Errorf("continue reading %+v", continueReading)
	}

	// Berserk: book 2 (2 pages) at page 1, book 3 unread: a quarter read.
	var berserk store.Series
	c.do("GET", "/api/series/2", "", http.StatusOK, &berserk)
	if berserk.Progress != 0.25 {
		t.Errorf("berserk progress %v, want 0.25", berserk.Progress)
	}
	c.do("PUT", "/api/series/2/read", `{"read":true}`, http.StatusOK, &berserk)
	if berserk.ReadCount != 2 {
		t.Errorf("berserk after marking read %+v", berserk)
	}
	c.do("PUT", "/api/series/2/read", `{"read":false}`, http.StatusOK, &berserk)
	if berserk.ReadCount != 0 || berserk.ReadingCount != 0 {
		t.Errorf("berserk after marking unread %+v", berserk)
	}

	var stats store.Stats
	c.do("GET", "/api/stats", "", http.StatusOK, &stats)
	if stats.Libraries != 1 || stats.Series != 3 || stats.Books != 4 || stats.Pages != 5 || stats.Users != 1 {
		t.Errorf("stats %+v", stats)
	}
}

func TestLibraryPathsAreAdminOnly(t *testing.T) {
	env := newTestServer(t)
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", t.TempDir()), http.StatusCreated, nil)
	reader := addReader(t, env)

	var libs []store.Library
	reader.do("GET", "/api/libraries", "", http.StatusOK, &libs)
	if len(libs) != 1 || libs[0].Path != "" {
		t.Errorf("reader sees %+v", libs)
	}
	reader.do("GET", "/api/stats", "", http.StatusForbidden, nil)
}

func TestEPUBProgress(t *testing.T) {
	env := newTestServer(t)
	c := env.admin
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "Novels", "Dune.epub"), "PK")
	c.do("POST", "/api/libraries", libraryJSON(t, "Books", root), http.StatusCreated, nil)
	c.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	c.do("PUT", "/api/books/1/progress", `{"page":101}`, http.StatusBadRequest, nil)
	c.do("PUT", "/api/books/1/progress", `{"page":42,"position":"epubcfi(/6/14!/4/2/1:0)"}`, http.StatusOK, nil)

	var series store.Series
	c.do("GET", "/api/series/1", "", http.StatusOK, &series)
	if series.Progress < 0.419 || series.Progress > 0.421 {
		t.Errorf("series progress %v, want 0.42", series.Progress)
	}

	var book store.Book
	c.do("GET", "/api/books/1", "", http.StatusOK, &book)
	if book.ReadPage != 42 || book.ReadPosition != "epubcfi(/6/14!/4/2/1:0)" {
		t.Errorf("book %+v", book)
	}
}
