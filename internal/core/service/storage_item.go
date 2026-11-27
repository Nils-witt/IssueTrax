package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageItemService satisfies the driving port.
var _ port.StorageItemService = (*StorageItemService)(nil)

// StorageItemService implements port.StorageItemService.
type StorageItemService struct {
	repos Repositories
	opts  options
}

// NewStorageItemService returns a StorageItemService backed by repos.
func NewStorageItemService(repos Repositories, opts ...Option) *StorageItemService {
	return &StorageItemService{repos: repos, opts: newOptions(opts)}
}

// Get returns the item with id, also when it is deleted.
func (s *StorageItemService) Get(ctx context.Context, id domain.StorageItemID) (port.StorageItemView, error) {
	item, err := s.get(ctx, id)
	if err != nil {
		return port.StorageItemView{}, err
	}

	views, err := s.views(ctx, []domain.StorageItem{item})
	if err != nil {
		return port.StorageItemView{}, err
	}

	return views[0], nil
}

// ListInStorage returns one page of the items in storage. An unknown
// storage is not found.
func (s *StorageItemService) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[port.StorageItemView], error) {
	if _, err := s.repos.Storages.Get(ctx, storage); err != nil {
		return domain.Page[port.StorageItemView]{}, fmt.Errorf("get storage %s: %w", storage, err)
	}

	items, err := s.repos.StorageItems.ListInStorage(ctx, storage, filter, page)
	if err != nil {
		return domain.Page[port.StorageItemView]{}, fmt.Errorf("list storage items: %w", err)
	}

	return s.pageViews(ctx, items)
}

// Search returns one page of items across all storages.
func (s *StorageItemService) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[port.StorageItemView], error) {
	items, err := s.repos.StorageItems.Search(ctx, filter, page)
	if err != nil {
		return domain.Page[port.StorageItemView]{}, fmt.Errorf("search storage items: %w", err)
	}

	return s.pageViews(ctx, items)
}

// Create validates and stores a new item. Using the proposed asset tag of
// the type's prefix advances its counter; any other free tag leaves it.
func (s *StorageItemService) Create(
	ctx context.Context, actor domain.UserID, in port.CreateStorageItemInput,
) (port.StorageItemView, error) {
	item, err := domain.NewStorageItem(domain.NewStorageItemParams{
		ID:                    domain.StorageItemID(s.opts.newID()),
		AssetTag:              in.AssetTag,
		Name:                  in.Name,
		Description:           in.Description,
		StorageID:             in.StorageID,
		TypeID:                in.TypeID,
		Usable:                in.Usable,
		OrderDate:             in.OrderDate,
		CommissionedAt:        in.CommissionedAt,
		DecommissionedAt:      in.DecommissionedAt,
		DecommissioningReason: in.DecommissioningReason,
		ResponsibleID:         in.ResponsibleID,
		Modules:               in.Modules,
		NewModuleID:           s.newModuleID,
		Now:                   s.opts.now(),
	})
	if err != nil {
		return port.StorageItemView{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		chain, err := s.checkRefs(ctx, item, nil, in.Modules)
		if err != nil {
			return err
		}

		prefix := domain.EffectiveTagPrefix(chain, s.opts.assetTagPrefix)

		proposed, n, err := proposeAssetTag(ctx, s.repos, prefix)
		if err != nil {
			return err
		}

		if err := s.repos.StorageItems.Create(ctx, item); err != nil {
			return fmt.Errorf("create storage item %q: %w", item.Name, err)
		}

		if strings.EqualFold(item.AssetTag, proposed) {
			if err := s.repos.AssetTagCounters.Set(ctx, prefix, n); err != nil {
				return fmt.Errorf("advance asset tag counter %q: %w", prefix, err)
			}
		}

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, domain.AuditStorageItemCreate, item, itemAudit(domain.StorageItem{}, item, true)))
	})
	if err != nil {
		return port.StorageItemView{}, err
	}

	s.notify(ctx)

	return s.Get(ctx, item.ID)
}

