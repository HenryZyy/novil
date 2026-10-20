package api

import (
	"net/http"
	"strings"
)

// The web app's Content-Security-Policy. Most of all it keeps scripts in
// books from running: the EPUB reader shows books in frames made from blob:
// URLs, which inherit this policy, and scripts may only come from novil's
// own files. Styles may be inline (books and React use them), images come
// from novil, books (blob:, data:) and the book databases (https:).
const appPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' blob:; " +
	"img-src 'self' blob: data: https:; font-src 'self' blob: data:; connect-src 'self' blob:; " +
	"media-src 'self' blob:; frame-src 'self' blob:; object-src 'none'; base-uri 'self'; " +
	"form-action 'self'; frame-ancestors 'none'"

// API answers are data, never pages to show.
const apiPolicy = "default-src 'none'; frame-ancestors 'none'; sandbox"

// securityHeaders adds the headers that keep browsers from misreading
// novil's answers (nosniff), framing them (clickjacking) or telling other
// sites which pages were open (referrer).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Content-Security-Policy", apiPolicy)
		} else {
			h.Set("Content-Security-Policy", appPolicy)
		}
		next.ServeHTTP(w, r)
	})
}
