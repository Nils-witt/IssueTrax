// Package config loads IssueTrax runtime configuration from flags and
// environment variables (12-factor: env wins as the default for each flag).
// Secrets are read from the environment only, never from flags, and each
// secret FOO can alternatively be read from the file named by FOO_FILE
// (Docker/Kubernetes secrets).
package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config is the application configuration.
type Config struct {
	// Addr is the TCP address the HTTP server listens on, e.g. ":8080".
	Addr string
	// LogLevel is the minimum level for structured logs.
	LogLevel slog.Level
	// DatabaseURL is the PostgreSQL connection string (secret).
	DatabaseURL string
	// MigrateOnStart applies pending schema migrations at startup.
	MigrateOnStart bool
}

// Load parses args (typically os.Args[1:]) into a Config. Each flag defaults
// to its environment variable, so env configures deployments and flags
// override locally.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("issuetrax", flag.ContinueOnError)

	addr := fs.String("addr", envOrDefault("ISSUETRAX_ADDR", ":8080"), "HTTP listen address (env ISSUETRAX_ADDR)")
	logLevel := fs.String("log-level", envOrDefault("ISSUETRAX_LOG_LEVEL", "info"), "log level: debug, info, warn, error (env ISSUETRAX_LOG_LEVEL)")
	migrate := fs.String("migrate-on-start", envOrDefault("ISSUETRAX_MIGRATE_ON_START", "true"),
		"apply database migrations at startup (env ISSUETRAX_MIGRATE_ON_START)")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg := Config{Addr: *addr}

	var errs []error

	if err := cfg.LogLevel.UnmarshalText([]byte(*logLevel)); err != nil {
		errs = append(errs, fmt.Errorf("invalid log level %q: %w", *logLevel, err))
	}

	var err error

	if cfg.MigrateOnStart, err = strconv.ParseBool(*migrate); err != nil {
		errs = append(errs, fmt.Errorf("invalid migrate-on-start %q: %w", *migrate, err))
	}

	if cfg.DatabaseURL, err = secret("ISSUETRAX_DATABASE_URL"); err != nil {
		errs = append(errs, err)
	}

	if cfg.Addr == "" {
		errs = append(errs, errors.New("addr must not be empty"))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("ISSUETRAX_DATABASE_URL (or ISSUETRAX_DATABASE_URL_FILE) is required"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// secret returns the value of env key, or the trimmed contents of the file
// named by key_FILE. Setting both is an error.
func secret(key string) (string, error) {
	value, path := os.Getenv(key), os.Getenv(key+"_FILE")

	switch {
	case value != "" && path != "":
		return "", fmt.Errorf("set only one of %s and %s_FILE", key, key)
	case path == "":
		return value, nil
	}

	b, err := os.ReadFile(path) //nolint:gosec // the operator chooses which secret file to read
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", key, err)
	}

	return strings.TrimSpace(string(b)), nil
}
