package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/config"
)

const dbURL = "postgres://localhost/db"

// Tests set environment variables, so they cannot run in parallel.

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ISSUETRAX_DATABASE_URL", dbURL)

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":8080" || cfg.LogLevel != slog.LevelInfo || cfg.DatabaseURL != dbURL || !cfg.MigrateOnStart {
		t.Fatalf("Load() = %+v", cfg)
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("ISSUETRAX_DATABASE_URL", dbURL)
	t.Setenv("ISSUETRAX_ADDR", ":1")

	cfg, err := config.Load([]string{"-addr", ":2", "-log-level", "debug", "-migrate-on-start=false"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":2" || cfg.LogLevel != slog.LevelDebug || cfg.MigrateOnStart {
		t.Fatalf("Load() = %+v", cfg)
	}
}

func TestLoad_SecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	if err := os.WriteFile(path, []byte(dbURL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ISSUETRAX_DATABASE_URL_FILE", path)

	cfg, err := config.Load(nil)
	if err != nil || cfg.DatabaseURL != dbURL {
		t.Fatalf("Load() = %+v, %v", cfg, err)
	}

	t.Setenv("ISSUETRAX_DATABASE_URL", dbURL)

	if _, err := config.Load(nil); err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("Load() with both = %v, want conflict error", err)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Setenv("ISSUETRAX_DATABASE_URL", "")

	_, err := config.Load([]string{"-log-level", "loud", "-migrate-on-start", "maybe"})
	if err == nil {
		t.Fatal("Load() = nil error")
	}

	for _, want := range []string{"log level", "migrate-on-start", "ISSUETRAX_DATABASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
