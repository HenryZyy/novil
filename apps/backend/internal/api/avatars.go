package api

import (
	"bytes"
	"errors"
	"net/http"
	"time"

	"novil/backend/internal/cover"
	"novil/backend/internal/store"
)

// maxAvatarUploadBytes limits uploaded profile pictures.
const maxAvatarUploadBytes = 20 << 20

// setMyAvatar stores the uploaded image (the raw file as the request body)
// as the user's profile picture. JPEG, PNG, GIF and WebP work.
func (s *Server) setMyAvatar(w http.ResponseWriter, r *http.Request) {
	data, err := cover.AvatarFromUpload(http.MaxBytesReader(w, r.Body, maxAvatarUploadBytes))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "the image is too large")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, "the picture must be a JPEG, PNG, GIF or WebP image of at most 50 megapixels")
		return
	}
	user, err := s.store.SetAvatar(currentUser(r).ID, data)
	respond(w, user, err)
}

func (s *Server) deleteMyAvatar(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.DeleteAvatar(currentUser(r).ID)
	respond(w, user, err)
}

// deleteUserAvatar lets admins remove someone's unsuitable picture.
func (s *Server) deleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.store.DeleteAvatar(id)
	respond(w, user, err)
}

// getAvatar sends a user's profile picture as a JPEG, or 404 without one.
func (s *Server) getAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.store.GetUser(id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if !user.HasAvatar {
		writeError(w, http.StatusNotFound, "this user has no profile picture")
		return
	}
	data, err := s.store.Avatar(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "this user has no profile picture")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	// Like covers: the browser keeps it but checks it's current, which
	// costs a 304 while it is.
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", `"avatar-`+user.AvatarUpdatedAt+`"`)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}
