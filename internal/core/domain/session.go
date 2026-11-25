package domain

import "time"

// SessionLifetime is how long a login lasts. Sessions are never extended;
// after it elapses the user signs in again (usually silently via the IdP).
const SessionLifetime = 24 * time.Hour

// Session is a signed-in browser. Only a hash of the session token is
// stored, so a database leak does not expose usable tokens.
type Session struct {
	TokenHash []byte
	UserID    UserID
	// IDToken is the identity provider's ID token, kept for logging out
	// at the provider (id_token_hint).
	IDToken   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewSession starts a session for user that expires after SessionLifetime.
func NewSession(tokenHash []byte, user UserID, idToken string, now time.Time) Session {
	return Session{TokenHash: tokenHash, UserID: user, IDToken: idToken, CreatedAt: now, ExpiresAt: now.Add(SessionLifetime)}
}

// Expired reports whether s is no longer valid at now.
func (s Session) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }
