package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that ItemTypeService satisfies the driving port.
var _ port.ItemTypeService = (*ItemTypeService)(nil)

// ItemTypeService implements port.ItemTypeService.
type ItemTypeService struct {
	repos Repositories
	opts  options
}

// NewItemTypeService returns an ItemTypeService backed by repos.
func NewItemTypeService(repos Repositories, opts ...Option) *ItemTypeService {
	return &ItemTypeService{repos: repos, opts: newOptions(opts)}
}

// Get returns the type with id, also when it is deleted, with what it
// inherits.
func (s *ItemTypeService) Get(ctx context.Context, id domain.ItemTypeID) (port.ItemTypeView, error) {
	item, err := s.repos.ItemTypes.Get(ctx, id)
	if err != nil {
		return port.ItemTypeView{}, fmt.Errorf("get item type %s: %w", id, err)
	}

	return s.view(ctx, item)
}

// ListChildren returns one page of parent's children (roots for nil
// parent). An unknown parent is not found.
func (s *ItemTypeService) ListChildren(
	ctx context.Context, parent *domain.ItemTypeID, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	if parent != nil {
		if _, err := s.repos.ItemTypes.Get(ctx, *parent); err != nil {
			return domain.Page[domain.ItemTypeListItem]{}, fmt.Errorf("get item type %s: %w", parent, err)
		}
	}

	types, err := s.repos.ItemTypes.ListChildren(ctx, parent, filter, page)
	if err != nil {
		return domain.Page[domain.ItemTypeListItem]{}, fmt.Errorf("list item types: %w", err)
	}

	return types, nil
}

// Search returns one page of the types whose name contains query.
func (s *ItemTypeService) Search(
	ctx context.Context, query string, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[port.ItemTypeView], error) {
	found, err := s.repos.ItemTypes.Search(ctx, query, filter, page)
	if err != nil {
		return domain.Page[port.ItemTypeView]{}, fmt.Errorf("search item types: %w", err)
	}

	views := make([]port.ItemTypeView, 0, len(found.Items))

	for _, t := range found.Items {
		v, err := s.view(ctx, t)
		if err != nil {
			return domain.Page[port.ItemTypeView]{}, err
		}

		views = append(views, v)
	}

	return domain.Page[port.ItemTypeView]{Items: views, Total: found.Total, NextCursor: found.NextCursor}, nil
}

// NextAssetTag proposes the asset tag for a new item of the type. Deleted
// types fail with domain.ErrItemTypeDeleted.
func (s *ItemTypeService) NextAssetTag(ctx context.Context, id domain.ItemTypeID) (string, error) {
	chain, err := liveTypeChain(ctx, s.repos, id, domain.ErrItemTypeNotFound)
	if err != nil {
		return "", err
	}

	tag, _, err := proposeAssetTag(ctx, s.repos, domain.EffectiveTagPrefix(chain, s.opts.assetTagPrefix))

	return tag, err
}

