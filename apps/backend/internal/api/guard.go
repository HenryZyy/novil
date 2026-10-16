package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

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

type userKey struct{}

// guard wraps a handler so it only runs for requests with a valid
// "Authorization: Bearer <token>" header from a user with the given access.
// The handler gets that user with currentUser.
func (s *Server) guard(a access, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticate(w, r)
		if !ok {
			return
		}
		if user.MustChangePassword && a != pendingPassword {
			writeCodedError(w, http.StatusForbidden, "password_change_required", "change your password first")
			return
		}
		if a == adminOnly && user.Role != store.RoleAdmin {
			writeCodedError(w, http.StatusForbidden, "forbidden", "only admins can do this")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	}
}

// authenticate loads the user the request's token belongs to, writing a 401
// response and returning false if there is no valid token.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	token, ok := bearerToken(r)
	if !ok {
		unauthorized(w, "missing bearer token")
		return store.User{}, false
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		unauthorized(w, "invalid or expired token")
		return store.User{}, false
	}
	user, err := s.store.GetUser(claims.UserID)
	if errors.Is(err, store.ErrNotFound) {
		unauthorized(w, "invalid or expired token")
		return store.User{}, false
	}
	if err != nil {
		internalError(w, err)
		return store.User{}, false
	}
	// The password changed since the token was issued.
	if user.TokenVersion != claims.Version {
		unauthorized(w, "invalid or expired token")
		return store.User{}, false
	}
	return user, true
}

// currentUser returns the user a guard let through. Only call it in
// handlers wrapped with guard.
func currentUser(r *http.Request) store.User {
	return r.Context().Value(userKey{}).(store.User)
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
