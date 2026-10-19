package api

import (
	"context"
	"errors"
	"image/color"
	"net/http"
	"path/filepath"
	"testing"

	"novil/backend/internal/isbn"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestBookISBN(t *testing.T) {
	png := testutil.PNG(t, 40, 60, color.RGBA{0, 0, 255, 255})
	var fetched []string
	env := newTestServerWith(t, Options{
		LoginBurst:  1000,
		LookupBurst: 1000,
		LookupISBN: func(ctx context.Context, code string) (isbn.Info, error) {
			switch code {
			case "9780441013593":
				return isbn.Info{ISBN: code, Title: "Dune", Authors: "Frank Herbert", CoverURL: "https://covers.example/dune.jpg"}, nil
			case "9780062941503":
				return isbn.Info{ISBN: code, Title: "My Dark Vanessa"}, nil
			}
			return isbn.Info{}, isbn.ErrNotFound
		},
		FetchCover: func(ctx context.Context, url string) ([]byte, error) {
			fetched = append(fetched, url)
			if url == "https://covers.example/dune.jpg" {
				return png, nil
			}
			return nil, errors.New("not found")
		},
	})
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "Docs", "dune.pdf"), testutil.PDF())
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Docs", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	// The ISBN is metadata like the rest, stored as ISBN-13.
	var b store.Book
	env.admin.do("PATCH", "/api/books/1", `{"isbn":"0-441-01359-7","author":"Frank Herbert"}`, http.StatusOK, &b)
	if b.ISBN != "9780441013593" || b.Author != "Frank Herbert" || len(b.Edited) != 2 {
		t.Errorf("after edit %+v", b)
	}
	env.admin.do("PATCH", "/api/books/1", `{"isbn":"0-441-01359-8"}`, http.StatusBadRequest, nil)

	// The cover from the book's ISBN.
	env.admin.do("PUT", "/api/books/1/cover/isbn", "", http.StatusOK, &b)
	if !b.CustomCover || len(fetched) != 1 {
		t.Errorf("cover %+v, fetched %v", b, fetched)
	}
	resp := env.admin.request("GET", "/api/books/1/cover", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cover: %d", resp.StatusCode)
	}
	// Or from another ISBN; one without a cover says so.
	env.admin.do("PUT", "/api/books/1/cover/isbn", `{"isbn":"9780062941503"}`, http.StatusNotFound, nil)
	env.admin.do("PUT", "/api/books/1/cover/isbn", `{"isbn":"9790000000001"}`, http.StatusNotFound, nil)
	env.admin.do("PUT", "/api/books/1/cover/isbn", `{"isbn":"123"}`, http.StatusBadRequest, nil)

	env.admin.do("PATCH", "/api/books/1", `{"isbn":null}`, http.StatusOK, &b)
	if b.ISBN != "" {
		t.Errorf("reset %+v", b)
	}
	reader := addReader(t, env)
	reader.do("PUT", "/api/books/1/cover/isbn", `{"isbn":"9780441013593"}`, http.StatusForbidden, nil)
}
