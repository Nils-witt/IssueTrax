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

// Compile-time checks that the services satisfy the driving ports.
var (
	_ port.ManufacturerService  = (*ManufacturerService)(nil)
	_ port.HardwareModelService = (*HardwareModelService)(nil)
)

// ManufacturerService implements port.ManufacturerService.
type ManufacturerService struct {
	repos Repositories
	opts  options
}

// NewManufacturerService returns a ManufacturerService backed by repos.
func NewManufacturerService(repos Repositories, opts ...Option) *ManufacturerService {
	return &ManufacturerService{repos: repos, opts: newOptions(opts)}
}

// Get returns the manufacturer with id, also when it is deleted.
func (s *ManufacturerService) Get(ctx context.Context, id domain.ManufacturerID) (domain.ManufacturerListItem, error) {
	m, err := s.repos.Manufacturers.Get(ctx, id)
	if err != nil {
		return domain.ManufacturerListItem{}, fmt.Errorf("get manufacturer %s: %w", id, err)
	}

	return m, nil
}

// List returns one page of manufacturers.
func (s *ManufacturerService) List(
	ctx context.Context, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.ManufacturerListItem], error) {
	found, err := s.repos.Manufacturers.List(ctx, filter, page)
	if err != nil {
		return domain.Page[domain.ManufacturerListItem]{}, fmt.Errorf("list manufacturers: %w", err)
	}

	return found, nil
}

// Create validates and stores a new manufacturer.
func (s *ManufacturerService) Create(ctx context.Context, actor domain.UserID, name string) (domain.ManufacturerListItem, error) {
	m, err := domain.NewManufacturer(domain.ManufacturerID(s.opts.newID()), name, s.opts.now())
	if err != nil {
		return domain.ManufacturerListItem{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repos.Manufacturers.Create(ctx, m); err != nil {
			return fmt.Errorf("create manufacturer %q: %w", m.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("name", m.Name)

		return record(ctx, s.repos, s.opts, manufacturerEvent(actor, domain.AuditManufacturerCreate, m, changes))
	})
	if err != nil {
		return domain.ManufacturerListItem{}, err
	}

	notifyCatalog(ctx, s.opts)

	return s.Get(ctx, m.ID)
}

// Rename changes the name of a live manufacturer.
func (s *ManufacturerService) Rename(
	ctx context.Context, actor domain.UserID, id domain.ManufacturerID, name string,
) (domain.ManufacturerListItem, error) {
	err := s.change(ctx, actor, id, domain.AuditManufacturerUpdate, func(m *domain.Manufacturer) (domain.AuditChanges, error) {
		before := m.Name
		if err := m.Rename(name, s.opts.now()); err != nil {
			return nil, err
		}

		changes := domain.AuditChanges{}
		if changes.Set("name", before, m.Name); len(changes) == 0 {
			return nil, nil
		}

		return changes, nil
	})
	if err != nil {
		return domain.ManufacturerListItem{}, err
	}

	return s.Get(ctx, id)
}

// Delete marks a manufacturer deleted; its models and items keep it.
func (s *ManufacturerService) Delete(ctx context.Context, actor domain.UserID, id domain.ManufacturerID) error {
	return s.setDeleted(ctx, actor, id, domain.AuditManufacturerDelete, (*domain.Manufacturer).Delete)
}

// Restore undoes Delete.
func (s *ManufacturerService) Restore(
	ctx context.Context, actor domain.UserID, id domain.ManufacturerID,
) (domain.ManufacturerListItem, error) {
	if err := s.setDeleted(ctx, actor, id, domain.AuditManufacturerRestore, (*domain.Manufacturer).Restore); err != nil {
		return domain.ManufacturerListItem{}, err
	}

	return s.Get(ctx, id)
}

func (s *ManufacturerService) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.ManufacturerID, action domain.AuditAction,
	apply func(*domain.Manufacturer, time.Time) error,
) error {
	return s.change(ctx, actor, id, action, func(m *domain.Manufacturer) (domain.AuditChanges, error) {
		return domain.AuditChanges{}, apply(m, s.opts.now())
	})
}

// change applies edit to the manufacturer id; see changeEntry.
//
//nolint:dupl // HardwareModelService.change is the same glue for models
func (s *ManufacturerService) change(
	ctx context.Context, actor domain.UserID, id domain.ManufacturerID, action domain.AuditAction,
	edit func(*domain.Manufacturer) (domain.AuditChanges, error),
) error {
	return changeEntry(ctx, s.repos, s.opts, catalogEntry[domain.Manufacturer]{
		what: "manufacturer " + id.String(),
		load: func(ctx context.Context) (domain.Manufacturer, error) {
			m, err := s.repos.Manufacturers.Get(ctx, id)
			return m.Manufacturer, err
		},
		store: s.repos.Manufacturers.Update,
		event: func(m domain.Manufacturer, c domain.AuditChanges) auditEvent {
			return manufacturerEvent(actor, action, m, c)
		},
	}, edit)
}

// HardwareModelService implements port.HardwareModelService.
type HardwareModelService struct {
	repos Repositories
	opts  options
}

// NewHardwareModelService returns a HardwareModelService backed by repos.
func NewHardwareModelService(repos Repositories, opts ...Option) *HardwareModelService {
	return &HardwareModelService{repos: repos, opts: newOptions(opts)}
}

// Get returns the model with id, also when it is deleted.
func (s *HardwareModelService) Get(ctx context.Context, id domain.HardwareModelID) (domain.HardwareModelListItem, error) {
	m, err := s.repos.HardwareModels.Get(ctx, id)
	if err != nil {
		return domain.HardwareModelListItem{}, fmt.Errorf("get hardware model %s: %w", id, err)
	}

	return m, nil
}

// List returns one page of the manufacturer's models. An unknown
// manufacturer is not found.
func (s *HardwareModelService) List(
	ctx context.Context, manufacturer domain.ManufacturerID, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.HardwareModelListItem], error) {
	if _, err := s.repos.Manufacturers.Get(ctx, manufacturer); err != nil {
		return domain.Page[domain.HardwareModelListItem]{}, fmt.Errorf("get manufacturer %s: %w", manufacturer, err)
	}

	found, err := s.repos.HardwareModels.List(ctx, manufacturer, filter, page)
	if err != nil {
		return domain.Page[domain.HardwareModelListItem]{}, fmt.Errorf("list hardware models: %w", err)
	}

	return found, nil
}

