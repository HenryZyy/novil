package api

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"novil/backend/internal/auth"
	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

// browser is a client that only has cookies, like an <img> tag or a link.
func browser(t *testing.T, env *testEnv) (*http.Client, func(method, path, body string) *http.Response) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	send := func(method, path, body string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, env.srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	return client, send
}

func status(resp *http.Response) int {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

func TestBrowserCookie(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 1.cbz"), "01.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	client, send := browser(t, env)
	if got := status(send("GET", "/api/books/1/pages/1", "")); got != http.StatusUnauthorized {
		t.Fatalf("page before login: %d, want 401", got)
	}

	resp := send("POST", "/api/auth/login", `{"username":"admin","password":"admin-password"}`)
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == tokenCookie {
			cookie = c
		}
	}
	if status(resp) != http.StatusOK || cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("login cookie %+v", cookie)
	}

	for _, path := range []string{"/api/books/1/pages/1", "/api/books/1/file", "/api/series"} {
		if got := status(send("GET", path, "")); got != http.StatusOK {
			t.Errorf("GET %s with cookie: %d, want 200", path, got)
		}
	}

	if got := status(send("POST", "/api/auth/logout", "")); got != http.StatusNoContent {
		t.Fatalf("logout: %d", got)
	}
	u, _ := url.Parse(env.srv.URL)
	if left := client.Jar.Cookies(u); len(left) != 0 {
		t.Errorf("cookies after logout: %v", left)
	}
	// The old cookie is revoked, not just deleted from this browser.
	req, _ := http.NewRequest("GET", env.srv.URL+"/api/series", nil)
	req.AddCookie(&http.Cookie{Name: tokenCookie, Value: cookie.Value})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := status(resp); got != http.StatusUnauthorized {
		t.Errorf("revoked cookie: %d, want 401", got)
	}

	req, _ = http.NewRequest("GET", env.srv.URL+"/api/series", nil)
	req.AddCookie(&http.Cookie{Name: tokenCookie, Value: "garbage"})
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := status(resp); got != http.StatusUnauthorized {
		t.Errorf("garbage cookie: %d, want 401", got)
	}
}

func TestLibraryAdminRoutes(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	hash, err := auth.HashPassword("reader-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.CreateUser("reader", hash, store.RoleUser, false); err != nil {
		t.Fatal(err)
	}
	reader := env.login(t, "reader", "reader-password")
	anon := env.anon(t)

	for _, r := range []struct{ method, path string }{
		{"PATCH", "/api/libraries/1"},
		{"POST", "/api/libraries/1/books?series=x&filename=a.cbz"},
	} {
		anon.do(r.method, r.path, `{"allowUploads":true}`, http.StatusUnauthorized, nil)
		reader.do(r.method, r.path, `{"allowUploads":true}`, http.StatusForbidden, nil)
	}
	env.admin.do("PATCH", "/api/libraries/1", `{"allowUploads":true}`, http.StatusOK, nil)
}

// A page on another site can't use the login cookie to change anything,
// nor log the browser in.
func TestCrossSiteRequests(t *testing.T) {
	env := newTestServer(t)
	var s session
	env.anon(t).do("POST", "/api/auth/login", `{"username":"admin","password":"admin-password"}`, http.StatusOK, &s)
	send := func(method, path, body string, headers map[string]string) int {
		req, _ := http.NewRequest(method, env.srv.URL+path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: tokenCookie, Value: s.Token})
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	newUser := `{"username":"evil","password":"evil-password-1","role":"admin"}`
	for _, h := range []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "https://evil.example"},
	} {
		if got := send("POST", "/api/users", newUser, h); got != http.StatusForbidden {
			t.Errorf("%v: %d, want 403", h, got)
		}
		if got := send("POST", "/api/auth/login", `{"username":"admin","password":"admin-password"}`, h); got != http.StatusForbidden {
			t.Errorf("login %v: %d, want 403", h, got)
		}
	}
	// Reading is fine from anywhere the cookie goes; so are novil's pages.
	if got := send("GET", "/api/me", "", map[string]string{"Sec-Fetch-Site": "cross-site"}); got != http.StatusOK {
		t.Errorf("GET: %d", got)
	}
	if got := send("POST", "/api/users", newUser, map[string]string{"Sec-Fetch-Site": "same-origin"}); got != http.StatusCreated {
		t.Errorf("same origin: %d", got)
	}
	// The token header isn't sent by browsers on their own, so it's fine.
	env.admin.do("DELETE", "/api/me/reading", "", http.StatusNoContent, nil)
}
