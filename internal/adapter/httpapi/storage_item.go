package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type storageItemResponse struct {
	ID          string          `json:"id"`
	AssetTag    string          `json:"assetTag"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	StorageID   string          `json:"storageId"`
	Type        itemTypeRefJSON `json:"type"`
	Usable      bool            `json:"usable"`
	// OrderDate, CommissionedAt and DecommissionedAt are null when unknown.
	OrderDate             *time.Time `json:"orderDate"`
	CommissionedAt        *time.Time `json:"commissionedAt"`
	DecommissionedAt      *time.Time `json:"decommissionedAt"`
	DecommissioningReason string     `json:"decommissioningReason"`
	// PersonResponsible is null when nobody is responsible.
	PersonResponsible *userResponse `json:"personResponsible"`
	// Modules are in display order: by kind, primary first, then by name.
	Modules   []moduleJSON `json:"modules"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	// DeletedAt is null unless the item is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

func toStorageItemResponse(v port.StorageItemView) storageItemResponse {
	resp := storageItemResponse{
		ID: v.ID.String(), AssetTag: v.AssetTag, Name: v.Name, Description: v.Description, StorageID: v.StorageID.String(),
		Type: toItemTypeRef(v.Type), Usable: v.Usable, OrderDate: v.OrderDate, CommissionedAt: v.CommissionedAt,
		DecommissionedAt: v.DecommissionedAt, DecommissioningReason: v.DecommissioningReason,
		Modules: toModulesJSON(v), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, DeletedAt: v.DeletedAt,
	}

	if v.Responsible != nil {
		resp.PersonResponsible = new(toUserResponse(*v.Responsible))
	}

	return resp
}

type createStorageItemRequest struct {
	AssetTag              string       `json:"assetTag"`
	Name                  string       `json:"name"`
	Description           string       `json:"description"`
	StorageID             string       `json:"storageId"`
	TypeID                string       `json:"typeId"`
	Usable                *bool        `json:"usable"`
	OrderDate             *time.Time   `json:"orderDate"`
	CommissionedAt        *time.Time   `json:"commissionedAt"`
	DecommissionedAt      *time.Time   `json:"decommissionedAt"`
	DecommissioningReason string       `json:"decommissioningReason"`
	PersonResponsibleID   *string      `json:"personResponsibleId"`
	Modules               []moduleJSON `json:"modules"`
}

type updateStorageItemRequest struct {
	// AssetTag cannot change; sending it is a validation error.
	AssetTag    *string `json:"assetTag"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	// StorageID moves the item, TypeID retypes it.
	StorageID *string `json:"storageId"`
	TypeID    *string `json:"typeId"`
	Usable    *bool   `json:"usable"`
	// The dates and the responsible person are cleared by null.
	OrderDate             optional[time.Time] `json:"orderDate"`
	CommissionedAt        optional[time.Time] `json:"commissionedAt"`
	DecommissionedAt      optional[time.Time] `json:"decommissionedAt"`
	DecommissioningReason *string             `json:"decommissioningReason"`
	PersonResponsibleID   optional[string]    `json:"personResponsibleId"`
	// Modules replaces all module instances when present; instances sent
	// with their id are kept.
	Modules *[]moduleJSON `json:"modules"`
}

// GET /api/storage-items?q=&typeId=&includeSubtypes=&usable=&responsibleId=
// &decommissioned=&deleted=&limit=&cursor= searches items across storages.
func (h *Handler) searchStorageItems(w http.ResponseWriter, r *http.Request, _ domain.User) {
	page, filter, err := itemListParams(r)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	found, err := h.storageItems.Search(r.Context(), filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(found, toStorageItemResponse))
}

// GET /api/storages/{id}/storage-items?recursive=&... lists the items kept
// in a storage (or its subtree) with the same filters as the search.
func (h *Handler) listStorageItemsInStorage(w http.ResponseWriter, r *http.Request, _ domain.User) {
	storage, ok := storagePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageNotFound)
		return
	}

	page, filter, err := itemListParams(r)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	if filter.Recursive, err = boolParam(r.URL.Query(), "recursive"); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	items, err := h.storageItems.ListInStorage(r.Context(), storage, filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(items, toStorageItemResponse))
}

// itemListParams reads the page and the filters every item list takes.
func itemListParams(r *http.Request) (domain.PageRequest, port.StorageItemFilter, error) {
	q := r.URL.Query()
	filter := port.StorageItemFilter{Query: strings.TrimSpace(q.Get("q"))}

	page, err := pageRequest(q)
	if err != nil {
		return domain.PageRequest{}, filter, err
	}

	if filter.Deleted, err = boolParam(q, "deleted"); err != nil {
		return page, filter, err
	}

	if filter.IncludeSubtypes, err = boolParam(q, "includeSubtypes"); err != nil {
		return page, filter, err
	}

	if filter.Usable, err = optionalBoolParam(q, "usable"); err != nil {
		return page, filter, err
	}

	if filter.Decommissioned, err = optionalBoolParam(q, "decommissioned"); err != nil {
		return page, filter, err
	}

	if raw := q.Get("typeId"); raw != "" {
		if filter.TypeID, err = parseItemTypeID(raw, "typeId"); err != nil {
			return page, filter, err
		}
	}

	if raw := q.Get("responsibleId"); raw != "" {
		if filter.ResponsibleID, err = parseUserID(raw, "responsibleId"); err != nil {
			return page, filter, err
		}
	}

	return page, filter, nil
}

// POST /api/storage-items.
func (h *Handler) createStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createStorageItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in, err := req.input()
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	item, err := h.storageItems.Create(r.Context(), user.ID, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toStorageItemResponse(item))
}

func (req createStorageItemRequest) input() (port.CreateStorageItemInput, error) {
	in := port.CreateStorageItemInput{
		AssetTag: req.AssetTag, Name: req.Name, Description: req.Description, Usable: req.Usable,
		OrderDate: req.OrderDate, CommissionedAt: req.CommissionedAt, DecommissionedAt: req.DecommissionedAt,
		DecommissioningReason: req.DecommissioningReason,
	}

	storage, err := parseStorageID(req.StorageID, "storageId")
	if err != nil {
		return in, err
	}

	typ, err := parseItemTypeID(req.TypeID, "typeId")
	if err != nil {
		return in, err
	}

	in.StorageID, in.TypeID = *storage, *typ

	if req.PersonResponsibleID != nil {
		if in.ResponsibleID, err = parseUserID(*req.PersonResponsibleID, "personResponsibleId"); err != nil {
			return in, err
		}
	}

	in.Modules, err = moduleParams(req.Modules)

	return in, err
}

// GET /api/storage-items/{id}. Deleted items are returned too.
func (h *Handler) getStorageItem(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	item, err := h.storageItems.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toStorageItemResponse(item))
}

// PATCH /api/storage-items/{id}.
func (h *Handler) updateStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	var req updateStorageItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in, err := req.input()
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	item, err := h.storageItems.Update(r.Context(), user.ID, id, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toStorageItemResponse(item))
}

func (req updateStorageItemRequest) input() (port.UpdateStorageItemInput, error) {
	if req.AssetTag != nil {
		return port.UpdateStorageItemInput{}, domain.NewValidationError(map[string]string{"assetTag": domain.FieldImmutable})
	}

	in := port.UpdateStorageItemInput{
		Name: req.Name, Description: req.Description, Usable: req.Usable, DecommissioningReason: req.DecommissioningReason,
		OrderDate: dateChange(req.OrderDate), CommissionedAt: dateChange(req.CommissionedAt),
		DecommissionedAt: dateChange(req.DecommissionedAt),
	}

	var err error

	if req.StorageID != nil {
		if in.StorageID, err = parseStorageID(*req.StorageID, "storageId"); err != nil {
			return in, err
		}
	}

	if req.TypeID != nil {
		if in.TypeID, err = parseItemTypeID(*req.TypeID, "typeId"); err != nil {
			return in, err
		}
	}

	if req.PersonResponsibleID.Set {
		in.Responsible = &domain.StorageItemResponsibleChange{}

		if req.PersonResponsibleID.Value != nil {
			if in.Responsible.ID, err = parseUserID(*req.PersonResponsibleID.Value, "personResponsibleId"); err != nil {
				return in, err
			}
		}
	}

	if req.Modules != nil {
		modules, err := moduleParams(*req.Modules)
		if err != nil {
			return in, err
		}

		in.Modules = &modules
	}

	return in, nil
}

func dateChange(o optional[time.Time]) *domain.StorageItemDateChange {
	if !o.Set {
		return nil
	}

	return &domain.StorageItemDateChange{At: o.Value}
}

// DELETE /api/storage-items/{id} marks the item deleted.
func (h *Handler) deleteStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	if err := h.storageItems.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/storage-items/{id}/restore.
func (h *Handler) restoreStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	item, err := h.storageItems.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toStorageItemResponse(item))
}

// parseUserID parses a user UUID; field names the input in the validation
// error.
func parseUserID(raw, field string) (*domain.UserID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, domain.NewValidationError(map[string]string{field: domain.FieldInvalid})
	}

	return new(domain.UserID(id)), nil
}
