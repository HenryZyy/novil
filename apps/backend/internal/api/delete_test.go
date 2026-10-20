package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestDeleteBook(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	vol1 := filepath.Join(root, "Akira", "Vol 1.cbz")
	vol2 := filepath.Join(root, "Akira", "Vol 2.cbz")
	single := filepath.Join(root, "Berserk", "Vol 1.cbz")
	testutil.WriteCBZ(t, vol1, "01.jpg")
	testutil.WriteCBZ(t, vol2, "01.jpg")
	testutil.WriteCBZ(t, single, "01.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)

	var books []store.Book
	env.admin.do("GET", "/api/books", "", http.StatusOK, &books)
	ids := map[string]string{}
	for _, b := range books {
		ids[b.SeriesName+"/"+b.Title] = itoa(b.ID)
	}
	reader.do("PUT", "/api/books/"+ids["Akira/Vol 1"]+"/progress", `{"page":1}`, http.StatusOK, nil)
	reader.do("POST", "/api/books/"+ids["Akira/Vol 1"]+"/annotations", `{"kind":"bookmark","page":1}`, http.StatusCreated, nil)
	reader.do("DELETE", "/api/books/"+ids["Akira/Vol 1"], "", http.StatusForbidden, nil)

	env.admin.do("DELETE", "/api/books/"+ids["Akira/Vol 1"], "", http.StatusNoContent, nil)
	if _, err := os.Stat(vol1); !os.IsNotExist(err) {
		t.Errorf("the file is still there: %v", err)
	}
	if _, err := os.Stat(vol2); err != nil {
		t.Errorf("the other volume went too: %v", err)
	}
	reader.do("GET", "/api/books/"+ids["Akira/Vol 1"], "", http.StatusNotFound, nil)
	var cont []store.Book
	reader.do("GET", "/api/continue", "", http.StatusOK, &cont)
	if len(cont) != 0 {
		t.Errorf("still continuing %+v", cont)
	}

	// The last book of a series takes the series and its folder along.
	env.admin.do("DELETE", "/api/books/"+ids["Berserk/Vol 1"], "", http.StatusNoContent, nil)
	if _, err := os.Stat(filepath.Dir(single)); !os.IsNotExist(err) {
		t.Errorf("the empty folder is still there: %v", err)
	}
	var series []store.Series
	env.admin.do("GET", "/api/series", "", http.StatusOK, &series)
	if len(series) != 1 || series[0].Name != "Akira" || series[0].BookCount != 1 {
		t.Errorf("series %+v", series)
	}
	// A rescan doesn't bring them back.
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	env.admin.do("GET", "/api/books", "", http.StatusOK, &books)
	if len(books) != 1 {
		t.Errorf("after rescan %+v", books)
	}
	env.admin.do("DELETE", "/api/books/"+ids["Berserk/Vol 1"], "", http.StatusNotFound, nil)
	var log []store.LogEntry
	env.admin.do("GET", "/api/logs?event=book_deleted", "", http.StatusOK, &log)
	if len(log) != 2 || log[0].Detail != "Berserk / Vol 1.cbz from Manga" {
		t.Errorf("log %+v", log)
	}
}

func TestDeleteSeries(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 2.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	var series []store.Series
	env.admin.do("GET", "/api/series", "", http.StatusOK, &series)
	akira := itoa(series[0].ID)
	// Hidden books go too.
	env.admin.do("PATCH", "/api/books/2", `{"hidden":true}`, http.StatusOK, nil)
	addReader(t, env).do("DELETE", "/api/series/"+akira, "", http.StatusForbidden, nil)

	env.admin.do("DELETE", "/api/series/"+akira, "", http.StatusNoContent, nil)
	if _, err := os.Stat(filepath.Join(root, "Akira")); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Berserk", "Vol 1.cbz")); err != nil {
		t.Errorf("the other series went too: %v", err)
	}
	env.admin.do("GET", "/api/series", "", http.StatusOK, &series)
	if len(series) != 1 || series[0].Name != "Berserk" {
		t.Errorf("series left %+v", series)
	}
	env.admin.do("DELETE", "/api/series/"+akira, "", http.StatusNotFound, nil)
	var log []store.LogEntry
	env.admin.do("GET", "/api/logs?event=series_deleted", "", http.StatusOK, &log)
	if len(log) != 1 || log[0].Detail != "Akira (2 books) from Manga" {
		t.Errorf("log %+v", log)
	}
}
