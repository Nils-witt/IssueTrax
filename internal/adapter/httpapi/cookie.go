package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// loginStateLifetime bounds how long a user may take at the IdP.
const loginStateLifetime = 10 * time.Minute

// cookies reads and writes IssueTrax's two cookies:
//
//   - the session cookie: an opaque random token, HttpOnly, 24 h;
//   - the login cookie: the HMAC-signed port.LoginState, HttpOnly, 10 min,
//     only alive between /auth/login and /auth/callback.
//
// Both are SameSite=Lax: the IdP's redirect back to /auth/callback is a
// top-level navigation, which Lax still allows, while cross-site
// subrequests never carry them. With secure set they also use the
// __Host- prefix (Secure, Path=/, no Domain), so subdomains cannot set them.
type cookies struct {
	secure bool
	key    []byte
}

func (c cookies) name(base string) string {
	if c.secure {
		return "__Host-" + base
	}

	return base
}

func (c cookies) set(w http.ResponseWriter, base, value string, maxAge time.Duration) {
	// security: Secure is only off in explicit dev mode (http://localhost).
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // see above
		Name:     c.name(base),
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c cookies) clear(w http.ResponseWriter, base string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is only off in explicit dev mode
		Name: c.name(base), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (c cookies) get(r *http.Request, base string) string {
	ck, err := r.Cookie(c.name(base))
	if err != nil {
		return ""
	}

	return ck.Value
}

const (
	sessionCookie = "issuetrax_session"
	loginCookie   = "issuetrax_login"
)

func (c cookies) setSession(w http.ResponseWriter, token string) {
	c.set(w, sessionCookie, token, domain.SessionLifetime)
}

func (c cookies) clearSession(w http.ResponseWriter) { c.clear(w, sessionCookie) }

func (c cookies) sessionToken(r *http.Request) string { return c.get(r, sessionCookie) }

// signedLoginState is the login cookie payload.
type signedLoginState struct {
	State     string `json:"s"`
	Nonce     string `json:"n"`
	Verifier  string `json:"v"`
	ReturnTo  string `json:"r"`
	ExpiresAt int64  `json:"e"`
}

func (c cookies) setLoginState(w http.ResponseWriter, s port.LoginState, now time.Time) {
	payload, _ := json.Marshal(signedLoginState{ //nolint:errchkjson // plain struct of strings and an int
		State: s.State, Nonce: s.Nonce, Verifier: s.Verifier, ReturnTo: s.ReturnTo,
		ExpiresAt: now.Add(loginStateLifetime).Unix(),
	})
	enc := base64.RawURLEncoding.EncodeToString(payload)

	c.set(w, loginCookie, enc+"."+c.mac(enc), loginStateLifetime)
}

// loginState returns the verified, unexpired login state from r.
func (c cookies) loginState(r *http.Request, now time.Time) (port.LoginState, bool) {
	enc, sig, ok := strings.Cut(c.get(r, loginCookie), ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(c.mac(enc))) {
		return port.LoginState{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return port.LoginState{}, false
	}

	var s signedLoginState
	if err := json.Unmarshal(payload, &s); err != nil || now.Unix() > s.ExpiresAt {
		return port.LoginState{}, false
	}

	return port.LoginState{State: s.State, Nonce: s.Nonce, Verifier: s.Verifier, ReturnTo: s.ReturnTo}, true
}

func (c cookies) clearLoginState(w http.ResponseWriter) { c.clear(w, loginCookie) }

func (c cookies) mac(data string) string {
	m := hmac.New(sha256.New, c.key)
	m.Write([]byte(data))

	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
