package api

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"novil/backend/internal/scanner"
)

func (s *Server) uploadBook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	lib, err := s.store.GetLibrary(id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if !lib.AllowUploads {
		writeError(w, http.StatusForbidden, "uploads are turned off for this library")
		return
	}
	series, ok := cleanName(r.URL.Query().Get("series"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid series name")
		return
	}
	filename, ok := cleanName(r.URL.Query().Get("filename"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid file name")
		return
	}
	format, ok := scanner.Formats[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		writeError(w, http.StatusBadRequest, "only cbz, epub and pdf files can be uploaded")
		return
	}
	if r.ContentLength > s.opts.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}

	dir := filepath.Join(lib.Path, series)
	dest := filepath.Join(dir, filename)
	if _, err := os.Lstat(dest); err == nil {
		writeError(w, http.StatusConflict, "a file with this name already exists in this series")
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeFileError(w, err)
		return
	}
	// Hidden, so scans skip it.
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		writeFileError(w, err)
		return
	}
	defer os.Remove(tmp.Name())

	_, err = io.Copy(tmp, http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "upload was interrupted")
		return
	}
	if err := scanner.Validate(tmp.Name(), format); err != nil {
		writeError(w, http.StatusBadRequest, "this isn't a valid "+format+" file")
		return
	}
	if err := moveNoReplace(tmp.Name(), dest); errors.Is(err, fs.ErrExist) {
		writeError(w, http.StatusConflict, "a file with this name already exists in this series")
		return
	} else if err != nil {
		writeFileError(w, err)
		return
	}

	found, err := scanner.ScanSeries(lib.Path, dir)
	if err != nil {
		internalError(w, err)
		return
	}
	if err := s.store.SyncSeries(lib.ID, found); err != nil {
		internalError(w, err)
		return
	}
	book, err := s.store.GetBookByPath(dest)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func cleanName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\x00") {
		return "", false
	}
	return name, true
}

// os.Link fails if dst exists, unlike os.Rename.
func moveNoReplace(src, dst string) error {
	err := os.Link(src, dst)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return fs.ErrExist
	}
	return os.Rename(src, dst)
}

func writeFileError(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EROFS) {
		slog.Error("library folder is not writable", "err", err)
		writeError(w, http.StatusInternalServerError, "the server can't write to this library's folder")
		return
	}
	internalError(w, err)
}
