package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

// errorBody is the JSON of an error response.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func TestRoutesNeedToken(t *testing.T) {
	env := newTestServer(t)
	anon := env.anon(t)
	anon.do("GET", "/health", "", http.StatusOK, nil)

	var e errorBody
	anon.do("GET", "/api/series", "", http.StatusUnauthorized, &e)
	if e.Code != "unauthorized" {
		t.Errorf("code %q, want unauthorized", e.Code)
	}
	anon.token = "not-a-token"
	anon.do("GET", "/api/series", "", http.StatusUnauthorized, nil)
	anon.do("GET", "/api/me", "", http.StatusUnauthorized, nil)
}

func TestLogin(t *testing.T) {
	env := newTestServer(t)
	anon := env.anon(t)

	// Wrong password and unknown user get the same answer.
	var wrongPassword, unknownUser errorBody
	anon.do("POST", "/api/auth/login", `{"username":"admin","password":"wrong-password"}`, http.StatusUnauthorized, &wrongPassword)
	anon.do("POST", "/api/auth/login", `{"username":"nobody","password":"wrong-password"}`, http.StatusUnauthorized, &unknownUser)
	if wrongPassword != unknownUser || wrongPassword.Code != "invalid_credentials" {
		t.Errorf("wrong password %+v, unknown user %+v: want the same invalid_credentials error", wrongPassword, unknownUser)
	}

	// Usernames ignore case.
	var s session
	anon.do("POST", "/api/auth/login", `{"username":"ADMIN","password":"admin-password"}`, http.StatusOK, &s)
	if s.Token == "" || s.User.Username != "admin" || s.User.Role != store.RoleAdmin {
		t.Errorf("session %+v", s)
	}
}

func TestNewUserMustChangePassword(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin

	var created store.User
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, &created)
	if created.Role != store.RoleUser || !created.MustChangePassword {
		t.Errorf("created %+v, want a user who must change the password", created)
	}
	admin.do("POST", "/api/users", `{"username":"Anna","password":"start-password"}`, http.StatusConflict, nil)
	admin.do("POST", "/api/users", `{"username":"bob","password":"short"}`, http.StatusBadRequest, nil)
	admin.do("POST", "/api/users", `{"username":"bob","password":"start-password","role":"king"}`, http.StatusBadRequest, nil)

	anna := env.login(t, "anna", "start-password")

	// Before changing the password, only /api/me and the change itself work.
	var e errorBody
	anna.do("GET", "/api/series", "", http.StatusForbidden, &e)
	if e.Code != "password_change_required" {
		t.Errorf("code %q, want password_change_required", e.Code)
	}
	var me store.User
	anna.do("GET", "/api/me", "", http.StatusOK, &me)
	if !me.MustChangePassword {
		t.Error("me: mustChangePassword is false")
	}

	anna.do("PUT", "/api/me/password", `{"currentPassword":"wrong","newPassword":"anna-password"}`, http.StatusForbidden, nil)
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"short"}`, http.StatusBadRequest, nil)
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"start-password"}`, http.StatusBadRequest, nil)

	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	if s.User.MustChangePassword {
		t.Error("mustChangePassword still set after the change")
	}

	// The old token stopped working, the new one works everywhere.
	anna.do("GET", "/api/me", "", http.StatusUnauthorized, nil)
	anna.token = s.Token
	anna.do("GET", "/api/series", "", http.StatusOK, nil)

	// Normal users can't do admin things.
	anna.do("GET", "/api/users", "", http.StatusForbidden, nil)
	anna.do("POST", "/api/users", `{"username":"eve","password":"eve-password"}`, http.StatusForbidden, nil)
	anna.do("POST", "/api/libraries", libraryJSON(t, "Mine", t.TempDir()), http.StatusForbidden, nil)
}

func TestAdminManagesUsers(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	var anna store.User
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, &anna)
	annaClient := env.login(t, "anna", "start-password")

	var users []store.User
	admin.do("GET", "/api/users", "", http.StatusOK, &users)
	if len(users) != 2 || users[0].Username != "admin" || users[1].Username != "anna" {
		t.Errorf("users %+v", users)
	}

	// A reset logs anna out and she has to change the new password again.
	admin.do("PUT", "/api/users/2/password", `{"password":"reset-password"}`, http.StatusOK, nil)
	annaClient.do("GET", "/api/me", "", http.StatusUnauthorized, nil)
	annaClient = env.login(t, "anna", "reset-password")
	var me store.User
	annaClient.do("GET", "/api/me", "", http.StatusOK, &me)
	if !me.MustChangePassword {
		t.Error("mustChangePassword not set after reset")
	}
	admin.do("PUT", "/api/users/99/password", `{"password":"reset-password"}`, http.StatusNotFound, nil)

	admin.do("DELETE", "/api/users/1", "", http.StatusBadRequest, nil)
	admin.do("DELETE", "/api/users/2", "", http.StatusNoContent, nil)
	admin.do("DELETE", "/api/users/2", "", http.StatusNotFound, nil)
	annaClient.do("GET", "/api/me", "", http.StatusUnauthorized, nil)
}

func TestProgressIsPerUser(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Series", "1.cbz"), "01.jpg", "02.jpg", "03.jpg")
	admin.do("POST", "/api/libraries", libraryJSON(t, "Lib", root), http.StatusCreated, nil)
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)

	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, nil)
	anna := env.login(t, "anna", "start-password")
	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	anna.token = s.Token

	admin.do("PUT", "/api/books/1/progress", `{"page":3,"completed":true}`, http.StatusOK, nil)
	anna.do("PUT", "/api/books/1/progress", `{"page":1}`, http.StatusOK, nil)

	var p store.Progress
	admin.do("GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 3 || !p.Completed {
		t.Errorf("admin progress %+v, want page 3 completed", p)
	}
	anna.do("GET", "/api/books/1/progress", "", http.StatusOK, &p)
	if p.Page != 1 || p.Completed {
		t.Errorf("anna progress %+v, want page 1", p)
	}
}

func TestCORS(t *testing.T) {
	env := newTestServer(t)

	preflight := func(origin string) *http.Response {
		req, _ := http.NewRequest("OPTIONS", env.srv.URL+"/api/series", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", "authorization")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}

	resp := preflight("http://localhost:5173")
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("allowed origin: status %d, allow-origin %q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	resp = preflight("https://evil.example")
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("other origin got allow-origin %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}
