package domain_test

import (
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		ident           domain.Identity
		wantDisplayName string
		wantFields      map[string]string
	}{
		{
			name:            "full profile",
			ident:           domain.Identity{Issuer: "iss", Subject: "sub", Email: "a@example.com", DisplayName: " Ada "},
			wantDisplayName: "Ada",
		},
		{
			name:            "falls back to email",
			ident:           domain.Identity{Issuer: "iss", Subject: "sub", Email: "a@example.com"},
			wantDisplayName: "a@example.com",
		},
		{
			name:            "falls back to subject",
			ident:           domain.Identity{Issuer: "iss", Subject: "sub"},
			wantDisplayName: "sub",
		},
		{
			name:       "missing identity",
			ident:      domain.Identity{DisplayName: "Ada"},
			wantFields: map[string]string{"issuer": domain.FieldRequired, "subject": domain.FieldRequired},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewUser(domain.UserID{}, tt.ident, testNow)
			wantFields(t, err, tt.wantFields)

			if tt.wantFields != nil {
				return
			}

			if got.DisplayName != tt.wantDisplayName || !got.CreatedAt.Equal(testNow) || !got.LastLoginAt.Equal(testNow) {
				t.Fatalf("NewUser() = %+v", got)
			}
		})
	}
}

func TestUser_RecordLogin(t *testing.T) {
	t.Parallel()

	u, err := domain.NewUser(domain.UserID{}, domain.Identity{Issuer: "iss", Subject: "sub", DisplayName: "Old"}, testNow)
	if err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)
	u.RecordLogin(domain.Identity{Email: "new@example.com", DisplayName: "New"}, later)

	if u.DisplayName != "New" || u.Email != "new@example.com" || !u.LastLoginAt.Equal(later) || !u.CreatedAt.Equal(testNow) {
		t.Fatalf("RecordLogin() = %+v", u)
	}

	if u.Issuer != "iss" || u.Subject != "sub" {
		t.Fatalf("RecordLogin() changed identity: %+v", u)
	}
}
