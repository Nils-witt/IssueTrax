package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AuthService satisfies the driving port.
var _ port.AuthService = (*AuthService)(nil)

// AuthService implements port.AuthService on top of an OpenID Connect
// identity provider and server-side sessions.
type AuthService struct {
	repos Repositories
	idp   port.IdentityProvider
	opts  options
}

// NewAuthService returns an AuthService using idp for sign-in.
func NewAuthService(repos Repositories, idp port.IdentityProvider, opts ...Option) *AuthService {
	return &AuthService{repos: repos, idp: idp, opts: newOptions(opts)}
}

// BeginLogin creates fresh state, nonce and PKCE verifier for one login.
func (s *AuthService) BeginLogin(_ context.Context, returnTo string) (string, port.LoginState, error) {
	state := port.LoginState{
		State:    randomToken(),
		Nonce:    randomToken(),
		Verifier: randomToken(),
		ReturnTo: SafeReturnTo(returnTo),
	}

	return s.idp.AuthCodeURL(state.State, state.Nonce, state.Verifier), state, nil
}

// CompleteLogin verifies the callback, provisions the user and starts a session.
func (s *AuthService) CompleteLogin(
	ctx context.Context, saved port.LoginState, returnedState, code string,
) (string, domain.User, error) {
	if saved.State == "" || subtle.ConstantTimeCompare([]byte(saved.State), []byte(returnedState)) != 1 {
		return "", domain.User{}, fmt.Errorf("%w: state mismatch", domain.ErrLoginFailed)
	}

	// From here on the browser completed a login we started, so failures
	// are worth auditing; earlier ones may be anyone poking the callback.
	if code == "" {
		return "", domain.User{}, s.loginFailed(ctx, "missing_code", fmt.Errorf("%w: missing authorization code", domain.ErrLoginFailed))
	}

	ident, idToken, err := s.idp.Exchange(ctx, code, saved.Verifier, saved.Nonce)
	if err != nil {
		return "", domain.User{}, s.loginFailed(ctx, "exchange_failed", fmt.Errorf("%w: %w", domain.ErrLoginFailed, err))
	}

	token := randomToken()

	var user domain.User

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error

		if user, err = s.upsertUser(ctx, ident); err != nil {
			return err
		}

		session := domain.NewSession(hashToken(token), user.ID, idToken, s.opts.now())
		if err := s.repos.Sessions.Create(ctx, session); err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		changes := domain.AuditChanges{}
		if user.Admin {
			changes.Initial("admin", "true")
		}

		return record(ctx, s.repos, s.opts, userEvent(user, domain.AuditLogin, changes))
	})
	if errors.Is(err, domain.ErrLoginFailed) {
		return "", domain.User{}, s.loginFailed(ctx, "invalid_identity", err)
	}

	if err != nil {
		return "", domain.User{}, err
	}

	return token, user, nil
}

// loginFailed records a failed sign-in without an actor and returns
// cause, joined with the recording error if that fails too.
func (s *AuthService) loginFailed(ctx context.Context, reason string, cause error) error {
	changes := domain.AuditChanges{}
	changes.Initial("reason", reason)

	if err := record(ctx, s.repos, s.opts, auditEvent{action: domain.AuditLoginFailed, changes: changes}); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}

func userEvent(user domain.User, action domain.AuditAction, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &user.ID, action: action, changes: changes,
		targetType: domain.AuditTargetUser, targetID: user.ID.String(), targetLabel: user.DisplayName,
	}
}

// isAdmin reports whether ident is in the configured admin group.
func (s *AuthService) isAdmin(ident domain.Identity) bool {
	return s.opts.adminGroup != "" && slices.Contains(ident.Groups, s.opts.adminGroup)
}

// upsertUser creates the user on first login and refreshes the profile on
// later ones.
func (s *AuthService) upsertUser(ctx context.Context, ident domain.Identity) (domain.User, error) {
	now := s.opts.now()

	user, err := s.repos.Users.GetByIdentity(ctx, ident.Issuer, ident.Subject)

	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		if user, err = domain.NewUser(domain.UserID(s.opts.newID()), ident, now); err != nil {
			return domain.User{}, fmt.Errorf("%w: %w", domain.ErrLoginFailed, err)
		}

		user.Admin = s.isAdmin(ident)

		if err := s.repos.Users.Create(ctx, user); err != nil {
			return domain.User{}, fmt.Errorf("create user: %w", err)
		}
	case err != nil:
		return domain.User{}, fmt.Errorf("get user: %w", err)
	default:
		user.RecordLogin(ident, now)
		user.Admin = s.isAdmin(ident)

		if err := s.repos.Users.Update(ctx, user); err != nil {
			return domain.User{}, fmt.Errorf("update user: %w", err)
		}
	}

	return user, nil
}

