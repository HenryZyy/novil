package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"novil/backend/internal/metadata"
	"novil/backend/internal/store"
)

// maxFieldLength limits each metadata field, in characters.
var maxFieldLength = map[string]int{
	"title":       500,
	"author":      500,
	"language":    35,
	"released":    10,
	"description": 20000,
}

// updateBook lets admins correct a book's metadata and hide it from users.
// Each metadata field in the body replaces the file's value, null goes back
// to the file's value, and missing fields stay as they are:
//
//	{"title": "Dune", "author": null, "hidden": true}
func (s *Server) updateBook(w http.ResponseWriter, r *http.Request) {
	b, ok := s.book(w, r)
	if !ok {
		return
	}
	// A map instead of a struct tells a missing field from a null one.
	var req map[string]json.RawMessage
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req) == 0 {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	var hidden *bool
	if raw, ok := req["hidden"]; ok {
		if err := json.Unmarshal(raw, &hidden); err != nil || hidden == nil {
			writeError(w, http.StatusBadRequest, "hidden must be true or false")
			return
		}
		delete(req, "hidden")
	}
	changes := map[string]*string{}
	for field, raw := range req {
		if !slices.Contains(store.MetadataFields, field) {
			writeError(w, http.StatusBadRequest, "unknown field "+field+"; editable are hidden, "+strings.Join(store.MetadataFields, ", "))
			return
		}
		if string(raw) == "null" {
			changes[field] = nil
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			writeError(w, http.StatusBadRequest, field+" must be a string or null")
			return
		}
		v = strings.TrimSpace(v)
		if msg := checkField(field, v); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		changes[field] = &v
	}
	if len(changes) > 0 {
		if err := s.store.SetMetadata(b.ID, changes); err != nil {
			internalError(w, err)
			return
		}
	}
	if hidden != nil {
		if err := s.store.SetHidden(b.ID, *hidden); err != nil {
			internalError(w, err)
			return
		}
	}
	b, err := s.store.GetBook(currentUser(r).ID, b.ID)
	respond(w, b, err)
}

// checkField returns why v can't be stored in field, or "" if it can.
func checkField(field, v string) string {
	if utf8.RuneCountInString(v) > maxFieldLength[field] {
		return field + " is too long"
	}
	switch field {
	case "title":
		if v == "" {
			return "title can't be empty"
		}
	case "released":
		if !metadata.ValidDate(v) {
			return "released must look like 2006, 2006-01 or 2006-01-02"
		}
	}
	return ""
}
