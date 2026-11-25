package domain_test

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestSession_Expired(t *testing.T) {
	t.Parallel()

	s := domain.NewSession([]byte("h"), domain.UserID{}, "id", testNow)

	if !s.ExpiresAt.Equal(testNow.Add(domain.SessionLifetime)) {
		t.Fatalf("ExpiresAt = %v", s.ExpiresAt)
	}

	if s.Expired(testNow.Add(domain.SessionLifetime - 1)) {
		t.Fatal("Expired() just before the lifetime = true")
	}

	if !s.Expired(testNow.Add(domain.SessionLifetime)) {
		t.Fatal("Expired() at the lifetime = false")
	}
}
