package storagetest

import (
	"bytes"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func apiKey(t *testing.T, r *service.Repositories, owner, by domain.User, name string, createdAt time.Time, expiresAt *time.Time) domain.APIKey {
	t.Helper()

	k := domain.APIKey{
		ID: domain.APIKeyID(newID()), UserID: owner.ID, Name: name, TokenHash: []byte("hash-" + name),
		Prefix: "itx_" + name, CreatedBy: by.ID, CreatedAt: createdAt, ExpiresAt: expiresAt,
	}
	must(t, r.APIKeys.Create(t.Context(), k))

	return k
}

func equalTimePtr(a, b *time.Time) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
}

func equalAPIKeys(a, b domain.APIKey) bool {
	return a.ID == b.ID && a.UserID == b.UserID && a.Name == b.Name && bytes.Equal(a.TokenHash, b.TokenHash) &&
		a.Prefix == b.Prefix && a.CreatedBy == b.CreatedBy && a.CreatedAt.Equal(b.CreatedAt) &&
		equalTimePtr(a.ExpiresAt, b.ExpiresAt) && equalTimePtr(a.LastUsedAt, b.LastUsedAt) && equalTimePtr(a.RevokedAt, b.RevokedAt)
}

func testAPIKeysLifecycle(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ctx := t.Context()
	ada := user(t, r, "ada")
	bot := serviceAccount(t, r, "ci")
	exp := now.Add(time.Hour)

	k := apiKey(t, &r, bot, ada, "deploy", now, &exp)

	got, err := r.APIKeys.Get(ctx, k.ID)
	if err != nil || !equalAPIKeys(got, k) {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, k)
	}

	got, err = r.APIKeys.GetByHash(ctx, k.TokenHash)
	if err != nil || !equalAPIKeys(got, k) {
		t.Fatalf("GetByHash() = %+v, %v; want %+v", got, err, k)
	}

	_, err = r.APIKeys.Get(ctx, domain.APIKeyID(newID()))
	wantErr(t, err, domain.ErrAPIKeyNotFound)

	_, err = r.APIKeys.GetByHash(ctx, []byte("unknown"))
	wantErr(t, err, domain.ErrAPIKeyNotFound)

	used, revoked := now.Add(time.Minute), now.Add(2*time.Minute)
	k.LastUsedAt, k.RevokedAt = &used, &revoked
	must(t, r.APIKeys.Update(ctx, k))

	got, err = r.APIKeys.Get(ctx, k.ID)
	if err != nil || !equalAPIKeys(got, k) {
		t.Fatalf("after Update() = %+v, %v; want %+v", got, err, k)
	}

	k.ID = domain.APIKeyID(newID())
	wantErr(t, r.APIKeys.Update(ctx, k), domain.ErrAPIKeyNotFound)
}

func testAPIKeysListByUser(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ada := user(t, r, "ada")
	ci, ops := serviceAccount(t, r, "ci"), serviceAccount(t, r, "ops")

	older := apiKey(t, &r, ci, ada, "older", now, nil)
	newer := apiKey(t, &r, ci, ada, "newer", now.Add(time.Hour), nil)
	apiKey(t, &r, ops, ada, "other", now, nil)

	list, err := r.APIKeys.ListByUser(t.Context(), ci.ID)
	must(t, err)

	if len(list) != 2 || !equalAPIKeys(list[0], newer) || !equalAPIKeys(list[1], older) {
		t.Fatalf("ListByUser() = %+v, want [newer older]", list)
	}

	list, err = r.APIKeys.ListByUser(t.Context(), ada.ID)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("ListByUser(no keys) = %#v, %v; want empty", list, err)
	}
}
