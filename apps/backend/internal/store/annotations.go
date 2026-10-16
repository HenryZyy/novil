package store

import (
	"cmp"
	"time"
)

type Annotation struct {
	ID     int64 `json:"id"`
	BookID int64 `json:"bookId"`
	// Kind is "bookmark" or "highlight".
	Kind string `json:"kind"`
	// Page is the page of a comic, or the percentage read of an EPUB.
	Page int `json:"page"`
	// Position is the EPUB CFI: a point for bookmarks, a range for
	// highlights. Empty for comics.
	Position  string `json:"position"`
	Text      string `json:"text"`
	Note      string `json:"note"`
	Color     string `json:"color"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

const annotationColumns = `id, book_id, kind, page, position, text, note, color, created_at, updated_at`

func scanAnnotation(row interface{ Scan(...any) error }) (Annotation, error) {
	var a Annotation
	err := row.Scan(&a.ID, &a.BookID, &a.Kind, &a.Page, &a.Position, &a.Text, &a.Note, &a.Color, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// ListAnnotations returns a user's annotations in a book, in reading order.
func (s *Store) ListAnnotations(userID, bookID int64) ([]Annotation, error) {
	rows, err := s.db.Query(`SELECT `+annotationColumns+` FROM annotations
		WHERE user_id = ? AND book_id = ? ORDER BY page, created_at`, userID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Annotation{}
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateAnnotation(userID int64, a Annotation) (Annotation, error) {
	now := time.Now().UTC().Format(timeMillis)
	return scanAnnotation(s.db.QueryRow(`
		INSERT INTO annotations (user_id, book_id, kind, page, position, text, note, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+annotationColumns,
		userID, a.BookID, a.Kind, a.Page, a.Position, a.Text, a.Note, a.Color, now, now))
}

// GetAnnotation returns one of the user's annotations; other users' are
// ErrNotFound.
func (s *Store) GetAnnotation(userID, id int64) (Annotation, error) {
	a, err := scanAnnotation(s.db.QueryRow(`SELECT `+annotationColumns+` FROM annotations
		WHERE id = ? AND user_id = ?`, id, userID))
	return a, notFound(err)
}

// UpdateAnnotation changes the note or colour; nil stays as it is.
func (s *Store) UpdateAnnotation(userID, id int64, note, color *string) (Annotation, error) {
	a, err := scanAnnotation(s.db.QueryRow(`
		UPDATE annotations SET note = COALESCE(?, note), color = COALESCE(?, color), updated_at = ?
		WHERE id = ? AND user_id = ? RETURNING `+annotationColumns,
		note, color, time.Now().UTC().Format(timeMillis), id, userID))
	return a, notFound(err)
}

func (s *Store) DeleteAnnotation(userID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM annotations WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return cmp.Or(err, ErrNotFound)
	}
	return nil
}

// Lists are the series lists users keep, as named in the series_lists
// table and the API.
var Lists = []string{"favorites", "want"}

// SetListed adds a series to one of the user's lists, or removes it.
func (s *Store) SetListed(userID, seriesID int64, list string, listed bool) error {
	if !listed {
		_, err := s.db.Exec(`DELETE FROM series_lists WHERE user_id = ? AND series_id = ? AND list = ?`, userID, seriesID, list)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO series_lists (user_id, series_id, list, added_at) VALUES (?, ?, ?, ?)
		ON CONFLICT DO NOTHING`, userID, seriesID, list, time.Now().UTC().Format(timeMillis))
	return err
}
