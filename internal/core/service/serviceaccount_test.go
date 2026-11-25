package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func (f *authFixture) addBot(t *testing.T, name string) domain.User {
	t.Helper()

	bot, err := f.accounts.Create(t.Context(), f.addUser(t, "creator-of-"+name), name)
	if err != nil {
		t.Fatalf("create service account %q: %v", name, err)
	}

	return bot
}

func TestBotService_Create(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	ada := f.addUser(t, "ada")

	bot, err := f.accounts.Create(t.Context(), ada, "Bot-CI")
	if err != nil {
		t.Fatal(err)
	}

	if bot.DisplayName != "bot-ci" || !bot.IsServiceAccount() || !bot.CreatedAt.Equal(testNow) {
		t.Fatalf("Create() = %+v", bot)
	}

	_, err = f.accounts.Create(t.Context(), ada, "ci")
	wantCode(t, err, domain.CodeServiceAccountNameTaken)

	_, err = f.accounts.Create(t.Context(), ada, "no spaces")
	wantField(t, err, "name", domain.FieldInvalid)

	_, err = f.accounts.Create(t.Context(), bot, "sneaky")
	wantCode(t, err, domain.CodeSessionRequired)

	f.addBot(t, "alpha")

	list, err := f.accounts.List(t.Context())
	if err != nil || len(list) != 2 || list[0].DisplayName != "bot-alpha" || list[1].ID != bot.ID {
		t.Fatalf("List() = %+v, %v; want [bot-alpha bot-ci]", list, err)
	}
}

func TestBotService_CreateKey(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	ada := f.addUser(t, "ada")
	bot := f.addBot(t, "ci")
	exp := testNow.Add(24 * time.Hour)

	key, token, err := f.accounts.CreateKey(t.Context(), ada, bot.ID, " deploy ", &exp)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(token, "itx_") || !strings.HasPrefix(token, key.Prefix) || len(key.Prefix) != 12 ||
		key.Name != "deploy" || key.CreatedBy != ada.ID || key.UserID != bot.ID || !key.ExpiresAt.Equal(exp) {
		t.Fatalf("CreateKey() = %+v, %q", key, token)
	}

	_, _, err = f.accounts.CreateKey(t.Context(), bot, bot.ID, "self", nil)
	wantCode(t, err, domain.CodeSessionRequired)

	_, _, err = f.accounts.CreateKey(t.Context(), ada, ada.ID, "person", nil)
	wantCode(t, err, domain.CodeServiceAccountNotFound)

	_, _, err = f.accounts.CreateKey(t.Context(), ada, bot.ID, "", nil)
	wantField(t, err, "name", domain.FieldRequired)

	keys, err := f.accounts.ListKeys(t.Context(), bot.ID)
	if err != nil || len(keys) != 1 || keys[0].ID != key.ID {
		t.Fatalf("ListKeys() = %+v, %v", keys, err)
	}

	_, err = f.accounts.ListKeys(t.Context(), domain.UserID(uuid.New()))
	wantCode(t, err, domain.CodeServiceAccountNotFound)
}

func TestBotService_RevokeKey(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	ada := f.addUser(t, "ada")
	bot, other := f.addBot(t, "ci"), f.addBot(t, "other")

	key, _, err := f.accounts.CreateKey(t.Context(), ada, bot.ID, "deploy", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.accounts.RevokeKey(t.Context(), ada, other.ID, key.ID)
	wantCode(t, err, domain.CodeAPIKeyNotFound)

	_, err = f.accounts.RevokeKey(t.Context(), bot, bot.ID, key.ID)
	wantCode(t, err, domain.CodeSessionRequired)

	revoked, err := f.accounts.RevokeKey(t.Context(), ada, bot.ID, key.ID)
	if err != nil || revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(testNow) {
		t.Fatalf("RevokeKey() = %+v, %v", revoked, err)
	}

	if _, err := f.accounts.RevokeKey(t.Context(), ada, bot.ID, key.ID); err != nil {
		t.Fatalf("second RevokeKey() = %v", err)
	}
}

func TestAuthService_AuthenticateAPIKey(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	ada := f.addUser(t, "ada")
	bot := f.addBot(t, "ci")
	exp := testNow.Add(time.Hour)

	key, token, err := f.accounts.CreateKey(t.Context(), ada, bot.ID, "deploy", &exp)
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.auth.AuthenticateAPIKey(t.Context(), token)
	if err != nil || got.ID != bot.ID {
		t.Fatalf("AuthenticateAPIKey() = %+v, %v", got, err)
	}

	stored, _ := f.repos.APIKeys.Get(t.Context(), key.ID)
	if stored.LastUsedAt == nil || !stored.LastUsedAt.Equal(testNow) {
		t.Fatalf("LastUsedAt = %v, want %v", stored.LastUsedAt, testNow)
	}

	for _, bad := range []string{"", "itx_unknown", strings.TrimPrefix(token, "itx_"), token + "x"} {
		_, err := f.auth.AuthenticateAPIKey(t.Context(), bad)
		wantCode(t, err, domain.CodeInvalidAPIKey)
	}

	*f.clock = exp
	_, err = f.auth.AuthenticateAPIKey(t.Context(), token)
	wantCode(t, err, domain.CodeInvalidAPIKey)

	*f.clock = testNow
	if _, err := f.accounts.RevokeKey(t.Context(), ada, bot.ID, key.ID); err != nil {
		t.Fatal(err)
	}

	_, err = f.auth.AuthenticateAPIKey(t.Context(), token)
	wantCode(t, err, domain.CodeInvalidAPIKey)
}

func TestAuthService_AuthenticateAPIKey_RejectsPeople(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	ada := f.addUser(t, "ada")

	// A key can only be created for service accounts; plant one directly.
	token := "itx_" + strings.Repeat("a", 43)

	key, err := domain.NewAPIKey(domain.APIKeyID(uuid.New()), ada.ID, "planted", service.HashTokenForTest(token), "itx_aaaaaaaa", ada.ID, nil, testNow)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repos.APIKeys.Create(t.Context(), key); err != nil {
		t.Fatal(err)
	}

	_, err = f.auth.AuthenticateAPIKey(t.Context(), token)
	wantCode(t, err, domain.CodeInvalidAPIKey)
}
