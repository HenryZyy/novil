package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

const firefox = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0"
const androidPhone = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36"

func TestActivityLog(t *testing.T) {
	env := newTestServer(t)
	env.anon(t).do("POST", "/api/auth/login", `{"username":"admin","password":"wrong-password-1"}`, http.StatusUnauthorized, nil)
	env.anon(t).do("POST", "/api/auth/login", `{"username":"nobody","password":"wrong-password-1"}`, http.StatusUnauthorized, nil)
	addReader(t, env)
	env.admin.do("PATCH", "/api/users/2", `{"role":"admin"}`, http.StatusOK, nil)
	env.admin.do("DELETE", "/api/users/2", "", http.StatusNoContent, nil)

	var log []store.LogEntry
	env.admin.do("GET", "/api/logs", "", http.StatusOK, &log)
	events := []string{}
	for _, e := range log {
		events = append(events, e.Event+":"+e.Username+":"+e.Detail)
	}
	want := []string{
		"user_deleted:admin:@reader",
		"user_updated:admin:@reader is now admin",
		"login:reader:",
		"login_failed:nobody:",
		"login_failed:admin:",
		"login:admin:",
	}
	if len(events) != len(want) {
		t.Fatalf("log %q, want %q", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("entry %d: %q, want %q", i, events[i], want[i])
		}
	}
	// The deleted reader's login keeps the name, but not the ID.
	if log[2].UserID != nil || log[5].UserID == nil || log[3].UserID != nil || log[4].UserID == nil {
		t.Errorf("user IDs %+v", log)
	}
	if log[0].IP == "" {
		t.Errorf("no IP in %+v", log[0])
	}

	env.admin.do("GET", "/api/logs?event=login_failed", "", http.StatusOK, &log)
	if len(log) != 2 {
		t.Errorf("failed logins %+v", log)
	}
	env.admin.do("GET", "/api/logs?search=nobody", "", http.StatusOK, &log)
	if len(log) != 1 {
		t.Errorf("search %+v", log)
	}
	var all []store.LogEntry
	env.admin.do("GET", "/api/logs", "", http.StatusOK, &all)
	env.admin.do("GET", "/api/logs?before="+itoa(all[1].ID), "", http.StatusOK, &log)
	if len(log) != len(all)-2 {
		t.Errorf("page after the second: %d, want %d", len(log), len(all)-2)
	}
	env.admin.do("GET", "/api/logs?event=hacked", "", http.StatusBadRequest, nil)
	readerAgain := addReader(t, env)
	readerAgain.do("GET", "/api/logs", "", http.StatusForbidden, nil)
}

func TestReadingNow(t *testing.T) {
	env := newTestServer(t)
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Akira", "Vol 1.cbz"), "01.jpg", "02.jpg")
	env.admin.do("POST", "/api/libraries", libraryJSON(t, "Manga", root), http.StatusCreated, nil)
	env.admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	reader := addReader(t, env)

	reader.userAgent = androidPhone
	reader.do("PUT", "/api/books/1/progress", `{"page":2}`, http.StatusOK, nil)
	reader.do("PUT", "/api/me/reading", `{"bookId":1}`, http.StatusNoContent, nil)
	reader.do("PUT", "/api/me/reading", `{"bookId":1}`, http.StatusNoContent, nil)
	reader.do("PUT", "/api/me/reading", `{"bookId":99}`, http.StatusNotFound, nil)
	env.admin.userAgent = firefox
	env.admin.do("PUT", "/api/me/reading", `{"bookId":1}`, http.StatusNoContent, nil)

	var now []store.ReadingSession
	env.admin.do("GET", "/api/reading", "", http.StatusOK, &now)
	if len(now) != 2 {
		t.Fatalf("reading %+v", now)
	}
	byName := map[string]store.ReadingSession{}
	for _, s := range now {
		byName[s.User.Username] = s
	}
	r := byName["reader"]
	if r.Device != "Chrome on Android" || r.DeviceKind != "phone" || r.Book.Title != "Vol 1" || r.Book.ReadPage != 2 {
		t.Errorf("reader's session %+v", r)
	}
	if byName["admin"].Device != "Firefox on Windows" || byName["admin"].Book.ReadPage != 0 {
		t.Errorf("admin's session %+v", byName["admin"])
	}

	reader.do("DELETE", "/api/me/reading", "", http.StatusNoContent, nil)
	env.admin.do("GET", "/api/reading", "", http.StatusOK, &now)
	if len(now) != 1 || now[0].User.Username != "admin" {
		t.Errorf("after closing %+v", now)
	}
	// A book hidden from the reader drops out of their session.
	reader.do("PUT", "/api/me/reading", `{"bookId":1}`, http.StatusNoContent, nil)
	env.admin.do("PATCH", "/api/books/1", `{"hidden":true}`, http.StatusOK, nil)
	env.admin.do("GET", "/api/reading", "", http.StatusOK, &now)
	if len(now) != 1 {
		t.Errorf("with a hidden book %+v", now)
	}
	reader.do("GET", "/api/reading", "", http.StatusForbidden, nil)
}
