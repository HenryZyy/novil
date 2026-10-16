package config

import (
	"os"
	"path/filepath"
	"testing"

	"novil/backend/internal/testutil"
)

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	testutil.WriteFile(t, path, "# comment\nNOVIL_ADDR=:9090\nNOVIL_DATA_DIR=from-file\n")
	t.Setenv("NOVIL_ADDR", "")
	os.Unsetenv("NOVIL_ADDR")
	t.Setenv("NOVIL_DATA_DIR", "from-env")

	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090 from the file", cfg.Addr)
	}
	if cfg.DataDir != "from-env" {
		t.Errorf("DataDir = %q, want the already set from-env", cfg.DataDir)
	}
}

func TestLoadEnvFileMissing(t *testing.T) {
	if err := LoadEnvFile(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Errorf("missing file: %v, want nil", err)
	}
}

func TestLoadEnvFileInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	testutil.WriteFile(t, path, "NOVIL_ADDR='unterminated\n")
	if err := LoadEnvFile(path); err == nil {
		t.Error("invalid file: got nil error")
	}
}
