package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditEntryID is an audit log entry's internal identifier.
type AuditEntryID uuid.UUID

func (id AuditEntryID) String() string { return uuid.UUID(id).String() }

// AuditAction names what happened, as "<target type>.<verb>".
type AuditAction string

// Recorded actions.
const (
	AuditLogin            AuditAction = "auth.login"
	AuditLoginFailed      AuditAction = "auth.login_failed"
	AuditLogout           AuditAction = "auth.logout"
	AuditProjectCreate    AuditAction = "project.create"
	AuditProjectUpdate    AuditAction = "project.update"
	AuditIssueCreate      AuditAction = "issue.create"
	AuditIssueUpdate      AuditAction = "issue.update"
	AuditTagUpdate        AuditAction = "tag.update"
	AuditCommentCreate    AuditAction = "comment.create"
	AuditCommentUpdate    AuditAction = "comment.update"
	AuditCommentDelete    AuditAction = "comment.delete"
	AuditAttachmentCreate AuditAction = "attachment.create"
	AuditAttachmentDelete AuditAction = "attachment.delete"
	AuditStorageCreate    AuditAction = "storage.create"
	AuditStorageUpdate    AuditAction = "storage.update"

	AuditStorageItemCreate  AuditAction = "storage_item.create"
	AuditStorageItemUpdate  AuditAction = "storage_item.update"
	AuditStorageItemDelete  AuditAction = "storage_item.delete"
	AuditStorageItemRestore AuditAction = "storage_item.restore"

	AuditItemTypeCreate  AuditAction = "item_type.create"
	AuditItemTypeUpdate  AuditAction = "item_type.update"
	AuditItemTypeDelete  AuditAction = "item_type.delete"
	AuditItemTypeRestore AuditAction = "item_type.restore"

	AuditServiceAccountCreate AuditAction = "service_account.create"
	AuditAPIKeyCreate         AuditAction = "api_key.create"
	AuditAPIKeyRevoke         AuditAction = "api_key.revoke" //nolint:gosec // an action name, not a credential
)

// AuditActions lists every recorded action.
var AuditActions = []AuditAction{
	AuditLogin, AuditLoginFailed, AuditLogout, AuditProjectCreate, AuditProjectUpdate, AuditIssueCreate,
	AuditIssueUpdate, AuditTagUpdate, AuditCommentCreate, AuditCommentUpdate, AuditCommentDelete,
	AuditAttachmentCreate, AuditAttachmentDelete, AuditStorageCreate, AuditStorageUpdate,
	AuditStorageItemCreate, AuditStorageItemUpdate, AuditStorageItemDelete, AuditStorageItemRestore,
	AuditItemTypeCreate, AuditItemTypeUpdate, AuditItemTypeDelete, AuditItemTypeRestore,
	AuditServiceAccountCreate, AuditAPIKeyCreate, AuditAPIKeyRevoke,
}

// Valid reports whether a is a known action.
func (a AuditAction) Valid() bool { return slices.Contains(AuditActions, a) }

// Audit target types: the kind of entity an entry is about.
const (
	AuditTargetUser           = "user"
	AuditTargetProject        = "project"
	AuditTargetIssue          = "issue"
	AuditTargetTag            = "tag"
	AuditTargetComment        = "comment"
	AuditTargetAttachment     = "attachment"
	AuditTargetStorage        = "storage"
	AuditTargetStorageItem    = "storage_item"
	AuditTargetItemType       = "item_type"
	AuditTargetServiceAccount = "service_account"
	AuditTargetAPIKey         = "api_key"
)

// AuditEntry records one security-relevant event or change. Entries are
// append-only.
type AuditEntry struct {
	ID AuditEntryID
	At time.Time
	// ActorID is who acted; nil when nobody is known (a failed sign-in).
	ActorID *UserID
	Action  AuditAction
	// TargetType, TargetID and TargetLabel name what was acted on. The
	// label (e.g. "TRAX-12") is a snapshot for display.
	TargetType  string
	TargetID    string
	TargetLabel string
	Changes     AuditChanges
	// IP and UserAgent describe the request, when known.
	IP        string
	UserAgent string
}

// AuditChange is the change of one field. Old is nil for a field set on
// creation. Redacted changes (long text) record only that the field changed.
type AuditChange struct {
	Old      *string
	New      *string
	Redacted bool
}

// AuditChanges maps a field name to its change.
type AuditChanges map[string]AuditChange

// Set records field changing from before to after, unless they are equal.
func (c AuditChanges) Set(field, before, after string) {
	if before != after {
		c[field] = AuditChange{Old: &before, New: &after}
	}
}

// Text records that the long text field changed, without its contents.
func (c AuditChanges) Text(field, before, after string) {
	if before != after {
		c[field] = AuditChange{Redacted: true}
	}
}

// List records a change of a set of values (compared order-insensitively,
// stored comma-separated).
func (c AuditChanges) List(field string, before, after []string) {
	c.Set(field, joinSorted(before), joinSorted(after))
}

// Initial records field being set to value on creation; empty values are skipped.
func (c AuditChanges) Initial(field, value string) {
	if value != "" {
		c[field] = AuditChange{New: &value}
	}
}

// InitialText records that a long text field was set on creation.
func (c AuditChanges) InitialText(field, value string) {
	if value != "" {
		c[field] = AuditChange{Redacted: true}
	}
}

// InitialList records a set of values on creation; empty sets are skipped.
func (c AuditChanges) InitialList(field string, values []string) {
	c.Initial(field, joinSorted(values))
}

func joinSorted(values []string) string {
	s := slices.Clone(values)
	slices.Sort(s)

	return strings.Join(s, ", ")
}
