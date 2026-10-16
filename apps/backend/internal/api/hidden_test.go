package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestHiddenBooks(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg", "02.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 2.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)

	var all []store.Book
	env.admin.do("GET", "/api/books?search=berserk", "", http.StatusOK, &all)
	if len(all) != 2 || all[0].Title != "Vol 1" {
		t.Fatalf("search %+v", all)
	}
	vol1, berserk := all[0].ID, all[0].SeriesID
	reader.do("PUT", "/api/books/"+itoa(vol1)+"/progress", `{"page":1}`, http.StatusOK, nil)

	var b store.Book
	env.admin.do("PATCH", "/api/books/"+itoa(vol1), `{"hidden":true}`, http.StatusOK, &b)
	if !b.Hidden || b.Title != "Vol 1" {
		t.Errorf("hidden book %+v", b)
	}

	// The reader can't reach the hidden book in any way.
	for _, path := range []string{"", "/file", "/pages/1", "/cover", "/progress"} {
		reader.do("GET", "/api/books/"+itoa(vol1)+path, "", http.StatusNotFound, nil)
	}
	reader.do("PUT", "/api/books/"+itoa(vol1)+"/progress", `{"page":2}`, http.StatusNotFound, nil)
	var books []store.Book
	reader.do("GET", "/api/series/"+itoa(berserk)+"/books", "", http.StatusOK, &books)
	if len(books) != 1 || books[0].Title != "Vol 2" {
		t.Errorf("reader's books %+v", books)
	}
	reader.do("GET", "/api/continue", "", http.StatusOK, &books)
	if len(books) != 0 {
		t.Errorf("reader continues %+v", books)
	}
	var series store.Series
	reader.do("GET", "/api/series/"+itoa(berserk), "", http.StatusOK, &series)
	if series.BookCount != 1 || series.HiddenCount != 0 || series.ReadingCount != 0 || *series.CoverBookID == vol1 {
		t.Errorf("reader's series %+v", series)
	}
	env.admin.do("GET", "/api/series/"+itoa(berserk), "", http.StatusOK, &series)
	if series.BookCount != 2 || series.HiddenCount != 1 {
		t.Errorf("admin's series %+v", series)
	}

	// A series with only hidden books is gone for the reader.
	env.admin.do("PATCH", "/api/series/"+itoa(berserk), `{"hidden":true}`, http.StatusOK, &series)
	if series.HiddenCount != 2 {
		t.Errorf("hidden series %+v", series)
	}
	reader.do("GET", "/api/series/"+itoa(berserk), "", http.StatusNotFound, nil)
	reader.do("PUT", "/api/series/"+itoa(berserk)+"/read", `{"read":true}`, http.StatusNotFound, nil)
	var list []store.Series
	reader.do("GET", "/api/series", "", http.StatusOK, &list)
	if len(list) != 1 || list[0].Name != "Akira" {
		t.Errorf("reader's series list %+v", list)
	}
	env.admin.do("GET", "/api/series?status=unread", "", http.StatusOK, &list)
	if len(list) != 2 {
		t.Errorf("admin's series list %+v", list)
	}

	// Rescans keep it hidden.
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	env.admin.do("GET", "/api/books?hidden=hidden", "", http.StatusOK, &all)
	if len(all) != 2 {
		t.Errorf("hidden after rescan %+v", all)
	}
	env.admin.do("GET", "/api/books?hidden=visible", "", http.StatusOK, &all)
	if len(all) != 1 || all[0].SeriesName != "Akira" {
		t.Errorf("visible books %+v", all)
	}

	// Only admins manage visibility, and admins see everything right away.
	reader.do("GET", "/api/books", "", http.StatusForbidden, nil)
	reader.do("PATCH", "/api/series/"+itoa(berserk), `{"hidden":false}`, http.StatusForbidden, nil)
	env.admin.do("PATCH", "/api/books/"+itoa(vol1), `{"hidden":"yes"}`, http.StatusBadRequest, nil)
	env.admin.do("GET", "/api/books?hidden=maybe", "", http.StatusBadRequest, nil)
	env.admin.do("PATCH", "/api/series/"+itoa(berserk), `{"hidden":false}`, http.StatusOK, nil)
	reader.do("GET", "/api/books/"+itoa(vol1), "", http.StatusOK, &b)
	if b.Hidden || b.ReadPage != 1 {
		t.Errorf("shown again %+v", b)
	}
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
