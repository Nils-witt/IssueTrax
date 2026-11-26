// Package service implements the IssueTrax use cases (driving ports) on top
// of the driven ports.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Repositories bundles the driven ports the services depend on.
type Repositories struct {
	Tx       port.TxManager
	Projects port.ProjectRepository
	Issues   port.IssueRepository
	Tags     port.TagRepository
	Comments port.CommentRepository
	Users    port.UserRepository
	Sessions port.SessionRepository
	APIKeys  port.APIKeyRepository
	Audit    port.AuditRepository
}

// Option customizes a service.
type Option func(*options)

type options struct {
	now        func() time.Time
	newID      func() uuid.UUID
	adminGroup string
}

// WithClock overrides the time source (useful in tests).
func WithClock(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// WithIDGenerator overrides how new entity IDs are generated.
func WithIDGenerator(newID func() uuid.UUID) Option {
	return func(o *options) { o.newID = newID }
}

// WithAdminGroup makes members of the identity provider group name admins
// (AuthService). Without it, nobody is an admin.
func WithAdminGroup(name string) Option {
	return func(o *options) { o.adminGroup = name }
}

func newOptions(opts []Option) options {
	o := options{
		// Microseconds match Postgres timestamptz precision, so values
		// survive a storage round trip unchanged.
		now: func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) },
		// NewV7 only fails if crypto/rand fails, which crashes the program anyway.
		newID: func() uuid.UUID { return uuid.Must(uuid.NewV7()) },
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// projectByKey loads a project by a raw key; malformed keys are not found.
func projectByKey(ctx context.Context, repo port.ProjectRepository, key domain.ProjectKey) (domain.Project, error) {
	key = domain.NormalizeProjectKey(string(key))
	if !key.Valid() {
		return domain.Project{}, fmt.Errorf("get project %q: %w", key, domain.ErrProjectNotFound)
	}

	project, err := repo.GetByKey(ctx, key)
	if err != nil {
		return domain.Project{}, fmt.Errorf("get project %q: %w", key, err)
	}

	return project, nil
}

// unique returns s without duplicates, keeping first occurrences in order.
func unique[T comparable](s []T) []T {
	out := make([]T, 0, len(s))

	seen := make(map[T]struct{}, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}

	return out
}

// auditEvent is what a use case knows about an audit entry; record adds
// the ID, time and request details.
type auditEvent struct {
	actor       *domain.UserID
	action      domain.AuditAction
	targetType  string
	targetID    string
	targetLabel string
	changes     domain.AuditChanges
}

// record appends an audit entry. Call it with the ctx of the use case's
// transaction so the entry is written if and only if the change is.
func record(ctx context.Context, repos Repositories, opts options, ev auditEvent) error {
	info := port.RequestInfoFrom(ctx)

	changes := ev.changes
	if changes == nil {
		changes = domain.AuditChanges{}
	}

	err := repos.Audit.Append(ctx, domain.AuditEntry{
		ID:          domain.AuditEntryID(opts.newID()),
		At:          opts.now(),
		ActorID:     ev.actor,
		Action:      ev.action,
		TargetType:  ev.targetType,
		TargetID:    ev.targetID,
		TargetLabel: ev.targetLabel,
		Changes:     changes,
		IP:          info.IP,
		UserAgent:   info.UserAgent,
	})
	if err != nil {
		return fmt.Errorf("record audit entry %s: %w", ev.action, err)
	}

	return nil
}