// Create validates and stores a new model of a live manufacturer. A
// deleted manufacturer fails with domain.ErrManufacturerDeleted.
func (s *HardwareModelService) Create(
	ctx context.Context, actor domain.UserID, manufacturer domain.ManufacturerID, name string,
) (domain.HardwareModelListItem, error) {
	m, err := domain.NewHardwareModel(domain.HardwareModelID(s.opts.newID()), manufacturer, name, s.opts.now())
	if err != nil {
		return domain.HardwareModelListItem{}, err
	}

	err = s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		maker, err := s.repos.Manufacturers.Get(ctx, manufacturer)
		if err != nil {
			return fmt.Errorf("get manufacturer %s: %w", manufacturer, err)
		}

		if maker.IsDeleted() {
			return domain.ErrManufacturerDeleted
		}

		if err := s.repos.HardwareModels.Create(ctx, m); err != nil {
			return fmt.Errorf("create hardware model %q: %w", m.Name, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("manufacturer", manufacturer.String())
		changes.Initial("name", m.Name)

		return record(ctx, s.repos, s.opts, hardwareModelEvent(actor, domain.AuditHardwareModelCreate, m, changes))
	})
	if err != nil {
		return domain.HardwareModelListItem{}, err
	}

	notifyCatalog(ctx, s.opts)

	return s.Get(ctx, m.ID)
}

// Rename changes the name of a live model.
func (s *HardwareModelService) Rename(
	ctx context.Context, actor domain.UserID, id domain.HardwareModelID, name string,
) (domain.HardwareModelListItem, error) {
	err := s.change(ctx, actor, id, domain.AuditHardwareModelUpdate, func(m *domain.HardwareModel) (domain.AuditChanges, error) {
		before := m.Name
		if err := m.Rename(name, s.opts.now()); err != nil {
			return nil, err
		}

		changes := domain.AuditChanges{}
		if changes.Set("name", before, m.Name); len(changes) == 0 {
			return nil, nil
		}

		return changes, nil
	})
	if err != nil {
		return domain.HardwareModelListItem{}, err
	}

	return s.Get(ctx, id)
}

// Delete marks a model deleted; items keep it.
func (s *HardwareModelService) Delete(ctx context.Context, actor domain.UserID, id domain.HardwareModelID) error {
	return s.setDeleted(ctx, actor, id, domain.AuditHardwareModelDelete, (*domain.HardwareModel).Delete)
}