// Create validates and stores a new type. Only admins may create types.
func (s *ItemTypeService) Create(ctx context.Context, actor domain.UserID, in port.CreateItemTypeInput) (port.ItemTypeView, error) {
	if err := requireAdmin(ctx, s.repos, actor); err != nil {
		return port.ItemTypeView{}, err
	}

	t, err := domain.NewItemType(domain.NewItemTypeParams{
		ID:        domain.ItemTypeID(s.opts.newID()),
		ParentID:  in.ParentID,
		Name:      in.Name,
		TagPrefix: in.TagPrefix,
		Defaults:  in.Defaults,
		Now:       s.opts.now(),
	})
	if err != nil {
		return port.ItemTypeView{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.checkParent(ctx, t.ParentID); err != nil {
			return err
		}

		if err := s.repos.ItemTypes.Create(ctx, t); err != nil {
			return fmt.Errorf("create item type %q: %w", t.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("name", t.Name)
		changes.Initial("parent", typeParentString(t))
		changes.Initial("tagPrefix", t.TagPrefix)
		changes.InitialList("defaults", defaultStrings(t.Defaults))

		return record(ctx, s.repos, s.opts, itemTypeEvent(actor, domain.AuditItemTypeCreate, t, changes))
	})
	if err != nil {
		return port.ItemTypeView{}, err
	}

	s.notify(ctx)

	return s.Get(ctx, t.ID)
}

// Update renames, moves, re-prefixes and/or changes the defaults of a type.
// Moving it into its own subtree fails with domain.ErrItemTypeCycle.
func (s *ItemTypeService) Update(
	ctx context.Context, actor domain.UserID, id domain.ItemTypeID, in port.UpdateItemTypeInput,
) (port.ItemTypeView, error) {
	if err := requireAdmin(ctx, s.repos, actor); err != nil {
		return port.ItemTypeView{}, err
	}

	changed := false

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if in.Parent != nil {
			// Taken before reading, so the subtree checked below cannot
			// change until this move is committed.
			if err := s.repos.ItemTypes.LockTree(ctx); err != nil {
				return fmt.Errorf("lock item type tree: %w", err)
			}
		}

		current, err := s.repos.ItemTypes.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get item type %s: %w", id, err)
		}

		t := current.ItemType
		before := t

		err = t.Apply(domain.ItemTypeChanges{Name: in.Name, Parent: in.Parent, TagPrefix: in.TagPrefix, Defaults: in.Defaults}, s.opts.now())
		if err != nil {
			return err
		}

		if typeParentString(before) != typeParentString(t) && t.ParentID != nil {
			if err := s.checkMove(ctx, t.ID, *t.ParentID); err != nil {
				return err
			}
		}

		audit := domain.AuditChanges{}
		audit.Set("name", before.Name, t.Name)
		audit.Set("parent", typeParentString(before), typeParentString(t))
		audit.Set("tagPrefix", before.TagPrefix, t.TagPrefix)
		audit.List("defaults", defaultStrings(before.Defaults), defaultStrings(t.Defaults))

		if len(audit) == 0 {
			return nil
		}

		changed = true

		if err := s.repos.ItemTypes.Update(ctx, t); err != nil {
			return fmt.Errorf("update item type %s: %w", t.ID, err)
		}

		return record(ctx, s.repos, s.opts, itemTypeEvent(actor, domain.AuditItemTypeUpdate, t, audit))
	})
	if err != nil {
		return port.ItemTypeView{}, err
	}

	if changed {
		s.notify(ctx)
	}

	return s.Get(ctx, id)
}

// Delete marks a type deleted; its children and items keep it.
func (s *ItemTypeService) Delete(ctx context.Context, actor domain.UserID, id domain.ItemTypeID) error {
	return s.setDeleted(ctx, actor, id, domain.AuditItemTypeDelete, (*domain.ItemType).Delete)
}

// Restore undoes Delete. It fails with domain.ErrItemTypeNameTaken when a
// live sibling has taken the name meanwhile.
func (s *ItemTypeService) Restore(ctx context.Context, actor domain.UserID, id domain.ItemTypeID) (port.ItemTypeView, error) {
	if err := s.setDeleted(ctx, actor, id, domain.AuditItemTypeRestore, (*domain.ItemType).Restore); err != nil {
		return port.ItemTypeView{}, err
	}

	return s.Get(ctx, id)
}

func (s *ItemTypeService) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.ItemTypeID, action domain.AuditAction,
	apply func(*domain.ItemType, time.Time) error,
) error {
	if err := requireAdmin(ctx, s.repos, actor); err != nil {
		return err
	}

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.repos.ItemTypes.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get item type %s: %w", id, err)
		}

		t := current.ItemType
		if err := apply(&t, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.ItemTypes.Update(ctx, t); err != nil {
			return fmt.Errorf("update item type %s: %w", id, err)
		}

		return record(ctx, s.repos, s.opts, itemTypeEvent(actor, action, t, nil))
	})
	if err != nil {
		return err
	}

	s.notify(ctx)

	return nil
}

