package port

import (
	"context"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// LoginState is the per-login secret material that must survive the round
// trip to the identity provider. The inbound adapter stores it where the
// browser cannot tamper with it (e.g. a signed cookie).
type LoginState struct {
	// State is echoed back by the IdP and binds the callback to this login.
	State string
	// Nonce is embedded in the ID token and binds the token to this login.
	Nonce string
	// Verifier is the PKCE code verifier.
	Verifier string
	// ReturnTo is a sanitized local path to show after login.
	ReturnTo string
}

// AuthService is the driving port for signing in and out.
type AuthService interface {
	// BeginLogin returns the IdP URL to redirect the browser to and the
	// state to keep until the callback. returnTo is sanitized to a local path.
	BeginLogin(ctx context.Context, returnTo string) (redirectURL string, state LoginState, err error)
	// CompleteLogin checks the callback against saved, exchanges code for
	// an identity, provisions or refreshes the user and starts a session.
	// The returned token identifies the session (store it in a cookie).
	// Failures wrap domain.ErrLoginFailed.
	CompleteLogin(ctx context.Context, saved LoginState, returnedState, code string) (token string, user domain.User, err error)
	// Authenticate resolves a session token to its user or returns domain.ErrNoSession.
	Authenticate(ctx context.Context, token string) (domain.User, error)
	// Logout ends the session (if any) and returns the IdP logout URL to
	// visit next, or "" when the IdP does not support it.
	Logout(ctx context.Context, token string) (idpLogoutURL string, err error)
	// DeleteExpiredSessions removes stale sessions and reports how many.
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// IdentityProvider is the driven port for the external OpenID Connect provider.
type IdentityProvider interface {
	// AuthCodeURL builds the authorization request URL (code flow + PKCE S256).
	AuthCodeURL(state, nonce, verifier string) string
	// Exchange redeems code, verifies the ID token (signature, audience,
	// expiry, nonce) and returns the asserted identity plus the raw ID token.
	Exchange(ctx context.Context, code, verifier, nonce string) (identity domain.Identity, idToken string, err error)
	// EndSessionURL returns the RP-initiated logout URL, or "" if unsupported.
	EndSessionURL(idToken string) string
}

// SessionRepository is the driven port for persisting sessions.
//
// Get returns domain.ErrNoSession for unknown hashes. Delete ignores unknown
// hashes. DeleteExpired removes sessions with ExpiresAt at or before now.
type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	Get(ctx context.Context, tokenHash []byte) (domain.Session, error)
	Delete(ctx context.Context, tokenHash []byte) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
