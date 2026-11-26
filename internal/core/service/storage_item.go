package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that GenericStorageItemService satisfies the driving port.
var _ port.GenericStorageItemService = (*GenericStorageItemService)(nil)

// GenericStorageItemService implements port.GenericStorageItemService.
type GenericStorageItemService struct {
	repos Repositories
	opts  options
}

// NewGenericStorageItemService returns a GenericStorageItemService backed by repos.
func NewGenericStorageItemService(repos Repositories, opts ...Option) *GenericStorageItemService {
	return &GenericStorageItemService{repos: repos, opts: newOptions(opts)}
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
	item, err := s.repos.GenericStorageItems.Get(ctx, id)
	if err != nil {
		return domain.GenericStorageItem{}, fmt.Errorf("get storage item %s: %w", id, err)
	}

	return item, nil
}

// ListInStorage returns one page of the items in storage. An unknown
// storage is not found.
func (s *GenericStorageItemService) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	if _, err := s.repos.Storages.Get(ctx, storage); err != nil {
		return domain.Page[domain.GenericStorageItem]{}, fmt.Errorf("get storage %s: %w", storage, err)
	}

	items, err := s.repos.GenericStorageItems.ListInStorage(ctx, storage, filter, page)
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, fmt.Errorf("list storage items: %w", err)
	}

	return items, nil
}

// Search returns one page of items across all storages.
func (s *GenericStorageItemService) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	items, err := s.repos.GenericStorageItems.Search(ctx, filter, page)
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, fmt.Errorf("search storage items: %w", err)
	}

	return items, nil
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
	_, err := s.setDeleted(ctx, actor, id, domain.AuditGenericStorageItemDelete, (*domain.StorageItem).Delete)

	return err
}

// Restore undoes Delete. An item that is not deleted fails with
// domain.ErrStorageItemNotDeleted.
func (s *GenericStorageItemService) Restore(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID,
) (domain.GenericStorageItem, error) {
	return s.setDeleted(ctx, actor, id, domain.AuditGenericStorageItemRestore, (*domain.StorageItem).Restore)
}

func (s *GenericStorageItemService) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, action domain.AuditAction,
	apply func(*domain.StorageItem, time.Time) error,
) (domain.GenericStorageItem, error) {
	var item domain.GenericStorageItem

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if item, err = s.Get(ctx, id); err != nil {
			return err
		}

		if err := apply(&item.StorageItem, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.GenericStorageItems.Update(ctx, item); err != nil {
			return fmt.Errorf("update storage item %s: %w", item.ID, err)
		}

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, action, item, nil))
	})
	if err != nil {
		return domain.GenericStorageItem{}, err
	}

	s.notify(ctx)

	return item, nil
}

// checkStorage fails with domain.ErrStorageItemStorageNotFound unless
// storage exists.
func (s *GenericStorageItemService) checkStorage(ctx context.Context, storage domain.StorageID) error {
	if _, err := s.repos.Storages.Get(ctx, storage); err != nil {
		if errors.Is(err, domain.ErrStorageNotFound) {
			return domain.ErrStorageItemStorageNotFound
		}

		return fmt.Errorf("get storage %s: %w", storage, err)
	}

	return nil
}

// notify announces a committed item change. It also changes item counts,
// which clients load with the storages.
func (s *GenericStorageItemService) notify(ctx context.Context) {
	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeStorageItems})
}

func storageItemEvent(
	actor domain.UserID, action domain.AuditAction, item domain.GenericStorageItem, changes domain.AuditChanges,
) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetGenericStorageItem, targetID: item.ID.String(), targetLabel: item.Name,
	}
}