// Update edits, moves and/or retypes an item. Deleted items fail with
// domain.ErrStorageItemDeleted.
func (s *StorageItemService) Update(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, in port.UpdateStorageItemInput,
) (port.StorageItemView, error) {
	changed := false

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		item, err := s.get(ctx, id)
		if err != nil {
			return err
		}

		before := item

		err = item.Apply(domain.StorageItemChanges{
			Name: in.Name, Description: in.Description, StorageID: in.StorageID, TypeID: in.TypeID, Usable: in.Usable,
			OrderDate: in.OrderDate, CommissionedAt: in.CommissionedAt, DecommissionedAt: in.DecommissionedAt,
			DecommissioningReason: in.DecommissioningReason, Responsible: in.Responsible, Modules: in.Modules,
			NewModuleID: s.newModuleID,
		}, s.opts.now())
		if err != nil {
			return err
		}

		var modules []domain.ModuleParams
		if in.Modules != nil {
			modules = *in.Modules
		}

		if _, err := s.checkRefs(ctx, item, &before, modules); err != nil {
			return err
		}

		audit := itemAudit(before, item, false)
		if len(audit) == 0 {
			return nil
		}

		changed = true

		if err := s.repos.StorageItems.Update(ctx, item); err != nil {
			return fmt.Errorf("update storage item %s: %w", item.ID, err)
		}

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, domain.AuditStorageItemUpdate, item, audit))
	})
	if err != nil {
		return port.StorageItemView{}, err
	}

	if changed {
		s.notify(ctx)
	}

	return s.Get(ctx, id)
}

// Delete marks an item deleted. An already deleted item fails with
// domain.ErrStorageItemDeleted.
func (s *StorageItemService) Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error {
	return s.setDeleted(ctx, actor, id, domain.AuditStorageItemDelete, (*domain.StorageItem).Delete)
}

// Restore undoes Delete. An item that is not deleted fails with
// domain.ErrStorageItemNotDeleted.
func (s *StorageItemService) Restore(ctx context.Context, actor domain.UserID, id domain.StorageItemID) (port.StorageItemView, error) {
	if err := s.setDeleted(ctx, actor, id, domain.AuditStorageItemRestore, (*domain.StorageItem).Restore); err != nil {
		return port.StorageItemView{}, err
	}

	return s.Get(ctx, id)
}

func (s *StorageItemService) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.StorageItemID, action domain.AuditAction,
	apply func(*domain.StorageItem, time.Time) error,
) error {
	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		item, err := s.get(ctx, id)
		if err != nil {
			return err
		}

		if err := apply(&item, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.StorageItems.Update(ctx, item); err != nil {
			return fmt.Errorf("update storage item %s: %w", id, err)
		}

		return record(ctx, s.repos, s.opts, storageItemEvent(actor, action, item, nil))
	})
	if err != nil {
		return err
	}

	s.notify(ctx)

	return nil
}

func (s *StorageItemService) get(ctx context.Context, id domain.StorageItemID) (domain.StorageItem, error) {
	item, err := s.repos.StorageItems.Get(ctx, id)
	if err != nil {
		return domain.StorageItem{}, fmt.Errorf("get storage item %s: %w", id, err)
	}

	return item, nil
}

// checkRefs checks the storage, type, responsible person, hardware
// manufacturer and model and WWAN SIM card item of item where they differ from before (all of
// them for a new item, whose before is nil) and returns the chain of
// item's type when it was checked. modules are the request's modules, to
// name the hardware module in errors.
func (s *StorageItemService) checkRefs(
	ctx context.Context, item domain.StorageItem, before *domain.StorageItem, modules []domain.ModuleParams,
) ([]domain.ItemType, error) {
	if err := checkModuleRefs(ctx, s.repos, item, before, modules); err != nil {
		return nil, err
	}

	isNew := before == nil
	if isNew {
		before = &domain.StorageItem{}
	}

	if isNew || item.StorageID != before.StorageID {
		if err := checkItemStorage(ctx, s.repos, item.StorageID); err != nil {
			return nil, err
		}
	}

	var chain []domain.ItemType

	if isNew || item.TypeID != before.TypeID {
		var err error
		if chain, err = liveTypeChain(ctx, s.repos, item.TypeID, domain.ErrStorageItemTypeNotFound); err != nil {
			return nil, err
		}
	}

	if item.ResponsibleID != nil && (before.ResponsibleID == nil || *before.ResponsibleID != *item.ResponsibleID) {
		if err := s.checkResponsible(ctx, *item.ResponsibleID); err != nil {
			return nil, err
		}
	}

	return chain, nil
}

// checkResponsible fails with domain.ErrStorageItemResponsibleInvalid
// unless id is a person (not a service account).
func (s *StorageItemService) checkResponsible(ctx context.Context, id domain.UserID) error {
	user, err := s.repos.Users.Get(ctx, id)

	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return domain.ErrStorageItemResponsibleInvalid
	case err != nil:
		return fmt.Errorf("get user %s: %w", id, err)
	case user.IsServiceAccount():
		return domain.ErrStorageItemResponsibleInvalid
	}

	return nil
}

func (s *StorageItemService) pageViews(
	ctx context.Context, items domain.Page[domain.StorageItem],
) (domain.Page[port.StorageItemView], error) {
	views, err := s.views(ctx, items.Items)
	if err != nil {
		return domain.Page[port.StorageItemView]{}, err
	}

	return domain.Page[port.StorageItemView]{Items: views, Total: items.Total, NextCursor: items.NextCursor}, nil
}

