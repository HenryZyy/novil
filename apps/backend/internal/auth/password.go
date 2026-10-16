// Package auth hashes passwords and issues and checks login tokens (JWTs).
// It knows nothing about HTTP or the database.
package auth

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLength = 8
	// bcrypt only looks at the first 72 bytes, so longer passwords are
	// rejected instead of silently cut.
	maxPasswordBytes = 72
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)

func ValidateUsername(name string) error {
	if !usernamePattern.MatchString(name) {
		return errors.New("username must be 3-32 characters: letters, digits, '.', '_' or '-'")
	}
	return nil
}

func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(pw) > maxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	}
	return nil
}

func HashPassword(pw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(hash), err
}

// dummyHash is checked when a user doesn't exist, so that a login with an
// unknown username takes as long as one with a wrong password. Otherwise the
// response time would reveal which usernames exist.
var dummyHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("novil-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
})

// CheckPassword reports whether pw matches hash. An empty hash (no such
// user) never matches but takes as long as a real check.
func CheckPassword(hash, pw string) bool {
	if hash == "" {
		bcrypt.CompareHashAndPassword(dummyHash(), []byte(pw))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
