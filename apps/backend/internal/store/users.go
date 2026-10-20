package store

import (
	"errors"
	"strings"
	"time"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

var ErrUsernameTaken = errors.New("username taken")

type User struct {
	ID                 int64     `json:"id"`
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	MustChangePassword bool      `json:"mustChangePassword"`
	CreatedAt          time.Time `json:"createdAt"`
	// HasAvatar says whether GET /api/users/{id}/avatar has a picture.
	HasAvatar bool `json:"hasAvatar"`
	// AvatarUpdatedAt is when the picture last changed, empty without one.
	// Clients add it to the picture's URL, so a new picture isn't hidden by
	// the browser's cached copy of the old one.
	AvatarUpdatedAt string `json:"avatarUpdatedAt,omitempty"`
	PasswordHash    string `json:"-"`
	TokenVersion    int    `json:"-"`
}

const userColumns = `id, username, password_hash, role, must_change_password, token_version, created_at,
	COALESCE(avatar_updated_at, '')`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.MustChangePassword, &u.TokenVersion, &created,
		&u.AvatarUpdatedAt); err != nil {
		return u, err
	}
	u.HasAvatar = u.AvatarUpdatedAt != ""
	var err error
	u.CreatedAt, err = time.Parse(time.RFC3339, created)
	return u, err
}

// CreateUser gives each user an ID never used before, see id_counters.
func (s *Store) CreateUser(username, passwordHash, role string, mustChangePassword bool) (User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRow(`
		INSERT INTO id_counters (name, last) VALUES ('users', 1)
		ON CONFLICT (name) DO UPDATE SET last = MAX(last, (SELECT COALESCE(MAX(id), 0) FROM users)) + 1
		RETURNING last`).Scan(&id); err != nil {
		return User{}, err
	}
	u, err := scanUser(tx.QueryRow(`
		INSERT INTO users (id, username, password_hash, role, must_change_password) VALUES (?, ?, ?, ?, ?)
		RETURNING `+userColumns, id, username, passwordHash, role, mustChangePassword))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	return u, tx.Commit()
}

func (s *Store) GetUser(id int64) (User, error) {
	u, err := scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	return u, notFound(err)
}

// GetUserByUsername ignores case, so "Anna" finds "anna".
func (s *Store) GetUserByUsername(username string) (User, error) {
	u, err := scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ?`, username))
	return u, notFound(err)
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// SetPassword stores a new password hash and raises the token version, which
// logs the user out everywhere. mustChange is true when an admin sets the
// password and false when users change their own.
func (s *Store) SetPassword(id int64, passwordHash string, mustChange bool) (User, error) {
	u, err := scanUser(s.db.QueryRow(`
		UPDATE users SET password_hash = ?, must_change_password = ?, token_version = token_version + 1
		WHERE id = ? RETURNING `+userColumns, passwordHash, mustChange, id))
	return u, notFound(err)
}

// UpdateUser renames a user or changes their role. A nil value stays as it
// is. Tokens stay valid: requests load the user fresh, so the change applies
// to the next request.
func (s *Store) UpdateUser(id int64, username, role *string) (User, error) {
	u, err := scanUser(s.db.QueryRow(`
		UPDATE users SET username = COALESCE(?, username), role = COALESCE(?, role)
		WHERE id = ? RETURNING `+userColumns, username, role, id))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return User{}, ErrUsernameTaken
	}
	return u, notFound(err)
}

// DeleteUser also deletes the user's reading progress.
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}
