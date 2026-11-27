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
	"maps"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
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
	// PublicURL is the origin users open, e.g. https://trax.example.com.
	PublicURL string
	// OIDCIssuer is the identity provider's issuer URL (Authentik application).
	OIDCIssuer string
	// OIDCClientID and OIDCClientSecret identify IssueTrax at the IdP (secret).
	OIDCClientID     string
	OIDCClientSecret string
	// CookieKey signs the login-state cookie (secret, >= 32 bytes).
	CookieKey []byte
	// OIDCAdminGroup names the identity provider group whose members are
	// admins (may read the audit log). Empty means nobody is an admin.
	OIDCAdminGroup string
	// AuditRetention is how long audit entries are kept; 0 keeps them forever.
	AuditRetention time.Duration
	// AssetTagPrefix starts proposed asset tags of items whose type chain
	// sets no prefix.
	AssetTagPrefix string
	// Dev relaxes cookie security for http://localhost development.
	Dev bool
}

// maxRetentionDays bounds the audit retention (100 years) so the duration
// cannot overflow.
const maxRetentionDays = 36500

// minCookieKeyBytes matches the requirement of the HTTP adapter.
const minCookieKeyBytes = 32

// Load parses args (typically os.Args[1:]) into a Config. Each flag defaults
// to its environment variable, so env configures deployments and flags
// override locally.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("issuetrax", flag.ContinueOnError)

	addr := fs.String("addr", envOrDefault("ISSUETRAX_ADDR", ":8080"), "HTTP listen address (env ISSUETRAX_ADDR)")
	logLevel := fs.String("log-level", envOrDefault("ISSUETRAX_LOG_LEVEL", "info"), "log level: debug, info, warn, error (env ISSUETRAX_LOG_LEVEL)")
	migrate := fs.String("migrate-on-start", envOrDefault("ISSUETRAX_MIGRATE_ON_START", "true"),
		"apply database migrations at startup (env ISSUETRAX_MIGRATE_ON_START)")

	publicURL := fs.String("public-url", os.Getenv("ISSUETRAX_PUBLIC_URL"), "public origin users open (env ISSUETRAX_PUBLIC_URL)")
	issuer := fs.String("oidc-issuer", os.Getenv("ISSUETRAX_OIDC_ISSUER"), "OIDC issuer URL (env ISSUETRAX_OIDC_ISSUER)")
	clientID := fs.String("oidc-client-id", os.Getenv("ISSUETRAX_OIDC_CLIENT_ID"), "OIDC client ID (env ISSUETRAX_OIDC_CLIENT_ID)")
	adminGroup := fs.String("oidc-admin-group", os.Getenv("ISSUETRAX_OIDC_ADMIN_GROUP"),
		"identity provider group whose members are admins (env ISSUETRAX_OIDC_ADMIN_GROUP)")
	retention := fs.String("audit-retention-days", envOrDefault("ISSUETRAX_AUDIT_RETENTION_DAYS", "0"),
		"days to keep audit log entries, 0 keeps them forever (env ISSUETRAX_AUDIT_RETENTION_DAYS)")
	tagPrefix := fs.String("asset-tag-prefix", envOrDefault("ISSUETRAX_ASSET_TAG_PREFIX", "INV-"),
		"prefix of proposed asset tags when no item type sets one (env ISSUETRAX_ASSET_TAG_PREFIX)")
	dev := fs.String("dev", envOrDefault("ISSUETRAX_DEV", "false"), "development mode: cookies without Secure for http://localhost (env ISSUETRAX_DEV)")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg := Config{
		Addr:           *addr,
		PublicURL:      strings.TrimSuffix(*publicURL, "/"),
		OIDCIssuer:     *issuer,
		OIDCClientID:   *clientID,
		OIDCAdminGroup: strings.TrimSpace(*adminGroup),
		AssetTagPrefix: strings.TrimSpace(*tagPrefix),
	}

	var errs []error

	if err := cfg.LogLevel.UnmarshalText([]byte(*logLevel)); err != nil {
		errs = append(errs, fmt.Errorf("invalid log level %q: %w", *logLevel, err))
	}

	var err error

	if cfg.MigrateOnStart, err = strconv.ParseBool(*migrate); err != nil {
		errs = append(errs, fmt.Errorf("invalid migrate-on-start %q: %w", *migrate, err))
	}

	if cfg.AuditRetention, err = parseRetentionDays(*retention); err != nil {
		errs = append(errs, err)
	}

	if !domain.ValidTagPrefix(cfg.AssetTagPrefix) {
		errs = append(errs, fmt.Errorf("invalid asset-tag-prefix %q: at most 20 of A-Z a-z 0-9 . _ / -", cfg.AssetTagPrefix))
	}

	if cfg.Dev, err = strconv.ParseBool(*dev); err != nil {
		errs = append(errs, fmt.Errorf("invalid dev %q: %w", *dev, err))
	}

	if cfg.DatabaseURL, err = secret("ISSUETRAX_DATABASE_URL"); err != nil {
		errs = append(errs, err)
	}

	if cfg.OIDCClientSecret, err = secret("ISSUETRAX_OIDC_CLIENT_SECRET"); err != nil {
		errs = append(errs, err)
	}

	cookieKey, err := secret("ISSUETRAX_COOKIE_KEY")
	if err != nil {
		errs = append(errs, err)
	}

	cfg.CookieKey = []byte(cookieKey)

	errs = append(errs, cfg.validateAuth()...)

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

// validateAuth checks the sign-in related settings.
func (c Config) validateAuth() []error {
	var errs []error

	required := map[string]string{
		"ISSUETRAX_PUBLIC_URL":         c.PublicURL,
		"ISSUETRAX_OIDC_ISSUER":        c.OIDCIssuer,
		"ISSUETRAX_OIDC_CLIENT_ID":     c.OIDCClientID,
		"ISSUETRAX_OIDC_CLIENT_SECRET": c.OIDCClientSecret,
	}
	for _, name := range slices.Sorted(maps.Keys(required)) {
		if required[name] == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
	}

	if len(c.CookieKey) < minCookieKeyBytes {
		errs = append(errs, fmt.Errorf("ISSUETRAX_COOKIE_KEY must be at least %d bytes (e.g. `openssl rand -base64 32`)", minCookieKeyBytes))
	}

	if c.PublicURL != "" {
		if err := c.validatePublicURL(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// validatePublicURL requires an origin, https unless in dev mode, and dev
// mode only on localhost.
func (c Config) validatePublicURL() error {
	u, err := url.Parse(c.PublicURL)

	switch {
	case err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/"):
		return fmt.Errorf("ISSUETRAX_PUBLIC_URL %q must be an origin like https://trax.example.com", c.PublicURL)
	case c.Dev && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1":
		return errors.New("ISSUETRAX_DEV=true is only allowed with a http://localhost public URL")
	case !c.Dev && u.Scheme != "https":
		return errors.New("ISSUETRAX_PUBLIC_URL must use https (set ISSUETRAX_DEV=true for local http)")
	}

	return nil
}

// parseRetentionDays parses a whole number of days (0 keeps entries forever).
func parseRetentionDays(raw string) (time.Duration, error) {
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 || days > maxRetentionDays {
		return 0, fmt.Errorf("invalid audit-retention-days %q: want a whole number from 0 to %d", raw, maxRetentionDays)
	}

	return time.Duration(days) * 24 * time.Hour, nil
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
