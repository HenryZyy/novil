package store

import (
	"errors"
	"time"
)

// LogEntry is one thing that happened, as admins see it in the log.
type LogEntry struct {
	ID int64  `json:"id"`
	At string `json:"at"`
	// Event is e.g. "login", "login_failed" or "user_deleted".
	Event string `json:"event"`
	// UserID and Username are who did it, or for a failed login the name
	// tried. UserID is nil when there's no such user (any more).
	UserID   *int64 `json:"userId"`
	Username string `json:"username"`
	IP       string `json:"ip"`
	// DeviceKind is "phone", "tablet", "desktop" or "" if unknown; Device
	// reads like "Firefox on Windows".
	DeviceKind string `json:"deviceKind"`
	Device     string `json:"device"`
	// Detail is what the event was about, e.g. the user who was deleted.
	Detail string `json:"detail"`
}

// logDays is how long the log is kept.
const logDays = 90

func (s *Store) AddLog(e LogEntry) error {
	now := time.Now().UTC()
	if _, err := s.db.Exec(`
		INSERT INTO activity_log (at, event, user_id, username, ip, device_kind, device, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		now.Format(timeMillis), e.Event, e.UserID, e.Username, e.IP, e.DeviceKind, e.Device, e.Detail); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM activity_log WHERE at < ?`, now.AddDate(0, 0, -logDays).Format(timeMillis))
	return err
}

type LogQuery struct {
	// Events are the events to list; empty is all.
	Events []string
	// Search matches the username, IP, device and detail.
	Search string
	// Before lists entries older than this ID, for the next page.
	Before int64
	Limit  int
}

// ListLog returns log entries, newest first.
func (s *Store) ListLog(q LogQuery) ([]LogEntry, error) {
	events := ""
	args := []any{q.Before, q.Search}
	for i, e := range q.Events {
		if i > 0 {
			events += ", "
		}
		events += "?"
		args = append(args, e)
	}
	where := ""
	if events != "" {
		where = ` AND event IN (` + events + `)`
	}
	args = append(args, q.Limit)
	rows, err := s.db.Query(`
		SELECT id, at, event, user_id, username, ip, device_kind, device, detail FROM activity_log
		WHERE (?1 = 0 OR id < ?1)
		AND (?2 = '' OR instr(lower(username || ' ' || ip || ' ' || device || ' ' || detail), lower(?2)) > 0)`+where+`
		ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Event, &e.UserID, &e.Username, &e.IP, &e.DeviceKind, &e.Device, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReadingSession is someone reading a book right now.
type ReadingSession struct {
	User struct {
		ID              int64  `json:"id"`
		Username        string `json:"username"`
		HasAvatar       bool   `json:"hasAvatar"`
		AvatarUpdatedAt string `json:"avatarUpdatedAt,omitempty"`
	} `json:"user"`
	Book       Book   `json:"book"`
	IP         string `json:"ip"`
	DeviceKind string `json:"deviceKind"`
	Device     string `json:"device"`
	StartedAt  string `json:"startedAt"`
	LastSeen   string `json:"lastSeen"`
}

// TouchReading records that the user is reading a book on a client, now.
// Opening another book on the same client starts a new session.
func (s *Store) TouchReading(userID int64, client string, bookID int64, ip, deviceKind, device string) error {
	now := time.Now().UTC().Format(timeMillis)
	_, err := s.db.Exec(`
		INSERT INTO reading_sessions (user_id, client, book_id, ip, device_kind, device, started_at, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, client) DO UPDATE SET
			started_at = CASE WHEN book_id = excluded.book_id THEN started_at ELSE excluded.started_at END,
			book_id = excluded.book_id, ip = excluded.ip, last_seen = excluded.last_seen`,
		userID, client, bookID, ip, deviceKind, device, now, now)
	return err
}

// EndReading records that the user closed the reader on a client.
func (s *Store) EndReading(userID int64, client string) error {
	_, err := s.db.Exec(`DELETE FROM reading_sessions WHERE user_id = ? AND client = ?`, userID, client)
	return err
}

// Reading returns who read something since the given time, most recent
// first, with each book as that user sees it (their progress).
func (s *Store) Reading(since time.Time) ([]ReadingSession, error) {
	cutoff := since.UTC().Format(timeMillis)
	// Sessions nobody reported for a day are gone for good.
	if _, err := s.db.Exec(`DELETE FROM reading_sessions WHERE last_seen < ?`,
		since.UTC().Add(-24*time.Hour).Format(timeMillis)); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT r.user_id, u.username, u.avatar_updated_at, r.book_id, r.ip, r.device_kind, r.device, r.started_at, r.last_seen
		FROM reading_sessions r JOIN users u ON u.id = r.user_id
		WHERE r.last_seen >= ? ORDER BY r.last_seen DESC`, cutoff)
	if err != nil {
		return nil, err
	}
	var out []ReadingSession
	var bookIDs []int64
	for rows.Next() {
		var r ReadingSession
		var avatar *string
		var bookID int64
		if err := rows.Scan(&r.User.ID, &r.User.Username, &avatar, &bookID, &r.IP, &r.DeviceKind, &r.Device, &r.StartedAt, &r.LastSeen); err != nil {
			rows.Close()
			return nil, err
		}
		if avatar != nil {
			r.User.HasAvatar, r.User.AvatarUpdatedAt = true, *avatar
		}
		out = append(out, r)
		bookIDs = append(bookIDs, bookID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A book hidden from the user since isn't theirs to read any more.
	shown := []ReadingSession{}
	for i, r := range out {
		b, err := s.GetBook(r.User.ID, bookIDs[i])
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		r.Book = b
		shown = append(shown, r)
	}
	return shown, nil
}
