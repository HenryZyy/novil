package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	issuer          = "novil"
	minSecretLength = 32
)

// Claims is what a token says about its user.
type Claims struct {
	UserID int64
	// Version must match the user's token_version in the database. It is
	// raised on password changes, which invalidates all older tokens.
	Version int
}

// jwtClaims is the JSON inside the token: the standard fields (sub, exp,
// iat, iss) plus our version.
type jwtClaims struct {
	Version int `json:"ver"`
	jwt.RegisteredClaims
}

// Tokens issues and checks JWTs signed with HMAC-SHA256.
type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret []byte, ttl time.Duration) *Tokens {
	return &Tokens{secret: secret, ttl: ttl}
}

// Issue returns a signed token for the user and when it expires.
func (t *Tokens) Issue(userID int64, version int) (string, time.Time, error) {
	now := time.Now().Truncate(time.Second)
	exp := now.Add(t.ttl)
	c := jwtClaims{
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	return token, exp, err
}

// Parse checks the token's signature, issuer and expiry and returns its
// claims. It doesn't check the version; that needs the database.
func (t *Tokens) Parse(token string) (Claims, error) {
	var c jwtClaims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		// Only accept our own algorithm, never "none" or a different one
		// an attacker picked.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, err
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid subject %q", c.Subject)
	}
	return Claims{UserID: id, Version: c.Version}, nil
}

// LoadSecret returns secret if it is set. Otherwise it reads the secret
// stored at path, creating a random one there on first use, so tokens stay
// valid across restarts.
func LoadSecret(secret, path string) ([]byte, error) {
	if secret != "" {
		if len(secret) < minSecretLength {
			return nil, fmt.Errorf("secret must be at least %d characters", minSecretLength)
		}
		return []byte(secret), nil
	}
	data, err := os.ReadFile(path)
	if err == nil {
		key, err := hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil || len(key) < minSecretLength {
			return nil, fmt.Errorf("%s does not hold a valid secret; delete it to create a new one", path)
		}
		return key, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, minSecretLength)
	rand.Read(key)
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
