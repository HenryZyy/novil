package store

import (
	"cmp"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrAlreadyRequested means the user already asked for the book.
var ErrAlreadyRequested = errors.New("already requested")

type BookRequest struct {
	ID        int64  `json:"id"`
	ISBN      string `json:"isbn"`
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Published string `json:"published"`
	CoverURL  string `json:"coverUrl"`
	// Status is "open", "added" (BookID is the book) or "declined".
	Status    string `json:"status"`
	BookID    *int64 `json:"bookId"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	// How many users asked for it, and whether the viewer is one of them.
	RequestCount  int  `json:"requestCount"`
	RequestedByMe bool `json:"requestedByMe"`
	// RequestedAt is when the viewer asked for it.
	RequestedAt *string `json:"requestedAt"`
	// Requesters are their usernames, for admins only.
	Requesters []string `json:"requesters,omitempty"`
}

// requestSelect reads requests as user ?1 sees them.
const requestSelect = `
	SELECT r.id, r.isbn, r.title, r.authors, r.published, r.cover_url, r.status, r.book_id,
		r.created_at, r.updated_at,
		(SELECT COUNT(*) FROM book_request_votes WHERE request_id = r.id),
		(SELECT created_at FROM book_request_votes WHERE request_id = r.id AND user_id = ?1),
		COALESCE((SELECT group_concat(u.username, char(31)) FROM book_request_votes v
			JOIN users u ON u.id = v.user_id WHERE v.request_id = r.id), '')
	FROM book_requests r`

func scanRequest(row interface{ Scan(...any) error }) (BookRequest, error) {
	var r BookRequest
	var requesters string
	err := row.Scan(&r.ID, &r.ISBN, &r.Title, &r.Authors, &r.Published, &r.CoverURL, &r.Status, &r.BookID,
		&r.CreatedAt, &r.UpdatedAt, &r.RequestCount, &r.RequestedAt, &requesters)
	r.RequestedByMe = r.RequestedAt != nil
	if requesters != "" {
		r.Requesters = strings.Split(requesters, "\x1f")
	}
	return r, err
}

func (s *Store) listRequests(query string, args ...any) ([]BookRequest, error) {
	rows, err := s.db.Query(requestSelect+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BookRequest{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRequests returns every request with the given status (all for ""),
// the most wanted first, then the oldest.
func (s *Store) ListRequests(viewerID int64, status string) ([]BookRequest, error) {
	return s.listRequests(` WHERE ?2 = '' OR r.status = ?2
		ORDER BY r.status != 'open', 11 DESC, r.created_at`, viewerID, status)
}

// MyRequests returns the requests the user asked for, newest first.
func (s *Store) MyRequests(userID int64) ([]BookRequest, error) {
	return s.listRequests(` WHERE EXISTS (SELECT 1 FROM book_request_votes WHERE request_id = r.id AND user_id = ?1)
		ORDER BY 12 DESC`, userID)
}

func (s *Store) GetRequest(viewerID, id int64) (BookRequest, error) {
	r, err := scanRequest(s.db.QueryRow(requestSelect+` WHERE r.id = ?2`, viewerID, id))
	return r, notFound(err)
}

func (s *Store) GetRequestByISBN(viewerID int64, isbn string) (BookRequest, error) {
	r, err := scanRequest(s.db.QueryRow(requestSelect+` WHERE r.isbn = ?2`, viewerID, isbn))
	return r, notFound(err)
}

// Request records that the user asks for the book. The first user to ask
// creates the request from r; later ones join it, and asking for a
// declined book opens it again. Asking twice is ErrAlreadyRequested.
func (s *Store) Request(userID int64, r BookRequest) (BookRequest, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return BookRequest{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(timeMillis)
	var id int64
	err = tx.QueryRow(`
		INSERT INTO book_requests (isbn, title, authors, published, cover_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (isbn) DO UPDATE SET
			status = CASE WHEN status = 'declined' THEN 'open' ELSE status END,
			updated_at = CASE WHEN status = 'declined' THEN excluded.updated_at ELSE updated_at END
		RETURNING id`, r.ISBN, r.Title, r.Authors, r.Published, r.CoverURL, now, now).Scan(&id)
	if err != nil {
		return BookRequest{}, err
	}
	res, err := tx.Exec(`INSERT INTO book_request_votes (request_id, user_id, created_at) VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING`, id, userID, now)
	if err != nil {
		return BookRequest{}, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return BookRequest{}, cmp.Or(err, ErrAlreadyRequested)
	}
	if err := tx.Commit(); err != nil {
		return BookRequest{}, err
	}
	return s.GetRequest(userID, id)
}

// Withdraw takes back the user's request. An open request nobody wants
// any more is deleted.
func (s *Store) Withdraw(userID, requestID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM book_request_votes WHERE request_id = ? AND user_id = ?`, requestID, userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return cmp.Or(err, ErrNotFound)
	}
	if _, err := tx.Exec(`DELETE FROM book_requests WHERE id = ? AND status = 'open'
		AND NOT EXISTS (SELECT 1 FROM book_request_votes WHERE request_id = ?)`, requestID, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetRequestStatus marks a request as added (with the book, if known),
// declined or open again.
func (s *Store) SetRequestStatus(id int64, status string, bookID *int64) error {
	res, err := s.db.Exec(`UPDATE book_requests SET status = ?, book_id = ?, updated_at = ? WHERE id = ?`,
		status, bookID, time.Now().UTC().Format(timeMillis), id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return cmp.Or(err, ErrNotFound)
	}
	return nil
}

func (s *Store) DeleteRequest(id int64) error {
	res, err := s.db.Exec(`DELETE FROM book_requests WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return cmp.Or(err, ErrNotFound)
	}
	return nil
}

// RequestCover returns a request's saved cover, nil if it has none yet.
func (s *Store) RequestCover(id int64) (data []byte, coverURL string, err error) {
	err = s.db.QueryRow(`SELECT cover, cover_url FROM book_requests WHERE id = ?`, id).Scan(&data, &coverURL)
	return data, coverURL, notFound(err)
}

func (s *Store) SetRequestCover(id int64, data []byte) error {
	_, err := s.db.Exec(`UPDATE book_requests SET cover = ? WHERE id = ?`, data, id)
	return err
}

// CachedLookup returns what an ISBN lookup found, as JSON, if it was done
// since the given times: found books keep longer than "not found".
func (s *Store) CachedLookup(isbn string, foundSince, missingSince time.Time) (info string, found, ok bool, err error) {
	var fetched string
	err = s.db.QueryRow(`SELECT info, found, fetched_at FROM isbn_lookups WHERE isbn = ?`, isbn).Scan(&info, &found, &fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	since := missingSince
	if found {
		since = foundSince
	}
	return info, found, fetched >= since.UTC().Format(timeMillis), nil
}

func (s *Store) SaveLookup(isbn string, found bool, info string) error {
	_, err := s.db.Exec(`
		INSERT INTO isbn_lookups (isbn, found, info, fetched_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (isbn) DO UPDATE SET found = excluded.found, info = excluded.info, fetched_at = excluded.fetched_at`,
		isbn, found, info, time.Now().UTC().Format(timeMillis))
	return err
}

// RecentRequests returns when the user asked for books since a time,
// oldest first.
func (s *Store) RecentRequests(userID int64, since time.Time) ([]time.Time, error) {
	rows, err := s.db.Query(`SELECT created_at FROM book_request_votes WHERE user_id = ? AND created_at >= ? ORDER BY created_at`,
		userID, since.UTC().Format(timeMillis))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var at string
		if err := rows.Scan(&at); err != nil {
			return nil, err
		}
		t, err := time.Parse(timeMillis, at)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
