package memory

import (
	"context"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time checks that the repositories satisfy the driven ports.
var (
	_ port.ManufacturerRepository  = (*ManufacturerRepository)(nil)
	_ port.HardwareModelRepository = (*HardwareModelRepository)(nil)
)

// ManufacturerRepository implements port.ManufacturerRepository on a Store.
type ManufacturerRepository struct{ s *Store }

// NewManufacturerRepository returns a ManufacturerRepository backed by s.
func NewManufacturerRepository(s *Store) *ManufacturerRepository {
	return &ManufacturerRepository{s: s}
}

// Create inserts m, rejecting a live manufacturer with the same name.
func (r *ManufacturerRepository) Create(_ context.Context, m domain.Manufacturer) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if r.nameTaken(m) {
		return domain.ErrManufacturerNameTaken
	}

	r.s.data.manufacturers[m.ID] = cloneManufacturer(m)

	return nil
}

// Update replaces an existing manufacturer, rejecting a live one with the
// same name while m is live.
func (r *ManufacturerRepository) Update(_ context.Context, m domain.Manufacturer) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.manufacturers[m.ID]
	if !ok {
		return domain.ErrManufacturerNotFound
	}

	if r.nameTaken(m) {
		return domain.ErrManufacturerNameTaken
	}

	m.CreatedAt = current.CreatedAt
	r.s.data.manufacturers[m.ID] = cloneManufacturer(m)

	return nil
}

// Get returns the manufacturer with id and its counts.
func (r *ManufacturerRepository) Get(_ context.Context, id domain.ManufacturerID) (domain.ManufacturerListItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	m, ok := r.s.data.manufacturers[id]
	if !ok {
		return domain.ManufacturerListItem{}, domain.ErrManufacturerNotFound
	}

	return r.listItem(m), nil
}

// GetMany returns the known manufacturers among ids.
func (r *ManufacturerRepository) GetMany(_ context.Context, ids []domain.ManufacturerID) ([]domain.Manufacturer, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	out := make([]domain.Manufacturer, 0, len(ids))

	for _, id := range ids {
		if m, ok := r.s.data.manufacturers[id]; ok {
			out = append(out, cloneManufacturer(m))
		}
	}

	return out, nil
}

// List returns one page of the manufacturers filter keeps, ordered by
// lower(name), then ID.
func (r *ManufacturerRepository) List(
	_ context.Context, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.ManufacturerListItem], error) {
	query := strings.ToLower(filter.Query)

	r.s.mu.RLock()

	var all []domain.ManufacturerListItem

	for _, m := range r.s.data.manufacturers {
		if strings.Contains(strings.ToLower(m.Name), query) && (filter.IncludeDeleted || !m.IsDeleted()) {
			all = append(all, r.listItem(m))
		}
	}

	r.s.mu.RUnlock()

	return pageByName(all, func(m domain.ManufacturerListItem) (string, string) { return m.Name, m.ID.String() }, page)
}

// nameTaken reports whether another live manufacturer has m's name, while
// m is live. Callers hold r.s.mu.
func (r *ManufacturerRepository) nameTaken(m domain.Manufacturer) bool {
	if m.IsDeleted() {
		return false
	}

	for _, o := range r.s.data.manufacturers {
		if o.ID != m.ID && !o.IsDeleted() && sameName(o.Name, m.Name) {
			return true
		}
	}

	return false
}

// listItem adds the counts to m. Callers hold r.s.mu.
func (r *ManufacturerRepository) listItem(m domain.Manufacturer) domain.ManufacturerListItem {
	item := domain.ManufacturerListItem{Manufacturer: cloneManufacturer(m)}

	for _, hm := range r.s.data.hardwareModels {
		if hm.ManufacturerID == m.ID && !hm.IsDeleted() {
			item.ModelCount++
		}
	}

	item.ItemCount = r.s.countHardware(func(h domain.Hardware) bool { return h.ManufacturerID != nil && *h.ManufacturerID == m.ID })

	return item
}

// HardwareModelRepository implements port.HardwareModelRepository on a
// Store.
type HardwareModelRepository struct{ s *Store }

// NewHardwareModelRepository returns a HardwareModelRepository backed by s.
func NewHardwareModelRepository(s *Store) *HardwareModelRepository {
	return &HardwareModelRepository{s: s}
}

