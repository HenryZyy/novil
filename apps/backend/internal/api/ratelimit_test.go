package api

import (
	"net/http"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func TestLogout(t *testing.T) {
	env := newTestServer(t)
	phone := env.login(t, "admin", "admin-password")
	laptop := env.admin

	phone.do("POST", "/api/auth/logout", "", http.StatusNoContent, nil)
	phone.do("GET", "/api/me", "", http.StatusUnauthorized, nil)
	phone.do("POST", "/api/auth/logout", "", http.StatusUnauthorized, nil)
	// Only that one token is logged out.
	laptop.do("GET", "/api/me", "", http.StatusOK, nil)
}

func TestLoginRateLimit(t *testing.T) {
	// newTestServerWith logs in once, so 3 attempts are left.
	env := newTestServerWith(t, Options{LoginBurst: 4, LoginInterval: time.Minute})
	anon := env.anon(t)
	for range 3 {
		anon.do("POST", "/api/auth/login", `{"username":"admin","password":"wrong-password"}`, http.StatusUnauthorized, nil)
	}

	// Now even the right password is refused until the bucket refills.
	resp := anon.request("POST", "/api/auth/login", `{"username":"admin","password":"admin-password"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", resp.StatusCode)
	}
	if s, _ := strconv.Atoi(resp.Header.Get("Retry-After")); s < 1 || s > 60 {
		t.Errorf("Retry-After %q, want 1-60 seconds", resp.Header.Get("Retry-After"))
	}

	// Refused attempts don't count, so the wait doesn't grow.
	resp = anon.request("POST", "/api/auth/login", `{}`)
	resp.Body.Close()
	if s, _ := strconv.Atoi(resp.Header.Get("Retry-After")); s > 60 {
		t.Errorf("Retry-After grew to %d", s)
	}

	// Password changes share the limit, since they check a password too.
	env.admin.do("PUT", "/api/me/password", `{"currentPassword":"admin-password","newPassword":"new-password"}`, http.StatusTooManyRequests, nil)
}

func TestLimiterForgetsIdleClients(t *testing.T) {
	l := newLimiter(time.Millisecond, 1)
	l.wait("a")
	time.Sleep(5 * time.Millisecond)
	l.lastSweep = time.Time{}
	l.wait("b")
	if _, ok := l.clients["a"]; ok {
		t.Error("idle client a was not forgotten")
	}
}

func TestClientIP(t *testing.T) {
	proxy := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		trusted    []netip.Prefix
		want       string
	}{
		{"direct", "203.0.113.5:1234", nil, nil, "203.0.113.5"},
		{"header ignored without trusted proxies", "203.0.113.5:1234", []string{"1.2.3.4"}, nil, "203.0.113.5"},
		{"header ignored from untrusted sender", "203.0.113.5:1234", []string{"1.2.3.4"}, proxy, "203.0.113.5"},
		{"behind proxy", "10.0.0.2:1234", []string{"198.51.100.7"}, proxy, "198.51.100.7"},
		// The client can put anything on the left; only the entry our proxy
		// added counts.
		{"spoofed entries skipped", "10.0.0.2:1234", []string{"1.2.3.4, 198.51.100.7"}, proxy, "198.51.100.7"},
		{"two proxies", "10.0.0.2:1234", []string{"198.51.100.7, 10.0.0.3"}, proxy, "198.51.100.7"},
		{"several headers", "10.0.0.2:1234", []string{"1.2.3.4", "198.51.100.7"}, proxy, "198.51.100.7"},
		{"garbage stops the walk", "10.0.0.2:1234", []string{"198.51.100.7, nonsense"}, proxy, "10.0.0.2"},
		{"ipv6", "[2001:db8::1]:1234", nil, nil, "2001:db8::1"},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest("GET", "/", nil)
		r.RemoteAddr = tt.remoteAddr
		for _, v := range tt.forwarded {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := clientIP(r, tt.trusted).String(); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestClientKeyGroupsIPv6(t *testing.T) {
	a := clientKey(netip.MustParseAddr("2001:db8:1:2::1"))
	b := clientKey(netip.MustParseAddr("2001:db8:1:2:ffff::9"))
	c := clientKey(netip.MustParseAddr("2001:db8:1:3::1"))
	if a != b {
		t.Errorf("same /64 got different keys %s and %s", a, b)
	}
	if a == c {
		t.Errorf("different /64 got the same key %s", a)
	}
	if k := clientKey(netip.MustParseAddr("203.0.113.5")); k != "203.0.113.5" {
		t.Errorf("ipv4 key %s", k)
	}
}
