package auth

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte(strings.Repeat("s", 32))

func TestTokenRoundTrip(t *testing.T) {
	tokens := NewTokens(testSecret, time.Hour)
	token, exp, err := tokens.Issue(42, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(exp); d < 59*time.Minute || d > time.Hour {
		t.Errorf("expires in %v, want about 1h", d)
	}
	c, err := tokens.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 42 || c.Version != 3 {
		t.Errorf("claims %+v, want user 42 version 3", c)
	}
}

func TestTokenRejected(t *testing.T) {
	tokens := NewTokens(testSecret, time.Hour)
	valid, _, _ := tokens.Issue(1, 0)
	expired, _, _ := NewTokens(testSecret, -time.Minute).Issue(1, 0)
	otherSecret, _, _ := NewTokens([]byte(strings.Repeat("x", 32)), time.Hour).Issue(1, 0)
	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "1", "iss": issuer, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	for name, token := range map[string]string{
		"expired":      expired,
		"other secret": otherSecret,
		"alg none":     unsigned,
		"tampered":     valid[:len(valid)-2] + "xx",
		"garbage":      "not-a-token",
	} {
		if _, err := tokens.Parse(token); err == nil {
			t.Errorf("%s: token accepted", name)
		}
	}
}

func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong horse") {
		t.Error("wrong password accepted")
	}
	if CheckPassword("", "correct horse") {
		t.Error("empty hash accepted")
	}

	if ValidatePassword("short") == nil {
		t.Error("short password accepted")
	}
	if ValidatePassword(strings.Repeat("a", 73)) == nil {
		t.Error("73 byte password accepted")
	}
	if err := ValidatePassword("long enough"); err != nil {
		t.Error(err)
	}
}

func TestValidateUsername(t *testing.T) {
	for _, name := range []string{"anna", "max.mustermann", "user_1", "a-b"} {
		if err := ValidateUsername(name); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
	for _, name := range []string{"", "ab", "with space", "ümlaut", strings.Repeat("a", 33)} {
		if ValidateUsername(name) == nil {
			t.Errorf("%q accepted", name)
		}
	}
}

func TestLoadSecret(t *testing.T) {
	if _, err := LoadSecret("too short", ""); err == nil {
		t.Error("short secret accepted")
	}

	path := filepath.Join(t.TempDir(), "jwt.key")
	first, err := LoadSecret("", path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadSecret("", path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("stored secret changed between loads")
	}
}
