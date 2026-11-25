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

// setValidEnv sets the minimal valid production configuration.
func setValidEnv(t *testing.T) {
	t.Helper()

	t.Setenv("ISSUETRAX_DATABASE_URL", dbURL)
	t.Setenv("ISSUETRAX_PUBLIC_URL", "https://trax.example/")
	t.Setenv("ISSUETRAX_OIDC_ISSUER", "https://auth.example/application/o/issuetrax/")
	t.Setenv("ISSUETRAX_OIDC_CLIENT_ID", "issuetrax")
	t.Setenv("ISSUETRAX_OIDC_CLIENT_SECRET", "secret")
	t.Setenv("ISSUETRAX_COOKIE_KEY", strings.Repeat("k", 32))
}

func TestLoad_Defaults(t *testing.T) { //nolint:paralleltest // setValidEnv uses t.Setenv
	setValidEnv(t)

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":8080" || cfg.LogLevel != slog.LevelInfo || cfg.DatabaseURL != dbURL || !cfg.MigrateOnStart || cfg.Dev {
		t.Fatalf("Load() = %+v", cfg)
	}

	if cfg.PublicURL != "https://trax.example" || cfg.OIDCClientSecret != "secret" || len(cfg.CookieKey) != 32 {
		t.Fatalf("Load() auth settings = %+v", cfg)
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	setValidEnv(t)
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
	setValidEnv(t)
	t.Setenv("ISSUETRAX_DATABASE_URL", "")

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
	t.Setenv("ISSUETRAX_PUBLIC_URL", "")
	t.Setenv("ISSUETRAX_COOKIE_KEY", "short")

	_, err := config.Load([]string{"-log-level", "loud", "-migrate-on-start", "maybe"})
	if err == nil {
		t.Fatal("Load() = nil error")
	}

	for _, want := range []string{
		"log level", "migrate-on-start", "ISSUETRAX_DATABASE_URL", "ISSUETRAX_PUBLIC_URL is required",
		"ISSUETRAX_OIDC_ISSUER is required", "ISSUETRAX_OIDC_CLIENT_SECRET is required", "ISSUETRAX_COOKIE_KEY",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoad_PublicURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		dev     string
		wantErr string
	}{
		{name: "https", url: "https://trax.example"},
		{name: "dev localhost", url: "http://localhost:5173", dev: "true"},
		{name: "http without dev", url: "http://trax.example", wantErr: "must use https"},
		{name: "dev on a real host", url: "http://trax.example", dev: "true", wantErr: "only allowed"},
		{name: "with path", url: "https://trax.example/app", wantErr: "must be an origin"},
		{name: "no scheme", url: "trax.example", wantErr: "must be an origin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv("ISSUETRAX_PUBLIC_URL", tt.url)
			t.Setenv("ISSUETRAX_DEV", tt.dev)

			_, err := config.Load(nil)

			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Load() = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("Load() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