// views resolves the types, responsible people, hardware manufacturers
// and models and WWAN SIM card items of items with one lookup each. Unknown ones are passed with only the ID set.
func (s *StorageItemService) views(ctx context.Context, items []domain.StorageItem) ([]port.StorageItemView, error) {
	typeIDs := make([]domain.ItemTypeID, 0, len(items))
	userIDs := make([]domain.UserID, 0, len(items))

	for _, i := range items {
		typeIDs = append(typeIDs, i.TypeID)
		if i.ResponsibleID != nil {
			userIDs = append(userIDs, *i.ResponsibleID)
		}
	}

	hardware, err := resolveHardware(ctx, s.repos, items)
	if err != nil {
		return nil, err
	}

	simCardItem, err := resolveSimCardItems(ctx, s.repos, items)
	if err != nil {
		return nil, err
	}

	types, err := s.repos.ItemTypes.GetMany(ctx, unique(typeIDs))
	if err != nil {
		return nil, fmt.Errorf("get item types: %w", err)
	}

	users, err := s.repos.Users.GetMany(ctx, unique(userIDs))
	if err != nil {
		return nil, fmt.Errorf("get responsible users: %w", err)
	}

	typeByID := indexBy(types, func(t domain.ItemType) domain.ItemTypeID { return t.ID })
	userByID := indexBy(users, func(u domain.User) domain.UserID { return u.ID })
	views := make([]port.StorageItemView, 0, len(items))

	for _, i := range items {
		v := port.StorageItemView{StorageItem: i, Type: domain.ItemType{ID: i.TypeID}}
		if t, ok := typeByID[i.TypeID]; ok {
			v.Type = t
		}

		if i.ResponsibleID != nil {
			u, ok := userByID[*i.ResponsibleID]
			if !ok {
				u = domain.User{ID: *i.ResponsibleID}
			}

			v.Responsible = &u
		}

		v.Manufacturer, v.Model = hardware(i)
		v.SimCardItem = simCardItem(i)

		views = append(views, v)
	}

	return views, nil
}

func (s *StorageItemService) newModuleID() domain.ModuleID { return domain.ModuleID(s.opts.newID()) }

// notify announces a committed item change. It also changes item counts,
// which clients load with the storages and item types.
func (s *StorageItemService) notify(ctx context.Context) {
	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeStorageItems})
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

// itemAudit lists what changed from before to item; initial records every
// set field of a new item instead.
func itemAudit(before, item domain.StorageItem, initial bool) domain.AuditChanges {
	c := domain.AuditChanges{}
	set, text, list := c.Set, c.Text, c.List

	if initial {
		set = func(field, _, after string) { c.Initial(field, after) }
		text = func(field, _, after string) { c.InitialText(field, after) }
		list = func(field string, _, after []string) { c.InitialList(field, after) }

		set("assetTag", "", item.AssetTag)
	}

	set("name", before.Name, item.Name)
	text("description", before.Description, item.Description)
	set("storage", idString(before.StorageID, initial), item.StorageID.String())
	set("type", idString(before.TypeID, initial), item.TypeID.String())

	if initial || before.Usable != item.Usable {
		set("usable", strconv.FormatBool(before.Usable), strconv.FormatBool(item.Usable))
	}

	set("orderDate", timeString(before.OrderDate), timeString(item.OrderDate))
	set("commissionedAt", timeString(before.CommissionedAt), timeString(item.CommissionedAt))
	set("decommissionedAt", timeString(before.DecommissionedAt), timeString(item.DecommissionedAt))
	text("decommissioningReason", before.DecommissioningReason, item.DecommissioningReason)
	set("personResponsible", userString(before.ResponsibleID), userString(item.ResponsibleID))
	list("modules", moduleStrings(before.Modules), moduleStrings(item.Modules))

	return c
}

// idString is id as text, or empty for the zero ID of a new item.
func idString[T fmt.Stringer](id T, initial bool) string {
	if initial {
		return ""
	}

	return id.String()
}

func timeString(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

func userString(id *domain.UserID) string {
	if id == nil {
		return ""
	}

	return id.String()
}

func moduleStrings(modules []domain.Module) []string {
	out := make([]string, 0, len(modules))
	for _, m := range modules {
		out = append(out, m.String())
	}

	return out
}

func storageItemEvent(actor domain.UserID, action domain.AuditAction, item domain.StorageItem, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetStorageItem, targetID: item.ID.String(), targetLabel: item.AssetTag + " " + item.Name,
	}
}
