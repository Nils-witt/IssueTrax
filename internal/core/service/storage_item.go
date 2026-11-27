package service

import (
	"context"
	"errors"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that GenericStorageItemService satisfies the driving port.
var _ port.GenericStorageItemService = (*GenericStorageItemService)(nil)

// GenericStorageItemService implements port.GenericStorageItemService.
type GenericStorageItemService struct {
	repos Repositories
	opts  options
	kind  itemKind[domain.GenericStorageItem]
}

// NewGenericStorageItemService returns a GenericStorageItemService backed by repos.
func NewGenericStorageItemService(repos Repositories, opts ...Option) *GenericStorageItemService {
	o := newOptions(opts)

	return &GenericStorageItemService{repos: repos, opts: o, kind: itemKind[domain.GenericStorageItem]{
		repos: repos, opts: o, repo: repos.GenericStorageItems,
		base:  func(i *domain.GenericStorageItem) *domain.StorageItem { return &i.StorageItem },
		event: storageItemEvent,
	}}
}

// Create validates and stores a new item in in.StorageID.
func (s *GenericStorageItemService) Create(
	ctx context.Context, actor domain.UserID, in port.CreateGenericStorageItemInput,
) (domain.GenericStorageItem, error) {
	item, err := domain.NewGenericStorageItem(domain.NewGenericStorageItemParams{
		ID:          domain.StorageItemID(s.opts.newID()),
		StorageID:   in.StorageID,
		Name:        in.Name,
		Description: in.Description,
		Now:         s.opts.now(),
	})
	if err != nil {
		return domain.GenericStorageItem{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.checkStorage(ctx, item.StorageID); err != nil {
			return err
		}

		if err := s.repos.GenericStorageItems.Create(ctx, item); err != nil {
			return fmt.Errorf("create storage item %q: %w", item.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("name", item.Name)
		changes.InitialText("description", item.Description)
		changes.Initial("storage", item.StorageID.String())

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, domain.AuditGenericStorageItemCreate, item, changes))
	})
	if err != nil {
		return domain.GenericStorageItem{}, err
	}

	s.notify(ctx)

	return item, nil
}

// Get returns the item with id, also when it is deleted.
func (s *GenericStorageItemService) Get(ctx context.Context, id domain.StorageItemID) (domain.GenericStorageItem, error) {
	return s.kind.get(ctx, id)
}

// ListInStorage returns one page of the items in storage. An unknown
// storage is not found.
func (s *GenericStorageItemService) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	return s.kind.listInStorage(ctx, storage, filter, page)
}

// Search returns one page of items across all storages.
func (s *GenericStorageItemService) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	return s.kind.search(ctx, filter, page)
}

// Update edits and/or moves an item. Deleted items fail with
// domain.ErrStorageItemDeleted.
func (s *GenericStorageItemService) Update(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, in port.UpdateGenericStorageItemInput,
) (domain.GenericStorageItem, error) {
	var (
		item    domain.GenericStorageItem
		changed bool
	)

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if item, err = s.Get(ctx, id); err != nil {
			return err
		}

		before := item

		changes := domain.GenericStorageItemChanges{Name: in.Name, Description: in.Description, StorageID: in.StorageID}
		if err := item.Apply(changes, s.opts.now()); err != nil {
			return err
		}

		if item.StorageID != before.StorageID {
			if err := s.checkStorage(ctx, item.StorageID); err != nil {
				return err
			}
		}

		audit := domain.AuditChanges{}
		audit.Set("name", before.Name, item.Name)
		audit.Text("description", before.Description, item.Description)
		audit.Set("storage", before.StorageID.String(), item.StorageID.String())

		if len(audit) == 0 {
			return nil
		}

		changed = true

		if err := s.repos.GenericStorageItems.Update(ctx, item); err != nil {
			return fmt.Errorf("update storage item %s: %w", item.ID, err)
		}

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, domain.AuditGenericStorageItemUpdate, item, audit))
	})
	if err != nil {
		return domain.GenericStorageItem{}, err
	}

	if changed {
		s.notify(ctx)
	}

	return item, nil
}

// Delete marks an item deleted. An already deleted item fails with
// domain.ErrStorageItemDeleted.
func (s *GenericStorageItemService) Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error {
	_, err := s.kind.setDeleted(ctx, actor, id, domain.AuditGenericStorageItemDelete, (*domain.StorageItem).Delete)

	return err
}

// Restore undoes Delete. An item that is not deleted fails with
// domain.ErrStorageItemNotDeleted.
func (s *GenericStorageItemService) Restore(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID,
) (domain.GenericStorageItem, error) {
	return s.kind.setDeleted(ctx, actor, id, domain.AuditGenericStorageItemRestore, (*domain.StorageItem).Restore)
}

// checkStorage fails with domain.ErrStorageItemStorageNotFound unless
// storage exists.
func (s *GenericStorageItemService) checkStorage(ctx context.Context, storage domain.StorageID) error {
	return checkItemStorage(ctx, s.repos, storage)
}

// checkItemStorage fails with domain.ErrStorageItemStorageNotFound unless
// the storage an item is put in exists.
func checkItemStorage(ctx context.Context, repos Repositories, storage domain.StorageID) error {
	if _, err := repos.Storages.Get(ctx, storage); err != nil {
		if errors.Is(err, domain.ErrStorageNotFound) {
			return domain.ErrStorageItemStorageNotFound
		}

		return fmt.Errorf("get storage %s: %w", storage, err)
	}

	return nil
}

// notify announces a committed item change.
func (s *GenericStorageItemService) notify(ctx context.Context) { notifyStorageItems(ctx, s.opts) }

func storageItemEvent(
	actor domain.UserID, action domain.AuditAction, item domain.GenericStorageItem, changes domain.AuditChanges,
) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetGenericStorageItem, targetID: item.ID.String(), targetLabel: item.Name,
	}
}
