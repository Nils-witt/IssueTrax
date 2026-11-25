package storagetest

import (
	"bytes"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func testSessionsLifecycle(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ada := user(t, r, "ada")
	ctx := t.Context()

	fresh := domain.NewSession([]byte("fresh-hash"), ada.ID, "id-token", now)
	stale := domain.NewSession([]byte("stale-hash"), ada.ID, "old", now.Add(-domain.SessionLifetime))

	must(t, r.Sessions.Create(ctx, fresh))
	must(t, r.Sessions.Create(ctx, stale))

	got, err := r.Sessions.Get(ctx, fresh.TokenHash)
	must(t, err)

	if !bytes.Equal(got.TokenHash, fresh.TokenHash) || got.UserID != ada.ID || got.IDToken != "id-token" ||
		!got.CreatedAt.Equal(fresh.CreatedAt) || !got.ExpiresAt.Equal(fresh.ExpiresAt) {
		t.Fatalf("Get() = %+v, want %+v", got, fresh)
	}

	_, err = r.Sessions.Get(ctx, []byte("unknown"))
	wantErr(t, err, domain.ErrNoSession)

	n, err := r.Sessions.DeleteExpired(ctx, now.Add(time.Second))
	must(t, err)

	if n != 1 {
		t.Fatalf("DeleteExpired() = %d, want 1", n)
	}

	_, err = r.Sessions.Get(ctx, stale.TokenHash)
	wantErr(t, err, domain.ErrNoSession)

	must(t, r.Sessions.Delete(ctx, fresh.TokenHash))
	must(t, r.Sessions.Delete(ctx, fresh.TokenHash)) // deleting twice is fine

	_, err = r.Sessions.Get(ctx, fresh.TokenHash)
	wantErr(t, err, domain.ErrNoSession)
}
