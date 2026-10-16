// Package config reads server settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr           string
	DataDir        string
	WebDir         string
	MaxUploadBytes int64
}

func Load() (Config, error) {
	maxUploadMB, err := strconv.ParseInt(env("NOVIL_MAX_UPLOAD_MB", "2048"), 10, 64)
	if err != nil || maxUploadMB <= 0 {
		return Config{}, fmt.Errorf("NOVIL_MAX_UPLOAD_MB must be a positive number")
	}
	return Config{
		Addr:           env("NOVIL_ADDR", ":8080"),
		DataDir:        env("NOVIL_DATA_DIR", "./data"),
		WebDir:         env("NOVIL_WEB_DIR", ""),
		MaxUploadBytes: maxUploadMB << 20,
	}, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
