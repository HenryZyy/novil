// Package config reads the server settings from environment variables.
package config

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DataDir holds the database and other files the server writes.
	DataDir string
	// JWTSecret signs the login tokens. If empty, a random secret is
	// created and stored in DataDir.
	JWTSecret string
	// AdminUser and AdminPassword create the first admin when there are no
	// users yet. They are ignored afterwards.
	AdminUser     string
	AdminPassword string
	// CORSOrigins are the browser origins allowed to call the API, e.g.
	// the Vite dev server. Empty means no cross-origin requests.
	CORSOrigins []string
	// TrustedProxies are the IPs or CIDR ranges of reverse proxies in front
	// of the server, e.g. "172.16.0.0/12". Their X-Forwarded-For header
	// tells the client's real IP for the login rate limit.
	TrustedProxies []string
}

// LoadEnvFile sets the variables from a .env file at path. Variables that
// are already set in the environment win, and a missing file is not an error.
func LoadEnvFile(path string) error {
	err := godotenv.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Load reads the NOVIL_* environment variables, falling back to defaults
// that work for local development.
func Load() Config {
	return Config{
		Addr:           env("NOVIL_ADDR", ":8080"),
		DataDir:        env("NOVIL_DATA_DIR", "data"),
		JWTSecret:      os.Getenv("NOVIL_JWT_SECRET"),
		AdminUser:      strings.TrimSpace(os.Getenv("NOVIL_ADMIN_USER")),
		AdminPassword:  os.Getenv("NOVIL_ADMIN_PASSWORD"),
		CORSOrigins:    list(os.Getenv("NOVIL_CORS_ORIGINS")),
		TrustedProxies: list(os.Getenv("NOVIL_TRUSTED_PROXIES")),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// list splits a comma-separated value, dropping empty entries.
func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