// Authenticate resolves a session token to its user.
func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, domain.ErrNoSession
	}

	session, err := s.repos.Sessions.Get(ctx, hashToken(token))
	if err != nil {
		return domain.User{}, fmt.Errorf("get session: %w", err)
	}

	if session.Expired(s.opts.now()) {
		return domain.User{}, domain.ErrNoSession
	}

	user, err := s.repos.Users.Get(ctx, session.UserID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return domain.User{}, domain.ErrNoSession
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("get session user: %w", err)
	}

	return user, nil
}

// AuthenticateAPIKey resolves an API key token to its service account and
// records when the key was last used.
func (s *AuthService) AuthenticateAPIKey(ctx context.Context, token string) (domain.User, error) {
	if !strings.HasPrefix(token, domain.APIKeyTokenPrefix) {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	key, err := s.repos.APIKeys.GetByHash(ctx, hashToken(token))
	if errors.Is(err, domain.ErrAPIKeyNotFound) {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("get api key: %w", err)
	}

	now := s.opts.now()
	if !key.Active(now) {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	user, err := s.repos.Users.Get(ctx, key.UserID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("get api key user: %w", err)
	}

	// Only service accounts may use keys; people sign in via the IdP.
	if !user.IsServiceAccount() {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	if key.TouchLastUsed(now) {
		if err := s.repos.APIKeys.Update(ctx, key); err != nil {
			return domain.User{}, fmt.Errorf("record api key use: %w", err)
		}
	}

	return user, nil
}

// Logout deletes the session and returns where to end the IdP session.
func (s *AuthService) Logout(ctx context.Context, token string) (string, error) {
	var idToken string

	if token != "" {
		err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
			var err error

			idToken, err = s.endSession(ctx, hashToken(token))

			return err
		})
		if err != nil {
			return "", err
		}
	}

	return s.idp.EndSessionURL(idToken), nil
}

// endSession deletes the session with hash, records the logout if it
// existed and returns its ID token.
func (s *AuthService) endSession(ctx context.Context, hash []byte) (string, error) {
	session, err := s.repos.Sessions.Get(ctx, hash)

	switch {
	case errors.Is(err, domain.ErrNoSession):
		return "", s.repos.Sessions.Delete(ctx, hash)
	case err != nil:
		return "", fmt.Errorf("get session: %w", err)
	}

	if err := s.repos.Sessions.Delete(ctx, hash); err != nil {
		return "", fmt.Errorf("delete session: %w", err)
	}

	user, err := s.repos.Users.Get(ctx, session.UserID)

	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		user = domain.User{ID: session.UserID}
	case err != nil:
		return "", fmt.Errorf("get session user: %w", err)
	}

	if err := record(ctx, s.repos, s.opts, userEvent(user, domain.AuditLogout, nil)); err != nil {
		return "", err
	}

	return session.IDToken, nil
}

// DeleteExpiredSessions removes sessions past their lifetime.
func (s *AuthService) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	n, err := s.repos.Sessions.DeleteExpired(ctx, s.opts.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	return n, nil
}

// SafeReturnTo returns p if it is a local, absolute path (no scheme, host
// or protocol-relative tricks such as "//evil" or "/\evil"), otherwise "/".
// This prevents open redirects after login.
func SafeReturnTo(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\\\r\n\t") {
		return "/"
	}

	u, err := url.Parse(p)
	if err != nil || u.IsAbs() || u.Host != "" {
		return "/"
	}

	return p
}

// randomToken returns 256 random bits, base64url-encoded (43 chars, also a
// valid PKCE verifier).
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error (Go 1.24+)

	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken returns the storage key for a session or API key token. A plain SHA-256 is
// sufficient because tokens carry 256 bits of entropy.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}
