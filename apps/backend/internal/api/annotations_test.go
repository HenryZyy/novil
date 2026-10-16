package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestLists(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Claymore", "Vol 1.cbz"), "01.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)

	var sr store.Series
	reader.do("PUT", "/api/series/3/lists/want", "", http.StatusOK, &sr)
	if !sr.WantToRead || sr.Favorite {
		t.Errorf("after want %+v", sr)
	}
	reader.do("PUT", "/api/series/1/lists/want", "", http.StatusOK, nil)
	reader.do("PUT", "/api/series/1/lists/want", "", http.StatusOK, nil)
	reader.do("PUT", "/api/series/2/lists/favorites", "", http.StatusOK, nil)

	names := func(c *client, query string) []string {
		var list []store.Series
		c.do("GET", "/api/series"+query, "", http.StatusOK, &list)
		out := []string{}
		for _, s := range list {
			out = append(out, s.Name)
		}
		return out
	}
	if got := names(reader, "?list=want&sort=listed"); len(got) != 2 || got[0] != "Akira" || got[1] != "Claymore" {
		t.Errorf("want to read, recent first: %v", got)
	}
	if got := names(reader, "?list=favorites"); len(got) != 1 || got[0] != "Berserk" {
		t.Errorf("favorites: %v", got)
	}
	// Lists are per user.
	if got := names(env.admin, "?list=want"); len(got) != 0 {
		t.Errorf("admin's want to read: %v", got)
	}

	reader.do("DELETE", "/api/series/1/lists/want", "", http.StatusOK, &sr)
	if sr.WantToRead {
		t.Errorf("after removing %+v", sr)
	}
	if got := names(reader, "?list=want"); len(got) != 1 {
		t.Errorf("want to read after removing: %v", got)
	}
	reader.do("PUT", "/api/series/1/lists/later", "", http.StatusNotFound, nil)
	reader.do("PUT", "/api/series/99/lists/want", "", http.StatusNotFound, nil)
	reader.do("GET", "/api/series?list=later", "", http.StatusBadRequest, nil)

	// Hidden series can't be listed, and drop out of lists.
	env.admin.do("PATCH", "/api/series/2", `{"hidden":true}`, http.StatusOK, nil)
	reader.do("PUT", "/api/series/2/lists/want", "", http.StatusNotFound, nil)
	if got := names(reader, "?list=favorites"); len(got) != 0 {
		t.Errorf("favorites with a hidden series: %v", got)
	}
}

func TestAnnotations(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg", "02.jpg", "03.jpg")
	testutil.WriteEPUB(t, filepath.Join(root, "Dune", "Dune.epub"), `<dc:title>Dune</dc:title>`)
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Books", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)
	var books []store.Book
	env.admin.do("GET", "/api/books", "", http.StatusOK, &books)
	var comic, book string
	for _, b := range books {
		if b.Format == "cbz" {
			comic = itoa(b.ID)
		} else {
			book = itoa(b.ID)
		}
	}

	var a store.Annotation
	reader.do("POST", "/api/books/"+comic+"/annotations", `{"kind":"bookmark","page":2,"note":"Great panel"}`, http.StatusCreated, &a)
	if a.ID == 0 || a.Page != 2 || a.Note != "Great panel" {
		t.Errorf("bookmark %+v", a)
	}
	reader.do("POST", "/api/books/"+comic+"/annotations", `{"kind":"bookmark","page":1}`, http.StatusCreated, nil)
	for _, body := range []string{
		`{"kind":"highlight","page":1,"color":"yellow"}`,
		`{"kind":"bookmark","page":4}`,
		`{"kind":"bookmark","page":0}`,
		`{"kind":"note","page":1}`,
		`{"kind":"bookmark","page":1,"color":"red"}`,
	} {
		reader.do("POST", "/api/books/"+comic+"/annotations", body, http.StatusBadRequest, nil)
	}

	var h store.Annotation
	reader.do("POST", "/api/books/"+book+"/annotations",
		`{"kind":"highlight","page":40,"position":"epubcfi(/6/4!/4/2,/1:0,/1:5)","text":"  Hello ","color":"yellow"}`, http.StatusCreated, &h)
	if h.Text != "Hello" || h.Color != "yellow" {
		t.Errorf("highlight %+v", h)
	}
	reader.do("POST", "/api/books/"+book+"/annotations", `{"kind":"highlight","page":40,"color":"yellow"}`, http.StatusBadRequest, nil)

	var list []store.Annotation
	reader.do("GET", "/api/books/"+comic+"/annotations", "", http.StatusOK, &list)
	if len(list) != 2 || list[0].Page != 1 || list[1].Page != 2 {
		t.Errorf("in page order %+v", list)
	}
	// Annotations are private.
	env.admin.do("GET", "/api/books/"+comic+"/annotations", "", http.StatusOK, &list)
	if len(list) != 0 {
		t.Errorf("admin sees %+v", list)
	}
	env.admin.do("PATCH", "/api/annotations/"+itoa(h.ID), `{"note":"mine"}`, http.StatusNotFound, nil)
	env.admin.do("DELETE", "/api/annotations/"+itoa(h.ID), "", http.StatusNotFound, nil)

	reader.do("PATCH", "/api/annotations/"+itoa(h.ID), `{"note":"Remember this","color":"blue"}`, http.StatusOK, &h)
	if h.Note != "Remember this" || h.Color != "blue" || h.Text != "Hello" {
		t.Errorf("updated %+v", h)
	}
	reader.do("PATCH", "/api/annotations/"+itoa(h.ID), `{"color":"red"}`, http.StatusBadRequest, nil)
	reader.do("PATCH", "/api/annotations/"+itoa(h.ID), `{}`, http.StatusBadRequest, nil)

	// Hiding the book hides its annotations too.
	env.admin.do("PATCH", "/api/books/"+book, `{"hidden":true}`, http.StatusOK, nil)
	reader.do("GET", "/api/books/"+book+"/annotations", "", http.StatusNotFound, nil)
	reader.do("PATCH", "/api/annotations/"+itoa(h.ID), `{"note":"x"}`, http.StatusNotFound, nil)
	env.admin.do("PATCH", "/api/books/"+book, `{"hidden":false}`, http.StatusOK, nil)

	reader.do("DELETE", "/api/annotations/"+itoa(h.ID), "", http.StatusNoContent, nil)
	reader.do("DELETE", "/api/annotations/"+itoa(h.ID), "", http.StatusNotFound, nil)
}
