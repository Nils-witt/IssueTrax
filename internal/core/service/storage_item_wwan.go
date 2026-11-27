package service

import (
	"context"
	"fmt"
	"strconv"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// checkModuleRefs checks the entries item's modules name where they differ
// from before's (nil for a new item): the hardware manufacturer and model
// and the WWAN SIM card item. modules are the request's modules, to name
// the module in errors.
func checkModuleRefs(
	ctx context.Context, repos Repositories, item domain.StorageItem, before *domain.StorageItem, modules []domain.ModuleParams,
) error {
	if err := checkHardwareRefs(ctx, repos, item, before, hardwareField(modules)); err != nil {
		return err
	}

	return checkSimCardRef(ctx, repos, item, before, wwanField(modules))
}

// checkSimCardRef checks the SIM card item item's WWAN module names, when
// it differs from before's (nil for a new item): it must be another live
// item carrying a SIM card module. Failures are reported on
// field.simCardItemId.
func checkSimCardRef(
	ctx context.Context, repos Repositories, item domain.StorageItem, before *domain.StorageItem, field string,
) error {
	w := domain.WWANOf(item.Modules)
	if w == nil || w.SimCardItemID == nil {
		return nil
	}

	if before != nil {
		if old := domain.WWANOf(before.Modules); old != nil && sameID(old.SimCardItemID, w.SimCardItemID) {
			return nil
		}
	}

	invalid := domain.NewValidationError(map[string]string{field + ".simCardItemId": domain.FieldInvalid})
	if *w.SimCardItemID == item.ID {
		return invalid
	}

	found, err := repos.StorageItems.GetMany(ctx, []domain.StorageItemID{*w.SimCardItemID})
	if err != nil {
		return fmt.Errorf("get sim card item %s: %w", *w.SimCardItemID, err)
	}

	if len(found) == 0 || found[0].IsDeleted() || !hasSimCard(found[0]) {
		return invalid
	}

	return nil
}

func hasSimCard(item domain.StorageItem) bool {
	for _, m := range item.Modules {
		if m.SimCard != nil {
			return true
		}
	}

	return false
}

// resolveSimCardItems loads the SIM card items the WWAN modules of items
// name, with one lookup, and returns a function giving that of one item
// (nil when unset; an unknown one has only the ID set).
func resolveSimCardItems(
	ctx context.Context, repos Repositories, items []domain.StorageItem,
) (func(domain.StorageItem) *domain.StorageItem, error) {
	var ids []domain.StorageItemID

	for _, i := range items {
		if w := domain.WWANOf(i.Modules); w != nil {
			ids = appendID(ids, w.SimCardItemID)
		}
	}

	if len(ids) == 0 {
		return func(domain.StorageItem) *domain.StorageItem { return nil }, nil
	}

	sims, err := repos.StorageItems.GetMany(ctx, unique(ids))
	if err != nil {
		return nil, fmt.Errorf("get sim card items: %w", err)
	}

	byID := indexBy(sims, func(i domain.StorageItem) domain.StorageItemID { return i.ID })

	return func(i domain.StorageItem) *domain.StorageItem {
		w := domain.WWANOf(i.Modules)
		if w == nil {
			return nil
		}

		return lookup(byID, w.SimCardItemID, func(id domain.StorageItemID) domain.StorageItem {
			return domain.StorageItem{ID: id}
		})
	}, nil
}

// wwanField is the request field of the WWAN module among modules, to name
// it in errors.
func wwanField(modules []domain.ModuleParams) string {
	for i, m := range modules {
		if m.Kind == domain.ModuleWWAN {
			return "modules[" + strconv.Itoa(i) + "].wwan"
		}
	}

	return "modules"
}
