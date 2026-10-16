package store

import "time"

// Avatar returns a user's profile picture, or ErrNotFound.
func (s *Store) Avatar(userID int64) ([]byte, error) {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM user_avatars WHERE user_id = ?`, userID).Scan(&data)
	return data, notFound(err)
}

// SetAvatar stores a user's profile picture and returns the updated user.
func (s *Store) SetAvatar(userID int64, data []byte) (User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	u, err := scanUser(tx.QueryRow(`UPDATE users SET avatar_updated_at = ? WHERE id = ? RETURNING `+userColumns,
		time.Now().UTC().Format(timeMillis), userID))
	if err != nil {
		return User{}, notFound(err)
	}
	if _, err := tx.Exec(`
		INSERT INTO user_avatars (user_id, data) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET data = excluded.data`, userID, data); err != nil {
		return User{}, err
	}
	return u, tx.Commit()
}

// DeleteAvatar removes a user's profile picture, if they have one, and
// returns the updated user.
func (s *Store) DeleteAvatar(userID int64) (User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	u, err := scanUser(tx.QueryRow(`UPDATE users SET avatar_updated_at = NULL WHERE id = ? RETURNING `+userColumns, userID))
	if err != nil {
		return User{}, notFound(err)
	}
	if _, err := tx.Exec(`DELETE FROM user_avatars WHERE user_id = ?`, userID); err != nil {
		return User{}, err
	}
	return u, tx.Commit()
}
