package domain

// ChangeKind names what kind of data a Change touched.
type ChangeKind string

// Change kinds. Each tells clients which cached data to reload.
const (
	// ChangeProject: a project was created or its name/description changed.
	ChangeProject ChangeKind = "project"
	// ChangeTags: a project's tags were renamed or recolored, which also
	// changes how its issues look.
	ChangeTags ChangeKind = "tags"
	// ChangeIssue: an issue was created or updated (possibly creating tags).
	ChangeIssue ChangeKind = "issue"
	// ChangeComments: a comment on an issue was added, edited or deleted.
	ChangeComments ChangeKind = "comments"
	// ChangeAttachments: a file was attached to or removed from an issue.
	ChangeAttachments ChangeKind = "attachments"
	// ChangeStorages: a storage was created, renamed or moved. Storages are
	// global, so Project is empty.
	ChangeStorages ChangeKind = "storages"
	// ChangeStorageItems: an item kept in a storage was created, changed,
	// moved, deleted or restored, which also changes storage item counts.
	// Items are global, so Project is empty.
	ChangeStorageItems ChangeKind = "storageItems"
	// ChangeItemTypes: an item type was created, changed, moved, deleted or
	// restored. Types are global, so Project is empty.
	ChangeItemTypes ChangeKind = "itemTypes"
	// ChangeManufacturers: a manufacturer or hardware model was created,
	// renamed, deleted or restored, which also changes how items naming it
	// look. The list is global, so Project is empty.
	ChangeManufacturers ChangeKind = "manufacturers"
)

// Change announces that data was committed. It carries only what changed,
// never the data itself: listeners reload what they show through the
// regular read use cases, which keeps one source of truth.
type Change struct {
	Kind    ChangeKind
	Project ProjectKey
	// Issue is the issue number for ChangeIssue, ChangeComments and
	// ChangeAttachments, else 0.
	Issue int64
}