// Create inserts m, rejecting a live model of the manufacturer with the
// same name.
func (r *HardwareModelRepository) Create(_ context.Context, m domain.HardwareModel) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.manufacturers[m.ManufacturerID]; !ok {
		return domain.ErrManufacturerNotFound
	}

	if r.nameTaken(m) {
		return domain.ErrHardwareModelNameTaken
	}

	r.s.data.hardwareModels[m.ID] = cloneHardwareModel(m)

	return nil
}

// Update stores the name, UpdatedAt and DeletedAt of an existing model,
// rejecting a live sibling with the same name while m is live.
func (r *HardwareModelRepository) Update(_ context.Context, m domain.HardwareModel) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.hardwareModels[m.ID]
	if !ok {
		return domain.ErrHardwareModelNotFound
	}

	m.ManufacturerID, m.CreatedAt = current.ManufacturerID, current.CreatedAt

	if r.nameTaken(m) {
		return domain.ErrHardwareModelNameTaken
	}

	r.s.data.hardwareModels[m.ID] = cloneHardwareModel(m)

	return nil
}

// Get returns the model with id and its count.
func (r *HardwareModelRepository) Get(_ context.Context, id domain.HardwareModelID) (domain.HardwareModelListItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	m, ok := r.s.data.hardwareModels[id]
	if !ok {
		return domain.HardwareModelListItem{}, domain.ErrHardwareModelNotFound
	}

	return r.listItem(m), nil
}

// GetMany returns the known models among ids.
func (r *HardwareModelRepository) GetMany(_ context.Context, ids []domain.HardwareModelID) ([]domain.HardwareModel, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	out := make([]domain.HardwareModel, 0, len(ids))

	for _, id := range ids {
		if m, ok := r.s.data.hardwareModels[id]; ok {
			out = append(out, cloneHardwareModel(m))
		}
	}

	return out, nil
}

// List returns one page of the manufacturer's models filter keeps, ordered
// by lower(name), then ID.
func (r *HardwareModelRepository) List(
	_ context.Context, manufacturer domain.ManufacturerID, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.HardwareModelListItem], error) {
	query := strings.ToLower(filter.Query)

	r.s.mu.RLock()

	var all []domain.HardwareModelListItem

	for _, m := range r.s.data.hardwareModels {
		if m.ManufacturerID == manufacturer && strings.Contains(strings.ToLower(m.Name), query) &&
			(filter.IncludeDeleted || !m.IsDeleted()) {
			all = append(all, r.listItem(m))
		}
	}

	r.s.mu.RUnlock()

	return pageByName(all, func(m domain.HardwareModelListItem) (string, string) { return m.Name, m.ID.String() }, page)
}

// nameTaken reports whether another live model of the same manufacturer
// has m's name, while m is live. Callers hold r.s.mu.
func (r *HardwareModelRepository) nameTaken(m domain.HardwareModel) bool {
	if m.IsDeleted() {
		return false
	}

	for _, o := range r.s.data.hardwareModels {
		if o.ID != m.ID && o.ManufacturerID == m.ManufacturerID && !o.IsDeleted() && sameName(o.Name, m.Name) {
			return true
		}
	}

	return false
}

// listItem adds the count to m. Callers hold r.s.mu.
func (r *HardwareModelRepository) listItem(m domain.HardwareModel) domain.HardwareModelListItem {
	return domain.HardwareModelListItem{
		HardwareModel: cloneHardwareModel(m),
		ItemCount:     r.s.countHardware(func(h domain.Hardware) bool { return h.ModelID != nil && *h.ModelID == m.ID }),
	}
}

// countHardware counts the live items whose hardware module matches.
// Callers hold s.mu.
func (s *Store) countHardware(match func(domain.Hardware) bool) int64 {
	var n int64

	for _, i := range s.data.storageItems {
		if hw := domain.HardwareOf(i.Modules); hw != nil && !i.IsDeleted() && match(*hw) {
			n++
		}
	}

	return n
}

// hardwareNames returns the names of the manufacturer and model hw names,
// empty for unknown ones. Callers hold s.mu.
func (s *Store) hardwareNames(hw domain.Hardware) (manufacturer, model string) {
	if hw.ManufacturerID != nil {
		manufacturer = s.data.manufacturers[*hw.ManufacturerID].Name
	}

	if hw.ModelID != nil {
		model = s.data.hardwareModels[*hw.ModelID].Name
	}

	return manufacturer, model
}

func cloneManufacturer(m domain.Manufacturer) domain.Manufacturer {
	m.DeletedAt = clonePtr(m.DeletedAt)

	return m
}

func cloneHardwareModel(m domain.HardwareModel) domain.HardwareModel {
	m.DeletedAt = clonePtr(m.DeletedAt)

	return m
}
