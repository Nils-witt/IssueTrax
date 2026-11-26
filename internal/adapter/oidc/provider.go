// Package oidc is the outbound adapter for the external OpenID Connect
// identity provider (Authentik). It runs the authorization code flow with
// PKCE and verifies ID tokens; the core only sees domain.Identity.
package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that Provider satisfies the driven port.
var _ port.IdentityProvider = (*Provider)(nil)

// Config configures the OIDC client.
type Config struct {
	// Issuer is the provider's issuer URL, used for discovery.
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL is this app's callback, e.g. https://trax.example.com/auth/callback.
	RedirectURL string
	// PostLogoutRedirectURL is where the IdP sends the browser after logout.
	PostLogoutRedirectURL string
}

// Provider implements port.IdentityProvider.
type Provider struct {
	oauth      oauth2.Config
	verifier   *gooidc.IDTokenVerifier
	endSession string
	cfg        Config
}

// New discovers the provider's endpoints and signing keys.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	p, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider %s: %w", cfg.Issuer, err)
	}

	var meta struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}

	if err := p.Claims(&meta); err != nil {
		return nil, fmt.Errorf("read oidc discovery document: %w", err)
	}

	return &Provider{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     p.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		},
		verifier:   p.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		endSession: meta.EndSessionEndpoint,
		cfg:        cfg,
	}, nil
}

// AuthCodeURL builds the authorization request with nonce and PKCE (S256).
func (p *Provider) AuthCodeURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange redeems code and verifies the returned ID token.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (domain.Identity, string, error) {
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return domain.Identity{}, "", fmt.Errorf("exchange authorization code: %w", err)
	}

	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return domain.Identity{}, "", errors.New("token response has no id_token")
	}

	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return domain.Identity{}, "", fmt.Errorf("verify id token: %w", err)
	}

	if idToken.Nonce != nonce {
		return domain.Identity{}, "", errors.New("id token nonce mismatch")
	}

	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		// Groups is sent by Authentik with the profile scope.
		Groups []string `json:"groups"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return domain.Identity{}, "", fmt.Errorf("decode id token claims: %w", err)
	}

	name := strings.TrimSpace(claims.Name)
	if name == "" {
		name = claims.PreferredUsername
	}

	return domain.Identity{
		Issuer:      idToken.Issuer,
		Subject:     idToken.Subject,
		Email:       claims.Email,
		DisplayName: name,
		Groups:      claims.Groups,
	}, raw, nil
}

// EndSessionURL builds the RP-initiated logout URL, or "" if the provider
// does not advertise an end_session_endpoint.
func (p *Provider) EndSessionURL(idToken string) string {
	if p.endSession == "" {
		return ""
	}

	u, err := url.Parse(p.endSession)
	if err != nil {
		return ""
	}

	q := u.Query()
	q.Set("client_id", p.cfg.ClientID)

	if idToken != "" {
		q.Set("id_token_hint", idToken)
	}

	if p.cfg.PostLogoutRedirectURL != "" {
		q.Set("post_logout_redirect_uri", p.cfg.PostLogoutRedirectURL)
	}

	u.RawQuery = q.Encode()

	return u.String()
}