// Restore undoes Delete.
func (s *HardwareModelService) Restore(
	ctx context.Context, actor domain.UserID, id domain.HardwareModelID,
) (domain.HardwareModelListItem, error) {
	if err := s.setDeleted(ctx, actor, id, domain.AuditHardwareModelRestore, (*domain.HardwareModel).Restore); err != nil {
		return domain.HardwareModelListItem{}, err
	}

	return s.Get(ctx, id)
}

func (s *HardwareModelService) setDeleted(
	ctx context.Context, actor domain.UserID, id domain.HardwareModelID, action domain.AuditAction,
	apply func(*domain.HardwareModel, time.Time) error,
) error {
	return s.change(ctx, actor, id, action, func(m *domain.HardwareModel) (domain.AuditChanges, error) {
		return domain.AuditChanges{}, apply(m, s.opts.now())
	})
}

// change applies edit to the model id; see changeEntry.
//
//nolint:dupl // ManufacturerService.change is the same glue for manufacturers
func (s *HardwareModelService) change(
	ctx context.Context, actor domain.UserID, id domain.HardwareModelID, action domain.AuditAction,
	edit func(*domain.HardwareModel) (domain.AuditChanges, error),
) error {
	return changeEntry(ctx, s.repos, s.opts, catalogEntry[domain.HardwareModel]{
		what: "hardware model " + id.String(),
		load: func(ctx context.Context) (domain.HardwareModel, error) {
			m, err := s.repos.HardwareModels.Get(ctx, id)
			return m.HardwareModel, err
		},
		store: s.repos.HardwareModels.Update,
		event: func(m domain.HardwareModel, c domain.AuditChanges) auditEvent {
			return hardwareModelEvent(actor, action, m, c)
		},
	}, edit)
}

// catalogEntry loads, stores and audits one manufacturer or model.
type catalogEntry[T any] struct {
	// what names the entry in errors, e.g. "manufacturer <id>".
	what  string
	load  func(context.Context) (T, error)
	store func(context.Context, T) error
	event func(T, domain.AuditChanges) auditEvent
}

// changeEntry loads an entry, lets edit change it and stores and audits
// the result in one transaction, then announces it. edit returns the
// audited field changes; nil means nothing changed, so nothing is stored
// or announced, while an empty map records the action without fields.
func changeEntry[T any](
	ctx context.Context, repos Repositories, opts options, e catalogEntry[T], edit func(*T) (domain.AuditChanges, error),
) error {
	changed := false

	err := repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		entry, err := e.load(ctx)
		if err != nil {
			return fmt.Errorf("get %s: %w", e.what, err)
		}

		changes, err := edit(&entry)
		if err != nil {
			return err
		}

		if changes == nil {
			return nil
		}

		if len(changes) == 0 {
			changes = nil
		}

		changed = true

		if err := e.store(ctx, entry); err != nil {
			return fmt.Errorf("update %s: %w", e.what, err)
		}

		return record(ctx, repos, opts, e.event(entry, changes))
	})
	if err != nil {
		return err
	}

	if changed {
		notifyCatalog(ctx, opts)
	}

	return nil
}

// notifyCatalog announces a committed manufacturer or model change.
func notifyCatalog(ctx context.Context, opts options) {
	opts.notify(ctx, domain.Change{Kind: domain.ChangeManufacturers})
}

func manufacturerEvent(actor domain.UserID, action domain.AuditAction, m domain.Manufacturer, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetManufacturer, targetID: m.ID.String(), targetLabel: m.Name,
	}
}

func hardwareModelEvent(actor domain.UserID, action domain.AuditAction, m domain.HardwareModel, changes domain.AuditChanges) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetHardwareModel, targetID: m.ID.String(), targetLabel: m.Name,
	}
}

// checkHardwareRefs checks the manufacturer and model item's hardware
// module names where they differ from before's (nil for a new item). They
// must exist and the model must belong to the manufacturer; newly picked
// entries must also be live, while kept ones may have been deleted since.
// Failures are reported on field (e.g. "modules[0].hardware").
func checkHardwareRefs(ctx context.Context, repos Repositories, item domain.StorageItem, before *domain.StorageItem, field string) error {
	hw := domain.HardwareOf(item.Modules)
	if hw == nil {
		return nil
	}

	var old domain.Hardware

	if before != nil {
		if h := domain.HardwareOf(before.Modules); h != nil {
			old = *h
		}
	}

	if sameID(hw.ManufacturerID, old.ManufacturerID) && sameID(hw.ModelID, old.ModelID) {
		return nil
	}

	if hw.ManufacturerID != nil {
		if err := checkManufacturerRef(ctx, repos, *hw.ManufacturerID, sameID(hw.ManufacturerID, old.ManufacturerID), field); err != nil {
			return err
		}
	}

	if hw.ModelID != nil {
		return checkModelRef(ctx, repos, *hw, sameID(hw.ModelID, old.ModelID), field)
	}

	return nil
}

