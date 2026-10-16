package api

import (
	"net/http"
	"testing"

	"novil/backend/internal/store"
)

func TestAdminEditsUsers(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, nil)
	admin.do("POST", "/api/users", `{"username":"bob","password":"start-password"}`, http.StatusCreated, nil)
	anna := env.login(t, "anna", "start-password")
	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	anna.token = s.Token

	// Renaming keeps the user logged in; they log in with the new name.
	var u store.User
	admin.do("PATCH", "/api/users/2", `{"username":"anna.m"}`, http.StatusOK, &u)
	if u.Username != "anna.m" || u.Role != store.RoleUser {
		t.Errorf("after rename %+v", u)
	}
	anna.do("GET", "/api/me", "", http.StatusOK, &u)
	if u.Username != "anna.m" {
		t.Errorf("me after rename %+v", u)
	}
	env.login(t, "anna.m", "anna-password")

	// Promoting takes effect on the next request, without logging in again.
	anna.do("GET", "/api/users", "", http.StatusForbidden, nil)
	admin.do("PATCH", "/api/users/2", `{"role":"admin"}`, http.StatusOK, &u)
	if u.Role != store.RoleAdmin {
		t.Errorf("after promote %+v", u)
	}
	anna.do("GET", "/api/users", "", http.StatusOK, nil)

	// Admins can demote other admins, but never change their own role, so
	// there is always an admin left.
	anna.do("PATCH", "/api/users/1", `{"role":"user"}`, http.StatusOK, nil)
	admin.do("GET", "/api/users", "", http.StatusForbidden, nil)
	anna.do("PATCH", "/api/users/2", `{"role":"user"}`, http.StatusBadRequest, nil)
	// Renaming yourself is fine.
	anna.do("PATCH", "/api/users/2", `{"username":"anna"}`, http.StatusOK, nil)

	for _, body := range []string{
		`{}`,
		`{"username":"bob"}`,
		`{"username":"BOB"}`,
		`{"username":"x"}`,
		`{"role":"king"}`,
	} {
		want := http.StatusBadRequest
		if body == `{"username":"bob"}` || body == `{"username":"BOB"}` {
			want = http.StatusConflict
		}
		anna.do("PATCH", "/api/users/1", body, want, nil)
	}
	anna.do("PATCH", "/api/users/99", `{"role":"admin"}`, http.StatusNotFound, nil)
}
