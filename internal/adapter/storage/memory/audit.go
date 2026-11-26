package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AuditRepository satisfies the driven port.
var _ port.AuditRepository = (*AuditRepository)(nil)

// AuditRepository implements port.AuditRepository on a Store.
type AuditRepository struct{ s *Store }

// NewAuditRepository returns an AuditRepository backed by s.
func NewAuditRepository(s *Store) *AuditRepository { return &AuditRepository{s: s} }

// Append stores entry.
func (r *AuditRepository) Append(_ context.Context, entry domain.AuditEntry) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.audit = append(r.s.data.audit, cloneAuditEntry(entry))

	return nil
}

// List returns one page of matching entries, newest first.
func (r *AuditRepository) List(_ context.Context, filter port.AuditFilter, page domain.PageRequest) (domain.Page[domain.AuditEntry], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.AuditCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.AuditEntry]{}, err
	}

	r.s.mu.RLock()
	all := slices.DeleteFunc(slices.Clone(r.s.data.audit), func(e domain.AuditEntry) bool { return !auditMatches(e, filter) })
	r.s.mu.RUnlock()

	slices.SortFunc(all, func(a, b domain.AuditEntry) int { return compareAudit(b, a) })

	after := all
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(all), func(e domain.AuditEntry) bool {
			return cmp.Or(e.At.Compare(cursor.At), cmp.Compare(e.ID.String(), cursor.ID)) >= 0
		})
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.AuditEntry]{Total: int64(len(all))}

	for _, e := range items {
		result.Items = append(result.Items, cloneAuditEntry(e))
	}

	if result.Items == nil {
		result.Items = []domain.AuditEntry{}
	}

	if more {
		last := items[len(items)-1]
		result.NextCursor = domain.EncodeCursor(domain.AuditCursor{At: last.At, ID: last.ID.String()})
	}

	return result, nil
}

// DeleteBefore removes entries older than t.
func (r *AuditRepository) DeleteBefore(_ context.Context, t time.Time) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	before := len(r.s.data.audit)
	r.s.data.audit = slices.DeleteFunc(r.s.data.audit, func(e domain.AuditEntry) bool { return e.At.Before(t) })

	return int64(before - len(r.s.data.audit)), nil
}

func auditMatches(e domain.AuditEntry, f port.AuditFilter) bool {
	switch {
	case f.ActorID != nil && (e.ActorID == nil || *e.ActorID != *f.ActorID):
		return false
	case len(f.Actions) > 0 && !slices.Contains(f.Actions, e.Action):
		return false
	case f.TargetType != "" && e.TargetType != f.TargetType:
		return false
	}

	return true
}

// compareAudit orders entries by time, then ID (as text, like the cursor).
func compareAudit(a, b domain.AuditEntry) int {
	return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.ID.String(), b.ID.String()))
}

func cloneAuditEntry(e domain.AuditEntry) domain.AuditEntry {
	e.Changes = maps.Clone(e.Changes)

	if e.ActorID != nil {
		actor := *e.ActorID
		e.ActorID = &actor
	}

	return e
}
