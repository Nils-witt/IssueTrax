package domain_test

import (
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewServiceAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in       string
		wantName string
		fields   map[string]string
	}{
		{in: "ci", wantName: "ci"},
		{in: "  Bot-Deploy-1 ", wantName: "deploy-1"},
		{in: "", fields: map[string]string{"name": domain.FieldRequired}},
		{in: "bot-", fields: map[string]string{"name": domain.FieldRequired}},
		{in: "-ci", fields: map[string]string{"name": domain.FieldInvalid}},
		{in: "has space", fields: map[string]string{"name": domain.FieldInvalid}},
		{in: "a2345678901234567890123456789012345678901", fields: map[string]string{"name": domain.FieldInvalid}},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			u, err := domain.NewServiceAccount(domain.UserID{1}, tt.in, testNow)
			wantFields(t, err, tt.fields)

			if tt.fields != nil {
				return
			}

			if !u.IsServiceAccount() || u.ServiceAccountName() != tt.wantName || u.DisplayName != "bot-"+tt.wantName ||
				u.Issuer != domain.ServiceAccountIssuer || u.Email != "" {
				t.Fatalf("NewServiceAccount(%q) = %+v", tt.in, u)
			}
		})
	}
}

func TestUser_IsServiceAccount(t *testing.T) {
	t.Parallel()

	u, err := domain.NewUser(domain.UserID{1}, domain.Identity{Issuer: "https://idp", Subject: "ci"}, testNow)
	if err != nil {
		t.Fatal(err)
	}

	if u.IsServiceAccount() {
		t.Fatal("OIDC user reported as service account")
	}
}

func TestNewAPIKey(t *testing.T) {
	t.Parallel()

	past, future := testNow.Add(-time.Hour), testNow.Add(time.Hour)

	tests := []struct {
		name      string
		keyName   string
		expiresAt *time.Time
		fields    map[string]string
	}{
		{name: "no expiry", keyName: " deploy "},
		{name: "future expiry", keyName: "deploy", expiresAt: &future},
		{name: "past expiry", keyName: "deploy", expiresAt: &past, fields: map[string]string{"expiresAt": domain.FieldOutOfRange}},
		{name: "now expiry", keyName: "deploy", expiresAt: &testNow, fields: map[string]string{"expiresAt": domain.FieldOutOfRange}},
		{name: "missing name", keyName: " ", fields: map[string]string{"name": domain.FieldRequired}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			k, err := domain.NewAPIKey(domain.APIKeyID{1}, domain.UserID{2}, tt.keyName, []byte("h"), "itx_abc", domain.UserID{3}, tt.expiresAt, testNow)
			wantFields(t, err, tt.fields)

			if tt.fields == nil && (k.Name != "deploy" || !k.Active(testNow) || !k.CreatedAt.Equal(testNow)) {
				t.Fatalf("NewAPIKey() = %+v", k)
			}
		})
	}
}

func TestAPIKey_Lifecycle(t *testing.T) {
	t.Parallel()

	exp := testNow.Add(time.Hour)
	k := domain.APIKey{ExpiresAt: &exp}

	if !k.Active(testNow) || k.Active(exp) {
		t.Fatal("expiry not honoured")
	}

	if !k.TouchLastUsed(testNow) || k.TouchLastUsed(testNow.Add(30*time.Second)) || !k.TouchLastUsed(testNow.Add(time.Minute)) {
		t.Fatal("TouchLastUsed granularity wrong")
	}

	k.Revoke(testNow)
	k.Revoke(testNow.Add(time.Minute))

	if k.Active(testNow) || !k.RevokedAt.Equal(testNow) {
		t.Fatalf("after Revoke: %+v", k)
	}
}
