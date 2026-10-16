package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"novil/backend/internal/auth"
	"novil/backend/internal/store"
)

// session is the response to a login or password change.
type session struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expiresAt"`
	User      store.User `json:"user"`
}

func (s *Server) writeSession(w http.ResponseWriter, user store.User) {
	token, exp, err := s.tokens.Issue(user.ID, user.TokenVersion)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session{Token: token, ExpiresAt: exp, User: user})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, err := s.store.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		internalError(w, err)
		return
	}
	// A missing user has an empty hash, which CheckPassword rejects as slowly
	// as a wrong password. Both get the same answer.
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeCodedError(w, http.StatusUnauthorized, "invalid_credentials", "wrong username or password")
		return
	}
	s.writeSession(w, user)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

// changePassword lets users replace their password, including the one an
// admin gave them. It returns a new token, since the old one stops working.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		writeCodedError(w, http.StatusForbidden, "invalid_credentials", "current password is wrong")
		return
	}
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "new password must differ from the current one")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		internalError(w, err)
		return
	}
	user, err = s.store.SetPassword(user.ID, hash, false)
	if err != nil {
		respond(w, nil, err)
		return
	}
	s.writeSession(w, user)
}
