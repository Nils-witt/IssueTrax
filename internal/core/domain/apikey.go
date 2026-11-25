package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	// APIKeyTokenPrefix starts every API key token, so keys are easy to
	// recognize (e.g. by secret scanners) and to tell apart from sessions.
	APIKeyTokenPrefix = "itx_"
	// APIKeyLastUsedGranularity is how stale LastUsedAt may get before a
	// request refreshes it; this avoids a database write per request.
	APIKeyLastUsedGranularity = time.Minute
	maxAPIKeyNameRunes        = 100
)

// APIKeyID is an API key's internal identifier.
type APIKeyID uuid.UUID

func (id APIKeyID) String() string { return uuid.UUID(id).String() }

// APIKey lets a service account call the API. Only a hash of the token is
// stored; the token itself is shown once, when the key is created.
type APIKey struct {
	ID     APIKeyID
	UserID UserID
	Name   string
	// TokenHash is the SHA-256 of the token.
	TokenHash []byte
	// Prefix is the start of the token, safe to display ("itx_AbCd1234").
	Prefix     string
	CreatedBy  UserID
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// NewAPIKey validates and builds a key for the service account owner.
// expiresAt is optional but must lie in the future.
func NewAPIKey(
	id APIKeyID, owner UserID, name string, tokenHash []byte, prefix string, createdBy UserID, expiresAt *time.Time, now time.Time,
) (APIKey, error) {
	f := fieldErrors{}
	name = checkRequiredText(f, "name", name, maxAPIKeyNameRunes)

	if expiresAt != nil && !expiresAt.After(now) {
		f.add("expiresAt", FieldOutOfRange)
	}

	if err := f.err(); err != nil {
		return APIKey{}, err
	}

	return APIKey{
		ID: id, UserID: owner, Name: name, TokenHash: tokenHash, Prefix: prefix,
		CreatedBy: createdBy, CreatedAt: now, ExpiresAt: expiresAt,
	}, nil
}

// Revoked reports whether the key was revoked.
func (k APIKey) Revoked() bool { return k.RevokedAt != nil }

// Expired reports whether the key has expired at now.
func (k APIKey) Expired(now time.Time) bool { return k.ExpiresAt != nil && !now.Before(*k.ExpiresAt) }

// Active reports whether the key may be used at now.
func (k APIKey) Active(now time.Time) bool { return !k.Revoked() && !k.Expired(now) }

// Revoke marks the key as revoked at now. Revoking twice keeps the first time.
func (k *APIKey) Revoke(now time.Time) {
	if k.RevokedAt == nil {
		k.RevokedAt = &now
	}
}

// TouchLastUsed records a use at now and reports whether LastUsedAt changed,
// i.e. whether it was older than APIKeyLastUsedGranularity.
func (k *APIKey) TouchLastUsed(now time.Time) bool {
	if k.LastUsedAt != nil && now.Sub(*k.LastUsedAt) < APIKeyLastUsedGranularity {
		return false
	}

	k.LastUsedAt = &now

	return true
}
