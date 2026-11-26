package service

import (
	"context"
	"fmt"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AuditService satisfies the driving port.
var _ port.AuditService = (*AuditService)(nil)

// AuditService implements port.AuditService.
type AuditService struct {
	repos     Repositories
	retention time.Duration
	opts      options
}

// NewAuditService returns an AuditService backed by repos. Entries older
// than retention are removed by DeleteExpired; 0 keeps them forever.
func NewAuditService(repos Repositories, retention time.Duration, opts ...Option) *AuditService {
	return &AuditService{repos: repos, retention: retention, opts: newOptions(opts)}
}

// List returns one page of the audit log, newest first, for admins only.
func (s *AuditService) List(
	ctx context.Context, viewer domain.User, filter port.AuditFilter, page domain.PageRequest,
) (domain.Page[port.AuditEntryView], error) {
	if !viewer.Admin || viewer.IsServiceAccount() {
		return domain.Page[port.AuditEntryView]{}, domain.ErrAdminRequired
	}

	for _, a := range filter.Actions {
		if !a.Valid() {
			return domain.Page[port.AuditEntryView]{}, domain.NewValidationError(map[string]string{"action": domain.FieldInvalid})
		}
	}

	entries, err := s.repos.Audit.List(ctx, filter, page)
	if err != nil {
		return domain.Page[port.AuditEntryView]{}, fmt.Errorf("list audit log: %w", err)
	}

	ids := make([]domain.UserID, 0, len(entries.Items))
	for _, e := range entries.Items {
		if e.ActorID != nil {
			ids = append(ids, *e.ActorID)
		}
	}

	users, err := s.repos.Users.GetMany(ctx, unique(ids))
	if err != nil {
		return domain.Page[port.AuditEntryView]{}, fmt.Errorf("get audit actors: %w", err)
	}

	byID := make(map[domain.UserID]domain.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	views := make([]port.AuditEntryView, 0, len(entries.Items))

	for _, e := range entries.Items {
		v := port.AuditEntryView{Entry: e}

		if e.ActorID != nil {
			actor, ok := byID[*e.ActorID]
			if !ok {
				actor = domain.User{ID: *e.ActorID}
			}

			v.Actor = &actor
		}

		views = append(views, v)
	}

	return domain.Page[port.AuditEntryView]{Items: views, Total: entries.Total, NextCursor: entries.NextCursor}, nil
}

// DeleteExpired removes entries older than the retention period.
func (s *AuditService) DeleteExpired(ctx context.Context) (int64, error) {
	if s.retention <= 0 {
		return 0, nil
	}

	n, err := s.repos.Audit.DeleteBefore(ctx, s.opts.now().Add(-s.retention))
	if err != nil {
		return 0, fmt.Errorf("delete expired audit entries: %w", err)
	}

	return n, nil
}
