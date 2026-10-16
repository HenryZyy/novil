package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"novil/backend/internal/auth"
	"novil/backend/internal/store"
)

// access says who may call a route.
type access int

const (
	// member: any logged-in user who has set their own password.
	member access = iota
	// pendingPassword: like member, but also lets in users who still have
	// to change the password an admin gave them.
	pendingPassword
	// adminOnly: only admins who have set their own password.
	adminOnly
)

type authKey struct{}

// authInfo is what a guard attaches to the request.
type authInfo struct {
	user   store.User
	claims auth.Claims
}

// guard wraps a handler so it only runs for requests with a valid
// "Authorization: Bearer <token>" header from a user with the given access.
// The handler gets that user with currentUser.
func (s *Server) guard(a access, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info, ok := s.authenticate(w, r)
		if !ok {
			return
		}
		if info.user.MustChangePassword && a != pendingPassword {
			writeCodedError(w, http.StatusForbidden, "password_change_required", "change your password first")
			return
		}
		if a == adminOnly && info.user.Role != store.RoleAdmin {
			writeCodedError(w, http.StatusForbidden, "forbidden", "only admins can do this")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), authKey{}, info)))
	}
}

// authenticate loads the user the request's token belongs to, writing a 401
// response and returning false if there is no valid token.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (authInfo, bool) {
	token, ok := bearerToken(r)
	if !ok {
		unauthorized(w, "missing bearer token")
		return authInfo{}, false
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		unauthorized(w, "invalid or expired token")
		return authInfo{}, false
	}
	user, err := s.store.GetUser(claims.UserID)
	if errors.Is(err, store.ErrNotFound) {
		unauthorized(w, "invalid or expired token")
		return authInfo{}, false
	}
	if err != nil {
		internalError(w, err)
		return authInfo{}, false
	}
	// The password changed since the token was issued.
	if user.TokenVersion != claims.Version {
		unauthorized(w, "invalid or expired token")
		return authInfo{}, false
	}
	revoked, err := s.store.IsTokenRevoked(claims.ID)
	if err != nil {
		internalError(w, err)
		return authInfo{}, false
	}
	if revoked {
		unauthorized(w, "invalid or expired token")
		return authInfo{}, false
	}
	return authInfo{user: user, claims: claims}, true
}

// currentUser returns the user a guard let through. Only call it in
// handlers wrapped with guard.
func currentUser(r *http.Request) store.User {
	return r.Context().Value(authKey{}).(authInfo).user
}

// currentClaims returns the claims of the token a guard let through.
func currentClaims(r *http.Request) auth.Claims {
	return r.Context().Value(authKey{}).(authInfo).claims
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeCodedError(w, http.StatusUnauthorized, "unauthorized", msg)
}
