package api

import (
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	env := newTestServer(t)
	resp := env.admin.request("GET", "/api/me", "")
	resp.Body.Close()
	for k, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": apiPolicy,
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
	// The app's policy allows no scripts but novil's own files.
	if !strings.Contains(appPolicy, "script-src 'self';") || strings.Contains(appPolicy, "unsafe-eval") {
		t.Errorf("app policy %q", appPolicy)
	}
}
