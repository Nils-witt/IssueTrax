// Package service implements the IssueTrax use cases (driving ports) on top
// of the driven ports.
package service

import (
	"context"
	"errors"
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
	// Attachments stores files attached to issues.
	Attachments port.AttachmentRepository
	// Storages stores the physical storage location tree.
	Storages port.StorageRepository
	// ItemTypes stores the item type tree.
	ItemTypes port.ItemTypeRepository
	// StorageItems stores the items kept in storages with their modules.
	StorageItems port.StorageItemRepository
	// Manufacturers and HardwareModels store the central list hardware
	// modules pick from.
	Manufacturers  port.ManufacturerRepository
	HardwareModels port.HardwareModelRepository
	// AssetTagCounters stores the per-prefix counters of proposed asset tags.
	AssetTagCounters port.AssetTagCounterRepository
	Users            port.UserRepository
	Sessions         port.SessionRepository
	APIKeys          port.APIKeyRepository
	Audit            port.AuditRepository
}

// Option customizes a service.
type Option func(*options)

type options struct {
	now        func() time.Time
	newID      func() uuid.UUID
	adminGroup string
	changes    port.ChangeNotifier
	// assetTagPrefix starts proposed asset tags when no type in the chain
	// has a prefix.
	assetTagPrefix string
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

// WithChangeNotifier announces every committed change to n, so connected
// clients can refresh. Without it, changes are not announced.
func WithChangeNotifier(n port.ChangeNotifier) Option {
	return func(o *options) { o.changes = n }
}

// WithAssetTagPrefix sets the prefix proposed asset tags start with when
// neither an item's type nor any of its ancestors has one. Without it,
// such tags are plain numbers.
func WithAssetTagPrefix(prefix string) Option {
	return func(o *options) { o.assetTagPrefix = prefix }
}

// notify announces a committed change. Call it only after the use case's
// transaction succeeded, so listeners never reload uncommitted data.
func (o options) notify(ctx context.Context, change domain.Change) {
	if o.changes != nil {
		o.changes.Notify(ctx, change)
	}
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

// issueByNumber loads issue number of the project with a raw key.
func issueByNumber(ctx context.Context, repos Repositories, key domain.ProjectKey, number int64) (domain.Issue, error) {
	project, err := projectByKey(ctx, repos.Projects, key)
	if err != nil {
		return domain.Issue{}, err
	}

	issue, err := repos.Issues.Get(ctx, project.ID, number)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("get issue %s-%d: %w", project.Key, number, err)
	}

	return issue, nil
}

// editableIssue loads the issue and rejects archived ones. Inside a
// transaction the issue row stays locked, so its status cannot change
// before the dependent change (a comment, an attachment) is written.
func editableIssue(ctx context.Context, repos Repositories, key domain.ProjectKey, number int64) (domain.Issue, error) {
	issue, err := issueByNumber(ctx, repos, key, number)
	if err != nil {
		return domain.Issue{}, err
	}

	if err := issue.CheckEditable(false); err != nil {
		return domain.Issue{}, err
	}

	return issue, nil
}

// withUsers builds a view of each item with the user userOf names
// resolved, loading all users with one lookup; what names those users in
// errors. Unknown users are passed as a User with only the ID set.
func withUsers[T, V any](
	ctx context.Context, repos Repositories, what string, items []T, userOf func(T) domain.UserID, view func(T, domain.User) V,
) ([]V, error) {
	ids := make([]domain.UserID, 0, len(items))
	for _, item := range items {
		ids = append(ids, userOf(item))
	}

	users, err := repos.Users.GetMany(ctx, unique(ids))
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", what, err)
	}

	byID := make(map[domain.UserID]domain.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	views := make([]V, 0, len(items))

	for _, item := range items {
		id := userOf(item)

		user, ok := byID[id]
		if !ok {
			user = domain.User{ID: id}
		}

		views = append(views, view(item, user))
	}

	return views, nil
}

// requireAdmin fails with domain.ErrAdminRequired unless actor is an admin
// person (not a service account).
func requireAdmin(ctx context.Context, repos Repositories, actor domain.UserID) error {
	user, err := repos.Users.Get(ctx, actor)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrAdminRequired
		}

		return fmt.Errorf("get user %s: %w", actor, err)
	}

	if !user.Admin || user.IsServiceAccount() {
		return domain.ErrAdminRequired
	}

	return nil
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
