package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// authed wraps a handler that needs a caller: a signed-in user (session
// cookie) or a service account (Authorization: Bearer <api key>). Without
// valid credentials it answers 401. A request that sends an Authorization
// header is judged by it alone, never by a cookie it may also carry.
func (h *Handler) authed(next func(http.ResponseWriter, *http.Request, domain.User)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.authenticate(r)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidAPIKey) {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			}

			h.writeDomainError(w, r, err)

			return
		}

		next(w, r, user)
	})
}

// sessionOnly is like authed but also refuses service accounts with 403,
// for actions only people may take (managing service accounts and keys).
func (h *Handler) sessionOnly(next func(http.ResponseWriter, *http.Request, domain.User)) http.Handler {
	return h.authed(func(w http.ResponseWriter, r *http.Request, user domain.User) {
		if user.IsServiceAccount() {
			h.writeDomainError(w, r, domain.ErrSessionRequired)
			return
		}

		next(w, r, user)
	})
}

func (h *Handler) authenticate(r *http.Request) (domain.User, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return h.auth.Authenticate(r.Context(), h.cookies.sessionToken(r))
	}

	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return domain.User{}, domain.ErrInvalidAPIKey
	}

	return h.auth.AuthenticateAPIKey(r.Context(), strings.TrimSpace(token))
}

// login starts the OIDC flow: GET /auth/login?returnTo=/p/TRAX.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	redirect, state, err := h.auth.BeginLogin(r.Context(), r.URL.Query().Get("returnTo"))
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	h.cookies.setLoginState(w, state, time.Now())
	// security: redirect is the IdP authorization URL built by the auth
	// service from configuration; no request input chooses the target.
	http.Redirect(w, r, redirect, http.StatusFound) //nolint:gosec // see above
}

// callback finishes the OIDC flow: GET /auth/callback?code=...&state=...
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	saved, ok := h.cookies.loginState(r, time.Now())
	h.cookies.clearLoginState(w)

	q := r.URL.Query()

	switch {
	case !ok:
		h.loginFailed(w, r, errors.New("missing, expired or tampered login cookie"))
		return
	case q.Get("error") != "":
		h.loginFailed(w, r, fmt.Errorf("identity provider returned %q", q.Get("error")))
		return
	}

	token, user, err := h.auth.CompleteLogin(r.Context(), saved, q.Get("state"), q.Get("code"))
	if errors.Is(err, domain.ErrLoginFailed) {
		h.loginFailed(w, r, err)
		return
	}

	if err != nil {
		h.internalError(w, r, err)
		return
	}

	h.logger.InfoContext(r.Context(), "user signed in", slog.String("user_id", user.ID.String()))
	h.cookies.setSession(w, token)
	// ReturnTo is signed and was sanitized at login; re-check anyway.
	http.Redirect(w, r, service.SafeReturnTo(saved.ReturnTo), http.StatusFound)
}

const loginFailedPage = `<!doctype html><meta charset="utf-8"><title>Sign-in failed</title>
<p>Sign-in failed. <a href="/auth/login">Try again</a></p>
<p>Anmeldung fehlgeschlagen. <a href="/auth/login">Erneut versuchen</a></p>
`

// loginFailed logs why and shows a static page offering a retry.
func (h *Handler) loginFailed(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.WarnContext(r.Context(), "sign-in failed", slog.Any("error", err))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(loginFailedPage))
}

// logout ends the session: POST /auth/logout → {"redirectUrl": "..."}.
// The SPA navigates to redirectUrl to end the IdP session too.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	redirect, err := h.auth.Logout(r.Context(), h.cookies.sessionToken(r))
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	h.cookies.clearSession(w)

	if redirect == "" {
		redirect = "/"
	}

	writeJSON(w, http.StatusOK, map[string]string{"redirectUrl": redirect})
}

// meResponse is the caller's own user, plus what only they need to know.
type meResponse struct {
	userResponse
	// Admin is true for members of the identity provider admin group; the
	// SPA shows admin pages (audit log) only to them.
	Admin bool `json:"admin"`
}

// me returns the signed-in user: GET /api/me.
func (h *Handler) me(w http.ResponseWriter, _ *http.Request, user domain.User) {
	writeJSON(w, http.StatusOK, meResponse{userResponse: toUserResponse(user), Admin: user.Admin})
}
