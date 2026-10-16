// Package config reads the server settings from environment variables.
package config

import (
	"errors"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DataDir holds the database and other files the server writes.
	DataDir string
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

// Load reads NOVIL_ADDR and NOVIL_DATA_DIR, falling back to defaults that
// work for local development.
func Load() Config {
	return Config{
		Addr:    env("NOVIL_ADDR", ":8080"),
		DataDir: env("NOVIL_DATA_DIR", "data"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
