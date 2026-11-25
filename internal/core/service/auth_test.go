package service_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// fakeIDP accepts code "good" and asserts ident for it.
type fakeIDP struct {
	ident domain.Identity
}

func (f *fakeIDP) AuthCodeURL(state, nonce, verifier string) string {
	return "https://idp.example/authorize?" + url.Values{"state": {state}, "nonce": {nonce}, "verifier": {verifier}}.Encode()
}

func (f *fakeIDP) Exchange(_ context.Context, code, verifier, nonce string) (domain.Identity, string, error) {
	if code != "good" || verifier == "" || nonce == "" {
		return domain.Identity{}, "", errors.New("invalid grant")
	}

	return f.ident, "id-token-for-" + f.ident.Subject, nil
}

func (f *fakeIDP) EndSessionURL(idToken string) string {
	return "https://idp.example/logout?id_token_hint=" + idToken
}

type authFixture struct {
	*fixture

	idp   *fakeIDP
	auth  *service.AuthService
	clock *time.Time
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	f := newFixture(t)
	now := testNow
	idp := &fakeIDP{ident: domain.Identity{Issuer: "https://idp.example", Subject: "ada", Email: "ada@example.com", DisplayName: "Ada"}}

	return &authFixture{
		fixture: f,
		idp:     idp,
		clock:   &now,
		auth:    service.NewAuthService(f.repos, idp, service.WithClock(func() time.Time { return now })),
	}
}

func (f *authFixture) login(t *testing.T) (string, domain.User) {
	t.Helper()

	_, state, err := f.auth.BeginLogin(t.Context(), "/p/TRAX")
	if err != nil {
		t.Fatal(err)
	}

	token, user, err := f.auth.CompleteLogin(t.Context(), state, state.State, "good")
	if err != nil {
		t.Fatalf("CompleteLogin() = %v", err)
	}

	return token, user
}

func TestAuthService_BeginLogin(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)

	redirect, state, err := f.auth.BeginLogin(t.Context(), "https://evil.example/")
	if err != nil {
		t.Fatal(err)
	}

	if len(state.State) < 43 || len(state.Nonce) < 43 || len(state.Verifier) < 43 || state.ReturnTo != "/" {
		t.Fatalf("state = %+v", state)
	}

	_, other, err := f.auth.BeginLogin(t.Context(), "/")
	if err != nil || other.State == state.State || other.Nonce == state.Nonce || other.Verifier == state.Verifier {
		t.Fatalf("logins share secrets: %+v vs %+v (%v)", state, other, err)
	}

	u, err := url.Parse(redirect)
	if err != nil || u.Query().Get("state") != state.State || u.Query().Get("nonce") != state.Nonce {
		t.Fatalf("redirect = %s", redirect)
	}
}

func TestAuthService_CompleteLogin(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)

	token, user, err := func() (string, domain.User, error) {
		_, state, _ := f.auth.BeginLogin(t.Context(), "/")
		return f.auth.CompleteLogin(t.Context(), state, state.State, "good")
	}()
	if err != nil {
		t.Fatal(err)
	}

	if token == "" || user.DisplayName != "Ada" || user.Subject != "ada" {
		t.Fatalf("CompleteLogin() = %q, %+v", token, user)
	}

	// A later login refreshes the profile and reuses the user.
	f.idp.ident.DisplayName = "Ada Lovelace"

	_, again := f.login(t)
	if again.ID != user.ID || again.DisplayName != "Ada Lovelace" {
		t.Fatalf("second login user = %+v, want same ID %v with new name", again, user.ID)
	}

	users, err := f.users.List(t.Context())
	if err != nil || len(users) != 1 {
		t.Fatalf("users = %+v, %v; want exactly one", users, err)
	}
}

func TestAuthService_CompleteLoginFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(saved *port.LoginState, returned, code *string)
		identity domain.Identity
	}{
		{name: "state mismatch", mutate: func(_ *port.LoginState, returned, _ *string) { *returned = "forged" }},
		{name: "empty saved state", mutate: func(saved *port.LoginState, returned, _ *string) { saved.State, *returned = "", "" }},
		{name: "missing code", mutate: func(_ *port.LoginState, _, code *string) { *code = "" }},
		{name: "idp rejects code", mutate: func(_ *port.LoginState, _, code *string) { *code = "bad" }},
		{name: "identity without subject", identity: domain.Identity{Issuer: "https://idp.example"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newAuthFixture(t)
			if tt.identity != (domain.Identity{}) {
				f.idp.ident = tt.identity
			}

			_, saved, err := f.auth.BeginLogin(t.Context(), "/")
			if err != nil {
				t.Fatal(err)
			}

			returned, code := saved.State, "good"
			if tt.mutate != nil {
				tt.mutate(&saved, &returned, &code)
			}

			_, _, err = f.auth.CompleteLogin(t.Context(), saved, returned, code)
			if !errors.Is(err, domain.ErrLoginFailed) {
				t.Fatalf("CompleteLogin() = %v, want ErrLoginFailed", err)
			}
		})
	}
}

func TestAuthService_Authenticate(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	token, user := f.login(t)

	got, err := f.auth.Authenticate(t.Context(), token)
	if err != nil || got.ID != user.ID {
		t.Fatalf("Authenticate() = %+v, %v", got, err)
	}

	for _, bad := range []string{"", "not-a-session", token + "x"} {
		if _, err := f.auth.Authenticate(t.Context(), bad); !errors.Is(err, domain.ErrNoSession) {
			t.Errorf("Authenticate(%q) = %v, want ErrNoSession", bad, err)
		}
	}

	*f.clock = testNow.Add(domain.SessionLifetime)

	if _, err := f.auth.Authenticate(t.Context(), token); !errors.Is(err, domain.ErrNoSession) {
		t.Fatalf("Authenticate() after lifetime = %v, want ErrNoSession", err)
	}

	n, err := f.auth.DeleteExpiredSessions(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpiredSessions() = %d, %v; want 1", n, err)
	}
}

func TestAuthService_Logout(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	token, _ := f.login(t)

	logoutURL, err := f.auth.Logout(t.Context(), token)
	if err != nil || logoutURL != "https://idp.example/logout?id_token_hint=id-token-for-ada" {
		t.Fatalf("Logout() = %q, %v", logoutURL, err)
	}

	if _, err := f.auth.Authenticate(t.Context(), token); !errors.Is(err, domain.ErrNoSession) {
		t.Fatalf("Authenticate() after logout = %v", err)
	}

	// Logging out without (or with a stale) session still ends the IdP session.
	if logoutURL, err := f.auth.Logout(t.Context(), token); err != nil || logoutURL != "https://idp.example/logout?id_token_hint=" {
		t.Fatalf("second Logout() = %q, %v", logoutURL, err)
	}
}

func TestSafeReturnTo(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"/p/TRAX/issues/1?x=1#c": "/p/TRAX/issues/1?x=1#c",
		"/":                      "/",
		"":                       "/",
		"p/TRAX":                 "/",
		"//evil.example":         "/",
		"/\\evil.example":        "/",
		"https://evil.example/":  "/",
		"/ok\r\nSet-Cookie: x":   "/",
		"javascript:alert(1)":    "/",
	}

	for in, want := range tests {
		if got := service.SafeReturnTo(in); got != want {
			t.Errorf("SafeReturnTo(%q) = %q, want %q", in, got, want)
		}
	}
}
