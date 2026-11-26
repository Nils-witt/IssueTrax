package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// UserID is a user's internal identifier.
type UserID uuid.UUID

func (id UserID) String() string { return uuid.UUID(id).String() }

// Identity is what the external identity provider asserts about a person.
type Identity struct {
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	// Groups are the identity provider groups the person belongs to.
	Groups []string
}

// User is a person known to IssueTrax. Users are created on their first
// login; only known users can be assigned to issues.
type User struct {
	ID          UserID
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	// Admin is true for people in the configured identity provider admin
	// group as of their latest sign-in. Admins may read the audit log.
	Admin       bool
	CreatedAt   time.Time
	LastLoginAt time.Time
}

// NewUser builds a User from a freshly authenticated identity.
func NewUser(id UserID, ident Identity, now time.Time) (User, error) {
	f := fieldErrors{}

	if strings.TrimSpace(ident.Issuer) == "" {
		f.add("issuer", FieldRequired)
	}

	if strings.TrimSpace(ident.Subject) == "" {
		f.add("subject", FieldRequired)
	}

	if err := f.err(); err != nil {
		return User{}, err
	}

	u := User{ID: id, Issuer: ident.Issuer, Subject: ident.Subject, CreatedAt: now}
	u.RecordLogin(ident, now)

	return u, nil
}

// RecordLogin refreshes profile data from ident and stamps the login time.
// The display name falls back to the email, then the subject.
func (u *User) RecordLogin(ident Identity, now time.Time) {
	u.Email = strings.TrimSpace(ident.Email)
	u.DisplayName = strings.TrimSpace(ident.DisplayName)

	if u.DisplayName == "" {
		u.DisplayName = u.Email
	}

	if u.DisplayName == "" {
		u.DisplayName = u.Subject
	}

	u.LastLoginAt = now
}
