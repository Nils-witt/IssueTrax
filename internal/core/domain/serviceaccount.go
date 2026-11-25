package domain

import (
	"regexp"
	"strings"
	"time"
)

const (
	// ServiceAccountIssuer is the synthetic identity issuer of service
	// accounts. Real OpenID Connect issuers are https URLs, so a login can
	// never produce a user with this issuer.
	ServiceAccountIssuer = "urn:issuetrax:service-account"
	// ServiceAccountPrefix starts the display name of every service account
	// ("bot-ci").
	ServiceAccountPrefix = "bot-"
)

var serviceAccountNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// NormalizeServiceAccountName trims and lower-cases s and drops a leading
// "bot-", so "ci" and "bot-CI" name the same account. It reports false when
// the result is not a valid name.
func NormalizeServiceAccountName(s string) (string, bool) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.TrimPrefix(n, ServiceAccountPrefix)

	return n, serviceAccountNamePattern.MatchString(n)
}

// NewServiceAccount builds the user behind a service account called name
// (shown as "bot-<name>"). Service accounts sign in with API keys only.
func NewServiceAccount(id UserID, name string, now time.Time) (User, error) {
	n, ok := NormalizeServiceAccountName(name)

	switch {
	case n == "":
		return User{}, NewValidationError(map[string]string{"name": FieldRequired})
	case !ok:
		return User{}, NewValidationError(map[string]string{"name": FieldInvalid})
	}

	return NewUser(id, Identity{Issuer: ServiceAccountIssuer, Subject: n, DisplayName: ServiceAccountPrefix + n}, now)
}

// IsServiceAccount reports whether u is a service account rather than a person.
func (u User) IsServiceAccount() bool { return u.Issuer == ServiceAccountIssuer }

// ServiceAccountName returns the name of a service account without the
// "bot-" prefix ("ci" for "bot-ci").
func (u User) ServiceAccountName() string { return u.Subject }
