package api

import (
	"image/color"
	"io"
	"net/http"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

// getAvatar fetches a user's profile picture and returns the response and
// its body.
func getAvatar(c *client, userID string, header ...string) (*http.Response, []byte) {
	c.t.Helper()
	req, _ := http.NewRequest("GET", c.srv.URL+"/api/users/"+userID+"/avatar", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func TestAvatars(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, nil)
	anna := env.login(t, "anna", "start-password")
	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	anna.token = s.Token

	if resp, _ := getAvatar(admin, "2"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("no picture yet: status %d, want 404", resp.StatusCode)
	}

	// Anna uploads a picture; everyone logged in sees it.
	red := testutil.PNG(t, 400, 300, color.RGBA{255, 0, 0, 255})
	var u store.User
	anna.do("PUT", "/api/me/avatar", string(red), http.StatusOK, &u)
	if !u.HasAvatar || u.ID != 2 {
		t.Errorf("after upload %+v", u)
	}
	resp, data := getAvatar(admin, "2")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" || !isRed(centerColor(t, data)) {
		t.Fatalf("admin reading anna's picture: status %d", resp.StatusCode)
	}
	// /api/me and the user list say who has one.
	anna.do("GET", "/api/me", "", http.StatusOK, &u)
	var users []store.User
	admin.do("GET", "/api/users", "", http.StatusOK, &users)
	if !u.HasAvatar || users[0].HasAvatar || !users[1].HasAvatar {
		t.Errorf("me %+v, users %+v", u, users)
	}

	// A new picture gets a new ETag, so browsers don't keep the old one.
	etag := resp.Header.Get("ETag")
	if resp, _ := getAvatar(anna, "2", "If-None-Match", etag); resp.StatusCode != http.StatusNotModified {
		t.Errorf("same picture: status %d, want 304", resp.StatusCode)
	}
	anna.do("PUT", "/api/me/avatar", string(testutil.PNG(t, 50, 50, color.RGBA{0, 0, 255, 255})), http.StatusOK, nil)
	if resp, data := getAvatar(anna, "2", "If-None-Match", etag); resp.StatusCode != http.StatusOK || !isBlue(centerColor(t, data)) {
		t.Errorf("new picture: status %d, want the new blue one", resp.StatusCode)
	}

	// Only images.
	anna.do("PUT", "/api/me/avatar", "MZ\x90\x00 not an image", http.StatusBadRequest, nil)

	// Users remove their own; admins remove anyone's; users can't remove others'.
	admin.do("PUT", "/api/me/avatar", string(red), http.StatusOK, nil)
	anna.do("DELETE", "/api/users/1/avatar", "", http.StatusForbidden, nil)
	admin.do("DELETE", "/api/users/2/avatar", "", http.StatusOK, &u)
	if u.HasAvatar {
		t.Error("anna still has a picture after admin removed it")
	}
	if resp, _ := getAvatar(anna, "2"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("after admin delete: status %d, want 404", resp.StatusCode)
	}
	admin.do("DELETE", "/api/me/avatar", "", http.StatusOK, &u)
	if u.HasAvatar {
		t.Error("admin still has a picture after deleting it")
	}
	// Deleting when there is none is fine.
	admin.do("DELETE", "/api/me/avatar", "", http.StatusOK, nil)
	admin.do("DELETE", "/api/users/99/avatar", "", http.StatusNotFound, nil)
	if resp, _ := getAvatar(admin, "99"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown user: status %d, want 404", resp.StatusCode)
	}

	// A deleted user's picture goes with them.
	anna.do("PUT", "/api/me/avatar", string(red), http.StatusOK, nil)
	admin.do("DELETE", "/api/users/2", "", http.StatusNoContent, nil)
	if _, err := env.store.Avatar(2); err == nil {
		t.Error("picture of deleted user is still stored")
	}

	if resp, _ := getAvatar(env.anon(t), "1"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("without login: status %d, want 401", resp.StatusCode)
	}
}
