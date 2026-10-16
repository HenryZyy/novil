package api

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

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