// checkManufacturerRef fails unless id exists and, unless kept, is live.
func checkManufacturerRef(ctx context.Context, repos Repositories, id domain.ManufacturerID, kept bool, field string) error {
	m, err := repos.Manufacturers.Get(ctx, id)

	switch {
	case errors.Is(err, domain.ErrManufacturerNotFound):
		return domain.NewValidationError(map[string]string{field + ".manufacturerId": domain.FieldInvalid})
	case err != nil:
		return fmt.Errorf("get manufacturer %s: %w", id, err)
	case m.IsDeleted() && !kept:
		return domain.ErrManufacturerDeleted
	}

	return nil
}

// checkModelRef fails unless hw's model exists, belongs to hw's
// manufacturer and, unless kept, is live.
func checkModelRef(ctx context.Context, repos Repositories, hw domain.Hardware, kept bool, field string) error {
	m, err := repos.HardwareModels.Get(ctx, *hw.ModelID)

	switch {
	case errors.Is(err, domain.ErrHardwareModelNotFound):
		return domain.NewValidationError(map[string]string{field + ".modelId": domain.FieldInvalid})
	case err != nil:
		return fmt.Errorf("get hardware model %s: %w", hw.ModelID, err)
	case hw.ManufacturerID == nil || m.ManufacturerID != *hw.ManufacturerID:
		return domain.NewValidationError(map[string]string{field + ".modelId": domain.FieldInvalid})
	case m.IsDeleted() && !kept:
		return domain.ErrHardwareModelDeleted
	}

	return nil
}

// resolveHardware loads the manufacturers and models the hardware modules
// of items name, with one lookup each, and returns a function giving those
// of one item (nil when unset; unknown ones have only the ID set).
func resolveHardware(
	ctx context.Context, repos Repositories, items []domain.StorageItem,
) (func(domain.StorageItem) (*domain.Manufacturer, *domain.HardwareModel), error) {
	var (
		makerIDs []domain.ManufacturerID
		modelIDs []domain.HardwareModelID
	)

	for _, i := range items {
		if hw := domain.HardwareOf(i.Modules); hw != nil {
			makerIDs = appendID(makerIDs, hw.ManufacturerID)
			modelIDs = appendID(modelIDs, hw.ModelID)
		}
	}

	makers, err := repos.Manufacturers.GetMany(ctx, unique(makerIDs))
	if err != nil {
		return nil, fmt.Errorf("get manufacturers: %w", err)
	}

	models, err := repos.HardwareModels.GetMany(ctx, unique(modelIDs))
	if err != nil {
		return nil, fmt.Errorf("get hardware models: %w", err)
	}

	makerByID := indexBy(makers, func(m domain.Manufacturer) domain.ManufacturerID { return m.ID })
	modelByID := indexBy(models, func(m domain.HardwareModel) domain.HardwareModelID { return m.ID })

	return func(i domain.StorageItem) (*domain.Manufacturer, *domain.HardwareModel) {
		hw := domain.HardwareOf(i.Modules)
		if hw == nil {
			return nil, nil
		}

		return lookup(makerByID, hw.ManufacturerID, func(id domain.ManufacturerID) domain.Manufacturer {
				return domain.Manufacturer{ID: id}
			}),
			lookup(modelByID, hw.ModelID, func(id domain.HardwareModelID) domain.HardwareModel {
				return domain.HardwareModel{ID: id}
			})
	}, nil
}

func appendID[T any](ids []T, id *T) []T {
	if id == nil {
		return ids
	}

	return append(ids, *id)
}

// lookup returns the value of id in m, a placeholder for an unknown id,
// or nil for a nil id.
func lookup[K comparable, V any](m map[K]V, id *K, placeholder func(K) V) *V {
	if id == nil {
		return nil
	}

	v, ok := m[*id]
	if !ok {
		v = placeholder(*id)
	}

	return &v
}

// hardwareField names the hardware module among the request's modules for
// errors, e.g. "modules[0].hardware".
func hardwareField(modules []domain.ModuleParams) string {
	for i, m := range modules {
		if m.Kind == domain.ModuleHardware {
			return "modules[" + strconv.Itoa(i) + "].hardware"
		}
	}

	return "modules"
}

func sameID[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
