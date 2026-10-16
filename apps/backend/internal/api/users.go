package api

import (
	"errors"
	"net/http"
	"strings"

	"novil/backend/internal/auth"
	"novil/backend/internal/store"
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	respond(w, users, err)
}

// createUser adds an account with a starting password, which the new user
// must change after their first login.
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Role == "" {
		req.Role = store.RoleUser
	}
	if req.Role != store.RoleUser && req.Role != store.RoleAdmin {
		writeError(w, http.StatusBadRequest, `role must be "user" or "admin"`)
		return
	}
	if err := auth.ValidateUsername(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internalError(w, err)
		return
	}
	user, err := s.store.CreateUser(req.Username, hash, req.Role, true)
	if errors.Is(err, store.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, "username is already taken")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

// resetPassword sets a new starting password, for users who forgot theirs.
// It logs them out and they must change it again after logging in.
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internalError(w, err)
		return
	}
	user, err := s.store.SetPassword(id, hash, true)
	respond(w, user, err)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// This also ensures there's always an admin left: the caller is one.
	if id == currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "you can't delete your own account")
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
