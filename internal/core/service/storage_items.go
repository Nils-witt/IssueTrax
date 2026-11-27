package service

import (
	"context"
	"fmt"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// itemRepository is what the shared item use cases need of one item
// kind's repository.
type itemRepository[T any] interface {
	port.StorageItemReader[T]
	Update(ctx context.Context, item T) error
}

// itemKind adapts the shared item use cases to one item kind T.
type itemKind[T any] struct {
	repos Repositories
	opts  options
	repo  itemRepository[T]
	// base returns the shared fields of an item.
	base func(*T) *domain.StorageItem
	// event builds the audit event for an action on an item.
	event func(actor domain.UserID, action domain.AuditAction, item T, changes domain.AuditChanges) auditEvent
}

// get returns the item with id, also when it is deleted.
func (k itemKind[T]) get(ctx context.Context, id domain.StorageItemID) (T, error) {
	item, err := k.repo.Get(ctx, id)
	if err != nil {
		var zero T

		return zero, fmt.Errorf("get storage item %s: %w", id, err)
	}

	return item, nil
}

// listInStorage returns one page of the items in storage. An unknown
// storage is not found.
func (k itemKind[T]) listInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[T], error) {
	if _, err := k.repos.Storages.Get(ctx, storage); err != nil {
		return domain.Page[T]{}, fmt.Errorf("get storage %s: %w", storage, err)
	}

	items, err := k.repo.ListInStorage(ctx, storage, filter, page)
	if err != nil {
		return domain.Page[T]{}, fmt.Errorf("list storage items: %w", err)
	}

	return items, nil
}

// search returns one page of items across all storages.
func (k itemKind[T]) search(ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest) (domain.Page[T], error) {
	items, err := k.repo.Search(ctx, filter, page)
	if err != nil {
		return domain.Page[T]{}, fmt.Errorf("search storage items: %w", err)
	}

	return items, nil
}

// setDeleted applies Delete or Restore to an item, stores and audits it as
// action, and announces the change.
func (k itemKind[T]) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, action domain.AuditAction,
	apply func(*domain.StorageItem, time.Time) error,
) (T, error) {
	var item T

	err := k.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if item, err = k.get(ctx, id); err != nil {
			return err
		}

		if err := apply(k.base(&item), k.opts.now()); err != nil {
			return err
		}

		if err := k.repo.Update(ctx, item); err != nil {
			return fmt.Errorf("update storage item %s: %w", id, err)
		}

		return record(ctx, k.repos, k.opts, k.event(actor, action, item, nil))
	})
	if err != nil {
		var zero T

		return zero, err
	}

	notifyStorageItems(ctx, k.opts)

	return item, nil
}

// notifyStorageItems announces a committed item change of any kind. It
// also changes item counts, which clients load with the storages.
func notifyStorageItems(ctx context.Context, opts options) {
	opts.notify(ctx, domain.Change{Kind: domain.ChangeStorageItems})
}