// view adds the inherited data to a type.
func (s *ItemTypeService) view(ctx context.Context, t domain.ItemTypeListItem) (port.ItemTypeView, error) {
	chain, err := s.repos.ItemTypes.Chain(ctx, t.ID)
	if err != nil {
		return port.ItemTypeView{}, fmt.Errorf("get item type chain %s: %w", t.ID, err)
	}

	return port.ItemTypeView{
		ItemTypeListItem:   t,
		Path:               chain[:len(chain)-1],
		EffectiveDefaults:  domain.EffectiveDefaults(chain),
		EffectiveTagPrefix: domain.EffectiveTagPrefix(chain, s.opts.assetTagPrefix),
	}, nil
}

// checkParent fails unless parent is nil or an existing live type.
func (s *ItemTypeService) checkParent(ctx context.Context, parent *domain.ItemTypeID) error {
	if parent == nil {
		return nil
	}

	p, err := s.repos.ItemTypes.Get(ctx, *parent)

	switch {
	case errors.Is(err, domain.ErrItemTypeNotFound):
		return domain.ErrItemTypeParentNotFound
	case err != nil:
		return fmt.Errorf("get parent item type %s: %w", parent, err)
	case p.IsDeleted():
		return domain.ErrItemTypeDeleted
	}

	return nil
}

// checkMove fails unless id may move under the live type parent.
func (s *ItemTypeService) checkMove(ctx context.Context, id, parent domain.ItemTypeID) error {
	if err := s.checkParent(ctx, &parent); err != nil {
		return err
	}

	inSubtree, err := s.repos.ItemTypes.IsDescendant(ctx, id, parent)
	if err != nil {
		return fmt.Errorf("check item type subtree: %w", err)
	}

	if inSubtree {
		return domain.ErrItemTypeCycle
	}

	return nil
}

func (s *ItemTypeService) notify(ctx context.Context) {
	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeItemTypes})
}

// liveTypeChain returns the chain of the live type id; a missing type
// fails with notFound, a deleted one with domain.ErrItemTypeDeleted.
func liveTypeChain(ctx context.Context, repos Repositories, id domain.ItemTypeID, notFound error) ([]domain.ItemType, error) {
	chain, err := repos.ItemTypes.Chain(ctx, id)

	switch {
	case errors.Is(err, domain.ErrItemTypeNotFound):
		return nil, notFound
	case err != nil:
		return nil, fmt.Errorf("get item type chain %s: %w", id, err)
	case chain[len(chain)-1].IsDeleted():
		return nil, domain.ErrItemTypeDeleted
	}

	return chain, nil
}

// proposeAssetTag returns the next free asset tag with prefix and its
// number: the number after the prefix's counter, skipping tags in use.
func proposeAssetTag(ctx context.Context, repos Repositories, prefix string) (string, int64, error) {
	n, err := repos.AssetTagCounters.Get(ctx, prefix)
	if err != nil {
		return "", 0, fmt.Errorf("get asset tag counter %q: %w", prefix, err)
	}

	for {
		n++
		tag := domain.FormatAssetTag(prefix, n)

		taken, err := repos.StorageItems.AssetTagTaken(ctx, tag)
		if err != nil {
			return "", 0, fmt.Errorf("check asset tag %q: %w", tag, err)
		}

		if !taken {
			return tag, n, nil
		}
	}
}

func typeParentString(t domain.ItemType) string {
	if t.ParentID == nil {
		return ""
	}

	return t.ParentID.String()
}

// defaultStrings renders defaults as "module×count".
func defaultStrings(defaults []domain.ModuleDefault) []string {
	out := make([]string, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, string(d.Module)+"×"+strconv.Itoa(d.Count))
	}

	return out
}

func itemTypeEvent(actor domain.UserID, action domain.AuditAction, t domain.ItemType, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetItemType, targetID: t.ID.String(), targetLabel: t.Name,
	}
}
