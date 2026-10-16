package store

import "time"

// RevokeToken blocks the token with the given ID until it expires. It also
// removes revoked tokens that have expired by now, since those are rejected
// anyway.
func (s *Store) RevokeToken(id string, expiresAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`DELETE FROM revoked_tokens WHERE expires_at < ?`, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO revoked_tokens (id, expires_at) VALUES (?, ?) ON CONFLICT (id) DO NOTHING`,
		id, expiresAt.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) IsTokenRevoked(id string) (bool, error) {
	var revoked bool
	err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM revoked_tokens WHERE id = ?)`, id).Scan(&revoked)
	return revoked, err
}
