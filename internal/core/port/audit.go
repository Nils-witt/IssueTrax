package port

import (
	"context"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// AuditFilter narrows the audit log. Different criteria combine with AND;
// zero values do not filter.
type AuditFilter struct {
	ActorID *domain.UserID
	// Actions to include; empty means all.
	Actions    []domain.AuditAction
	TargetType string
}

// AuditEntryView is an audit entry with its actor resolved. Actor is nil
// when the entry has none.
type AuditEntryView struct {
	Entry domain.AuditEntry
	Actor *domain.User
}

// AuditService is the driving port for reading the audit log. Entries are
// written by the other services as a side effect of their use cases.
type AuditService interface {
	// List returns one page of entries, newest first. Only admins may read
	// it; others get domain.ErrAdminRequired.
	List(ctx context.Context, viewer domain.User, filter AuditFilter, page domain.PageRequest) (domain.Page[AuditEntryView], error)
	// DeleteExpired removes entries older than the retention period and
	// reports how many; it does nothing when entries are kept forever.
	DeleteExpired(ctx context.Context) (int64, error)
}

// AuditRepository is the driven port for persisting audit entries.
//
// List orders by At, then ID, both descending. DeleteBefore removes entries
// with At before t.
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
	List(ctx context.Context, filter AuditFilter, page domain.PageRequest) (domain.Page[domain.AuditEntry], error)
	DeleteBefore(ctx context.Context, t time.Time) (int64, error)
}

// RequestInfo describes the request a use case runs for. Inbound adapters
// attach it to the context so audit entries can record where a change
// came from.
type RequestInfo struct {
	IP        string
	UserAgent string
}

type requestInfoKey struct{}

// WithRequestInfo returns a copy of ctx carrying info.
func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, info)
}

// RequestInfoFrom returns the RequestInfo attached to ctx, if any.
func RequestInfoFrom(ctx context.Context) RequestInfo {
	info, _ := ctx.Value(requestInfoKey{}).(RequestInfo)

	return info
}
