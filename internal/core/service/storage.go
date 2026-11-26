package service

import (
	"context"
	"errors"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageService satisfies the driving port.
var _ port.StorageService = (*StorageService)(nil)

// StorageService implements port.StorageService.
type StorageService struct {
	repos Repositories
	opts  options
}

// NewStorageService returns a StorageService backed by repos.
func NewStorageService(repos Repositories, opts ...Option) *StorageService {
	return &StorageService{repos: repos, opts: newOptions(opts)}
}

// Create validates and stores a new storage under in.ParentID (or as a root).
func (s *StorageService) Create(ctx context.Context, actor domain.UserID, in port.CreateStorageInput) (domain.StorageListItem, error) {
	storage, err := domain.NewStorage(domain.NewStorageParams{
		ID:       domain.StorageID(s.opts.newID()),
		ParentID: in.ParentID,
		Name:     in.Name,
		Now:      s.opts.now(),
	})
	if err != nil {
		return domain.StorageListItem{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.checkParent(ctx, storage.ParentID); err != nil {
			return err
		}

		if err := s.repos.Storages.Create(ctx, storage); err != nil {
			return fmt.Errorf("create storage %q: %w", storage.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("name", storage.Name)
		changes.Initial("parent", storage.ParentString())

		return record(ctx, s.repos, s.opts, storageEvent(actor, domain.AuditStorageCreate, storage, changes))
	})
	if err != nil {
		return domain.StorageListItem{}, err
	}

	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeStorages})

	return domain.StorageListItem{Storage: storage}, nil
}

// Get returns the storage with id.
func (s *StorageService) Get(ctx context.Context, id domain.StorageID) (domain.StorageListItem, error) {
	item, err := s.repos.Storages.Get(ctx, id)
	if err != nil {
		return domain.StorageListItem{}, fmt.Errorf("get storage %s: %w", id, err)
	}

	return item, nil
}

// List returns one page of parent's direct children (roots for nil parent),
// ordered by name. An unknown parent is not found.
func (s *StorageService) List(
	ctx context.Context, parent *domain.StorageID, page domain.PageRequest,
) (domain.Page[domain.StorageListItem], error) {
	if parent != nil {
		if _, err := s.Get(ctx, *parent); err != nil {
			return domain.Page[domain.StorageListItem]{}, err
		}
	}

	items, err := s.repos.Storages.ListChildren(ctx, parent, page)
	if err != nil {
		return domain.Page[domain.StorageListItem]{}, fmt.Errorf("list storages: %w", err)
	}

	return items, nil
}

// Update renames and/or moves a storage. Moving it into its own subtree is
// rejected with domain.ErrStorageCycle.
func (s *StorageService) Update(
	ctx context.Context, actor domain.UserID, id domain.StorageID, in port.UpdateStorageInput,
) (domain.StorageListItem, error) {
	var (
		item    domain.StorageListItem
		changed bool
	)

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if in.Parent != nil {
			// Taken before reading, so the subtree checked below cannot
			// change until this move is committed.
			if err := s.repos.Storages.LockTree(ctx); err != nil {
				return fmt.Errorf("lock storage tree: %w", err)
			}
		}

		current, err := s.Get(ctx, id)
		if err != nil {
			return err
		}

		storage := current.Storage
		before := storage

		changes := domain.StorageChanges{Name: in.Name, Parent: in.Parent}
		if err := storage.Apply(changes, s.opts.now()); err != nil {
			return err
		}

		if in.Parent != nil && in.Parent.ID != nil && before.ParentString() != storage.ParentString() {
			if err := s.checkMove(ctx, storage.ID, *in.Parent.ID); err != nil {
				return err
			}
		}

		audit := domain.AuditChanges{}
		audit.Set("name", before.Name, storage.Name)
		audit.Set("parent", before.ParentString(), storage.ParentString())

		item = domain.StorageListItem{Storage: storage, ChildCount: current.ChildCount}

		if len(audit) == 0 {
			return nil
		}

		changed = true

		if err := s.repos.Storages.Update(ctx, storage); err != nil {
			return fmt.Errorf("update storage %s: %w", storage.ID, err)
		}

		return record(ctx, s.repos, s.opts, storageEvent(actor, domain.AuditStorageUpdate, storage, audit))
	})
	if err != nil {
		return domain.StorageListItem{}, err
	}

	if changed {
		s.opts.notify(ctx, domain.Change{Kind: domain.ChangeStorages})
	}

	return item, nil
}

// checkParent fails with domain.ErrStorageParentNotFound unless parent is
// nil or an existing storage.
func (s *StorageService) checkParent(ctx context.Context, parent *domain.StorageID) error {
	if parent == nil {
		return nil
	}

	if _, err := s.repos.Storages.Get(ctx, *parent); err != nil {
		if errors.Is(err, domain.ErrStorageNotFound) {
			return domain.ErrStorageParentNotFound
		}

		return fmt.Errorf("get parent storage %s: %w", parent, err)
	}

	return nil
}

// checkMove fails unless id may move under the existing storage parent.
func (s *StorageService) checkMove(ctx context.Context, id, parent domain.StorageID) error {
	if err := s.checkParent(ctx, &parent); err != nil {
		return err
	}

	inSubtree, err := s.repos.Storages.IsDescendant(ctx, id, parent)
	if err != nil {
		return fmt.Errorf("check storage subtree: %w", err)
	}

	if inSubtree {
		return domain.ErrStorageCycle
	}

	return nil
}

func storageEvent(actor domain.UserID, action domain.AuditAction, st domain.Storage, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetStorage, targetID: st.ID.String(), targetLabel: st.Name,
	}
}
