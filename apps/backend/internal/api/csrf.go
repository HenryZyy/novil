package api

import (
	"net/http"
	"net/netip"
	"net/url"
	"slices"
)

// fromOwnPages reports whether a request that changes something may use
// the login cookie: it must come from novil's own pages, not from another
// site the user has open (CSRF). Browsers say where a request comes from in
// Sec-Fetch-Site, older ones in Origin. Requests without either aren't from
// a browser page, so another site can't have sent them.
func (s *Server) fromOwnPages(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "":
	default:
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if slices.Contains(s.opts.CORSOrigins, origin) {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

// secureRequest reports whether the browser reached novil over HTTPS: by
// TLS to novil itself, or to a trusted reverse proxy that says so.
func (s *Server) secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && isTrusted(peer.Addr().Unmap(), s.trustedProxies) && r.Header.Get("X-Forwarded-Proto") == "https"
}
