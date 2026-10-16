package store

import (
	"database/sql"
	"errors"
	"time"
)

// CustomCover returns the cover an admin uploaded for a book, and when, or
// ErrNotFound.
func (s *Store) CustomCover(bookID int64) ([]byte, string, error) {
	var data []byte
	var updated string
	err := s.db.QueryRow(`SELECT data, updated_at FROM book_covers WHERE book_id = ?`, bookID).Scan(&data, &updated)
	return data, updated, notFound(err)
}

func (s *Store) SetCustomCover(bookID int64, data []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO book_covers (book_id, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (book_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		bookID, data, time.Now().UTC().Format(timeMillis))
	return err
}

// DeleteCustomCover goes back to the cover made from the file.
func (s *Store) DeleteCustomCover(bookID int64) error {
	_, err := s.db.Exec(`DELETE FROM book_covers WHERE book_id = ?`, bookID)
	return err
}

// CachedCover returns the cover made earlier from a book's file. found is
// false if none was made from this version of the file yet. data is nil if
// the file has no usable cover.
func (s *Store) CachedCover(bookID int64, fileKey string) (data []byte, found bool, err error) {
	err = s.db.QueryRow(`SELECT data FROM cover_cache WHERE book_id = ? AND file_key = ?`, bookID, fileKey).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return data, err == nil, err
}

// SetCachedCover stores a cover made from the file; nil data records that
// the file has none.
func (s *Store) SetCachedCover(bookID int64, fileKey string, data []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO cover_cache (book_id, file_key, data) VALUES (?, ?, ?)
		ON CONFLICT (book_id) DO UPDATE SET file_key = excluded.file_key, data = excluded.data`,
		bookID, fileKey, data)
	return err
}
