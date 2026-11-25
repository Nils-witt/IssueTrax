// Package config loads IssueTrax runtime configuration from flags and
// environment variables (12-factor: env wins as the default for each flag).
package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

// Config is the application configuration.
type Config struct {
	// Addr is the TCP address the HTTP server listens on, e.g. ":8080".
	Addr string
	// LogLevel is the minimum level for structured logs.
	LogLevel slog.Level
}

// Load parses args (typically os.Args[1:]) into a Config. Each flag defaults
// to its environment variable, so env configures deployments and flags
// override locally.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("issuetrax", flag.ContinueOnError)

	addr := fs.String("addr", envOrDefault("ISSUETRAX_ADDR", ":8080"), "HTTP listen address (env ISSUETRAX_ADDR)")
	logLevel := fs.String("log-level", envOrDefault("ISSUETRAX_LOG_LEVEL", "info"), "log level: debug, info, warn, error (env ISSUETRAX_LOG_LEVEL)")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg := Config{Addr: *addr}

	if err := cfg.LogLevel.UnmarshalText([]byte(*logLevel)); err != nil {
		return Config{}, fmt.Errorf("invalid log level %q: %w", *logLevel, err)
	}

	if cfg.Addr == "" {
		return Config{}, errors.New("addr must not be empty")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
