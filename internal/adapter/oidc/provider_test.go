package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"nilswitt.dev/issuetrax/internal/adapter/oidc"
)

const clientID = "issuetrax"

// fakeIDP is a minimal OpenID provider: discovery, JWKS, token endpoint
// with PKCE verification, and RS256-signed ID tokens.
type fakeIDP struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	mu     sync.Mutex
	grants map[string]grant // code -> grant
	// tamper lets a test corrupt the issued ID token claims.
	tamper func(claims map[string]any)
}

type grant struct {
	challenge string
	nonce     string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	f := &fakeIDP{key: key, grants: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /jwks", f.jwks)
	mux.HandleFunc("POST /token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	return f
}

func (f *fakeIDP) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                f.srv.URL,
		"authorization_endpoint":                f.srv.URL + "/authorize",
		"token_endpoint":                        f.srv.URL + "/token",
		"jwks_uri":                              f.srv.URL + "/jwks",
		"end_session_endpoint":                  f.srv.URL + "/end-session/",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeIDP) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

// authorize simulates the user approving the authorization request at u.
func (f *fakeIDP) authorize(t *testing.T, authURL string) string {
	t.Helper()

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}

	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != clientID {
		t.Fatalf("authorization request = %s", authURL)
	}

	code := "code-" + q.Get("state")

	f.mu.Lock()
	f.grants[code] = grant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce")}
	f.mu.Unlock()

	return code
}

func (f *fakeIDP) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	g, ok := f.grants[r.PostForm.Get("code")]
	f.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))

		return
	}

	claims := map[string]any{
		"iss": f.srv.URL, "sub": "user-1", "aud": clientID, "nonce": g.nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"email": "ada@example.com", "preferred_username": "ada", "groups": []string{"staff", "issuetrax-admins"},
	}
	if f.tamper != nil {
		f.tamper(claims)
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idToken, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newProvider(t *testing.T, f *fakeIDP) *oidc.Provider {
	t.Helper()

	p, err := oidc.New(t.Context(), oidc.Config{
		Issuer:                f.srv.URL,
		ClientID:              clientID,
		ClientSecret:          "secret",
		RedirectURL:           "https://trax.example/auth/callback",
		PostLogoutRedirectURL: "https://trax.example/",
	})
	if err != nil {
		t.Fatal(err)
	}

	return p
}

const (
	verifier = "verifier-verifier-verifier-verifier-verifier"
	nonce    = "nonce-1"
)

func TestProvider_Exchange(t *testing.T) {
	t.Parallel()

	f := newFakeIDP(t)
	p := newProvider(t, f)

	code := f.authorize(t, p.AuthCodeURL("state-1", nonce, verifier))

	ident, idToken, err := p.Exchange(t.Context(), code, verifier, nonce)
	if err != nil {
		t.Fatal(err)
	}

	if ident.Issuer != f.srv.URL || ident.Subject != "user-1" || ident.Email != "ada@example.com" || ident.DisplayName != "ada" {
		t.Fatalf("Exchange() identity = %+v", ident)
	}

	if !slices.Equal(ident.Groups, []string{"staff", "issuetrax-admins"}) {
		t.Fatalf("Exchange() groups = %v", ident.Groups)
	}

	if strings.Count(idToken, ".") != 2 {
		t.Fatalf("Exchange() id token = %q, want a JWT", idToken)
	}
}

func TestProvider_ExchangeFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		verifier string
		nonce    string
		tamper   func(map[string]any)
	}{
		{name: "wrong pkce verifier", verifier: verifier + "x", nonce: nonce},
		{name: "wrong nonce", verifier: verifier, nonce: "other"},
		{name: "wrong audience", verifier: verifier, nonce: nonce, tamper: func(c map[string]any) { c["aud"] = "someone-else" }},
		{name: "expired token", verifier: verifier, nonce: nonce, tamper: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "wrong issuer", verifier: verifier, nonce: nonce, tamper: func(c map[string]any) { c["iss"] = "https://evil.example" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFakeIDP(t)
			f.tamper = tt.tamper
			p := newProvider(t, f)

			code := f.authorize(t, p.AuthCodeURL("state-1", nonce, verifier))

			if _, _, err := p.Exchange(t.Context(), code, tt.verifier, tt.nonce); err == nil {
				t.Fatal("Exchange() = nil error")
			}
		})
	}
}

func TestProvider_EndSessionURL(t *testing.T) {
	t.Parallel()

	f := newFakeIDP(t)
	p := newProvider(t, f)

	u, err := url.Parse(p.EndSessionURL("the-id-token"))
	if err != nil {
		t.Fatal(err)
	}

	q := u.Query()
	if u.Path != "/end-session/" || q.Get("id_token_hint") != "the-id-token" ||
		q.Get("post_logout_redirect_uri") != "https://trax.example/" || q.Get("client_id") != clientID {
		t.Fatalf("EndSessionURL() = %s", u)
	}

	if strings.Contains(p.EndSessionURL(""), "id_token_hint") {
		t.Fatal("EndSessionURL(\"\") includes an empty id_token_hint")
	}
}

func TestNew_DiscoveryFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	if _, err := oidc.New(t.Context(), oidc.Config{Issuer: srv.URL, ClientID: clientID}); err == nil {
		t.Fatal("New() = nil error for an issuer without discovery")
	}
}
