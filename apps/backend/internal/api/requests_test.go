package api

import (
	"context"
	"errors"
	"image/color"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"novil/backend/internal/auth"
	"novil/backend/internal/isbn"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

func TestBookRequests(t *testing.T) {
	var lookups atomic.Int32
	env := newTestServerWith(t, Options{
		LoginBurst:  1000,
		LookupBurst: 1000,
		RequestGap:  -1,
		LookupISBN: func(ctx context.Context, code string) (isbn.Info, error) {
			lookups.Add(1)
			switch code {
			case "9780062941503":
				return isbn.Info{ISBN: code, Title: "My Dark Vanessa", Authors: "Kate Elizabeth Russell", Published: "2020"}, nil
			case "9780441013593":
				return isbn.Info{ISBN: code, Title: "Dune", Authors: "Frank Herbert"}, nil
			case "9780000000002":
				return isbn.Info{}, errors.New("timeout")
			}
			return isbn.Info{}, isbn.ErrNotFound
		},
	})
	anna := addReader(t, env)
	hash, _ := auth.HashPassword("bob-password")
	if _, err := env.store.CreateUser("bob", hash, store.RoleUser, false); err != nil {
		t.Fatal(err)
	}
	bob := env.login(t, "bob", "bob-password")

	var found struct {
		Book    isbn.Info          `json:"book"`
		Request *store.BookRequest `json:"request"`
	}
	anna.do("GET", "/api/isbn/978-0-06-294150-3", "", http.StatusOK, &found)
	if found.Book.Title != "My Dark Vanessa" || found.Request != nil {
		t.Errorf("lookup %+v", found)
	}
	anna.do("GET", "/api/isbn/9780062941504", "", http.StatusBadRequest, nil)
	anna.do("GET", "/api/isbn/9790000000001", "", http.StatusNotFound, nil)
	anna.do("GET", "/api/isbn/9780000000002", "", http.StatusBadGateway, nil)

	var req store.BookRequest
	anna.do("POST", "/api/requests", `{"isbn":"9780062941503"}`, http.StatusCreated, &req)
	if req.Title != "My Dark Vanessa" || req.RequestCount != 1 || !req.RequestedByMe || req.Status != "open" || req.Requesters != nil {
		t.Errorf("anna's request %+v", req)
	}
	// Once per user, also written as an ISBN-10 of the same book.
	anna.do("POST", "/api/requests", `{"isbn":"006294150X"}`, http.StatusConflict, nil)
	anna.do("POST", "/api/requests", `{"isbn":"bad"}`, http.StatusBadRequest, nil)
	anna.do("POST", "/api/requests", `{"isbn":"9790000000001"}`, http.StatusNotFound, nil)

	// Bob joins Anna's request instead of making a second one, without a
	// second lookup.
	before := lookups.Load()
	bob.do("POST", "/api/requests", `{"isbn":"9780062941503"}`, http.StatusOK, &req)
	if req.RequestCount != 2 || lookups.Load() != before {
		t.Errorf("bob's request %+v, lookups %d", req, lookups.Load()-before)
	}
	bob.do("POST", "/api/requests", `{"isbn":"9780441013593"}`, http.StatusCreated, nil)

	var mine []store.BookRequest
	anna.do("GET", "/api/me/requests", "", http.StatusOK, &mine)
	if len(mine) != 1 || mine[0].RequestCount != 2 || mine[0].Requesters != nil {
		t.Errorf("anna's requests %+v", mine)
	}

	// Admins see each book once, the most wanted first, with who asked.
	var all []store.BookRequest
	env.admin.do("GET", "/api/requests?status=open", "", http.StatusOK, &all)
	if len(all) != 2 || all[0].Title != "My Dark Vanessa" || len(all[0].Requesters) != 2 || all[1].RequestCount != 1 {
		t.Errorf("admin's requests %+v", all)
	}
	var stats store.Stats
	env.admin.do("GET", "/api/stats", "", http.StatusOK, &stats)
	if stats.Requests != 2 {
		t.Errorf("stats %+v", stats)
	}
	anna.do("GET", "/api/requests", "", http.StatusForbidden, nil)
	anna.do("PATCH", "/api/requests/"+itoa(all[0].ID), `{"status":"declined"}`, http.StatusForbidden, nil)

	// The admin adds the book; the request shows it to the users.
	root := t.TempDir()
	testutil.WriteEPUB(t, filepath.Join(root, "My Dark Vanessa.epub"), `<dc:title>My Dark Vanessa</dc:title>`)
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Books", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	var books []store.Book
	env.admin.do("GET", "/api/books", "", http.StatusOK, &books)
	env.admin.do("PATCH", "/api/requests/"+itoa(all[0].ID), `{"status":"added","bookId":999}`, http.StatusBadRequest, nil)
	env.admin.do("PATCH", "/api/requests/"+itoa(all[0].ID), `{"status":"added","bookId":`+itoa(books[0].ID)+`}`, http.StatusOK, &req)
	if req.Status != "added" || req.BookID == nil || *req.BookID != books[0].ID {
		t.Errorf("added %+v", req)
	}
	anna.do("GET", "/api/me/requests", "", http.StatusOK, &mine)
	if mine[0].Status != "added" || mine[0].BookID == nil {
		t.Errorf("anna sees %+v", mine)
	}
	// Hidden from her, the book doesn't show, not even its ID.
	env.admin.do("PATCH", "/api/books/"+itoa(books[0].ID), `{"hidden":true}`, http.StatusOK, nil)
	anna.do("GET", "/api/me/requests", "", http.StatusOK, &mine)
	if mine[0].BookID != nil {
		t.Errorf("anna sees the hidden book %d", *mine[0].BookID)
	}
	env.admin.do("PATCH", "/api/books/"+itoa(books[0].ID), `{"hidden":false}`, http.StatusOK, nil)
	var carl *client
	{
		hash, _ := auth.HashPassword("carl-password")
		env.store.CreateUser("carl", hash, store.RoleUser, false)
		carl = env.login(t, "carl", "carl-password")
	}
	carl.do("POST", "/api/requests", `{"isbn":"9780062941503"}`, http.StatusConflict, nil)

	// Declined, then wanted by someone new: open again.
	env.admin.do("PATCH", "/api/requests/"+itoa(all[1].ID), `{"status":"declined"}`, http.StatusOK, nil)
	bob.do("POST", "/api/requests", `{"isbn":"9780441013593"}`, http.StatusConflict, nil)
	carl.do("POST", "/api/requests", `{"isbn":"9780441013593"}`, http.StatusOK, &req)
	if req.Status != "open" || req.RequestCount != 2 {
		t.Errorf("reopened %+v", req)
	}

	// Withdrawing the last vote of an open request deletes it.
	bob.do("DELETE", "/api/me/requests/"+itoa(req.ID), "", http.StatusNoContent, nil)
	carl.do("DELETE", "/api/me/requests/"+itoa(req.ID), "", http.StatusNoContent, nil)
	carl.do("DELETE", "/api/me/requests/"+itoa(req.ID), "", http.StatusNotFound, nil)
	env.admin.do("GET", "/api/requests", "", http.StatusOK, &all)
	if len(all) != 1 {
		t.Errorf("after withdrawing %+v", all)
	}
	env.admin.do("DELETE", "/api/requests/"+itoa(all[0].ID), "", http.StatusNoContent, nil)
	anna.do("GET", "/api/me/requests", "", http.StatusOK, &mine)
	if len(mine) != 0 {
		t.Errorf("after delete %+v", mine)
	}
}

func TestSavedLookups(t *testing.T) {
	png := testutil.PNG(t, 40, 60, color.RGBA{0, 128, 0, 255})
	var lookups, fetches atomic.Int32
	env := newTestServerWith(t, Options{
		LoginBurst:  1000,
		LookupBurst: 1000,
		LookupISBN: func(ctx context.Context, code string) (isbn.Info, error) {
			lookups.Add(1)
			if code == "9780441013593" {
				return isbn.Info{ISBN: code, Title: "Dune", CoverURL: "https://covers.example/dune.jpg"}, nil
			}
			return isbn.Info{}, isbn.ErrNotFound
		},
		FetchCover: func(ctx context.Context, url string) ([]byte, error) {
			fetches.Add(1)
			return png, nil
		},
	})
	// The same ISBN, also when not found, is only looked up once.
	for range 3 {
		env.admin.do("GET", "/api/isbn/9780441013593", "", http.StatusOK, nil)
		env.admin.do("GET", "/api/isbn/9790000000001", "", http.StatusNotFound, nil)
	}
	if n := lookups.Load(); n != 2 {
		t.Errorf("%d lookups, want 2", n)
	}

	// A request saves its cover once; views don't download it again.
	var req store.BookRequest
	env.admin.do("POST", "/api/requests", `{"isbn":"9780441013593"}`, http.StatusCreated, &req)
	for i := 0; i < 50; i++ {
		if data, _, _ := env.store.RequestCover(req.ID); data != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for range 3 {
		resp := env.admin.request("GET", "/api/requests/"+itoa(req.ID)+"/cover", "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("cover: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	if n := fetches.Load(); n != 1 {
		t.Errorf("cover downloaded %d times, want 1", n)
	}
	if n := lookups.Load(); n != 2 {
		t.Errorf("requesting looked it up again: %d lookups", n)
	}
	env.admin.do("GET", "/api/requests/999/cover", "", http.StatusNotFound, nil)
	// Only for the people who asked, and admins.
	other := addReader(t, env)
	other.do("GET", "/api/requests/"+itoa(req.ID)+"/cover", "", http.StatusNotFound, nil)
}

func TestRequestCooldown(t *testing.T) {
	found := func(ctx context.Context, code string) (isbn.Info, error) {
		return isbn.Info{ISBN: code, Title: "Book " + code}, nil
	}
	books := []string{"9780441013593", "9780062941503", "9780804429573", "9780140449136"}

	// Requests a day.
	env := newTestServerWith(t, Options{LoginBurst: 1000, LookupBurst: 1000, LookupISBN: found, RequestsPerDay: 2, RequestGap: -1})
	reader := addReader(t, env)
	reader.do("POST", "/api/requests", `{"isbn":"`+books[0]+`"}`, http.StatusCreated, nil)
	reader.do("POST", "/api/requests", `{"isbn":"`+books[1]+`"}`, http.StatusCreated, nil)
	resp := reader.request("POST", "/api/requests", `{"isbn":"`+books[2]+`"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("third request: %d, want 429", resp.StatusCode)
	}
	// Free again a day after the first.
	if after, _ := strconv.Atoi(resp.Header.Get("Retry-After")); after < 23*3600 || after > 24*3600 {
		t.Errorf("Retry-After %q", resp.Header.Get("Retry-After"))
	}
	// Admins aren't limited.
	for _, b := range books {
		resp := env.admin.request("POST", "/api/requests", `{"isbn":"`+b+`"}`)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("admin limited on %s", b)
		}
	}

	// The gap between two.
	env = newTestServerWith(t, Options{LoginBurst: 1000, LookupBurst: 1000, LookupISBN: found, RequestGap: time.Hour})
	reader = addReader(t, env)
	reader.do("POST", "/api/requests", `{"isbn":"`+books[0]+`"}`, http.StatusCreated, nil)
	reader.do("POST", "/api/requests", `{"isbn":"`+books[1]+`"}`, http.StatusTooManyRequests, nil)
}

func TestWriteLimit(t *testing.T) {
	env := newTestServerWith(t, Options{LoginBurst: 1000, WriteBurst: 5})
	reader := addReader(t, env)
	for range 5 {
		reader.do("DELETE", "/api/me/reading", "", http.StatusNoContent, nil)
	}
	reader.do("DELETE", "/api/me/reading", "", http.StatusTooManyRequests, nil)
	// Reading isn't limited, and each user has their own allowance.
	reader.do("GET", "/api/me", "", http.StatusOK, nil)
	env.admin.do("DELETE", "/api/me/reading", "", http.StatusNoContent, nil)
}
