package storagetest

import (
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func testUsersCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ada := user(t, r, "ada")

	got, err := r.Users.Get(t.Context(), ada.ID)
	if err != nil || got != ada {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, ada)
	}

	got, err = r.Users.GetByIdentity(t.Context(), ada.Issuer, ada.Subject)
	if err != nil || got != ada {
		t.Fatalf("GetByIdentity() = %+v, %v; want %+v", got, err, ada)
	}

	_, err = r.Users.GetByIdentity(t.Context(), "https://other-idp", ada.Subject)
	wantErr(t, err, domain.ErrUserNotFound)

	ada.Email, ada.DisplayName, ada.LastLoginAt = "new@example.com", "Ada L.", now.Add(time.Hour)
	must(t, r.Users.Update(t.Context(), ada))

	got, err = r.Users.Get(t.Context(), ada.ID)
	if err != nil || got != ada {
		t.Fatalf("after Update() = %+v, %v; want %+v", got, err, ada)
	}

	ada.ID = domain.UserID(newID())
	wantErr(t, r.Users.Update(t.Context(), ada), domain.ErrUserNotFound)

	_, err = r.Users.Get(t.Context(), ada.ID)
	wantErr(t, err, domain.ErrUserNotFound)
}

func testUsersListAndGetMany(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	zoe, ada, bob := user(t, r, "zoe"), user(t, r, "Ada"), user(t, r, "bob")

	list, err := r.Users.List(t.Context())
	must(t, err)

	if len(list) != 3 || list[0] != ada || list[1] != bob || list[2] != zoe {
		t.Fatalf("List() = %+v, want [Ada bob zoe]", list)
	}

	many, err := r.Users.GetMany(t.Context(), []domain.UserID{zoe.ID, domain.UserID(newID())})
	if err != nil || len(many) != 1 || many[0] != zoe {
		t.Fatalf("GetMany() = %+v, %v", many, err)
	}
}
