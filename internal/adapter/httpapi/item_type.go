package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type itemTypeRefJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

func toItemTypeRef(t domain.ItemType) itemTypeRefJSON {
	return itemTypeRefJSON{ID: t.ID.String(), Name: t.Name, Deleted: t.IsDeleted()}
}

type itemTypeResponse struct {
	ID string `json:"id"`
	// ParentID is null for a root type.
	ParentID  *string             `json:"parentId"`
	Name      string              `json:"name"`
	TagPrefix string              `json:"tagPrefix"`
	Defaults  []moduleDefaultJSON `json:"defaults"`
	// ChildCount counts all direct children, ItemCount the live items of
	// exactly this type.
	ChildCount int64     `json:"childCount"`
	ItemCount  int64     `json:"itemCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	// DeletedAt is null unless the type is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

// itemTypeViewResponse adds what a type inherits.
type itemTypeViewResponse struct {
	itemTypeResponse

	// Path lists the ancestors from the root down to the parent.
	Path               []itemTypeRefJSON   `json:"path"`
	EffectiveDefaults  []moduleDefaultJSON `json:"effectiveDefaults"`
	EffectiveTagPrefix string              `json:"effectiveTagPrefix"`
}

func toItemTypeResponse(t domain.ItemTypeListItem) itemTypeResponse {
	resp := itemTypeResponse{
		ID: t.ID.String(), Name: t.Name, TagPrefix: t.TagPrefix, Defaults: toModuleDefaultsJSON(t.Defaults),
		ChildCount: t.ChildCount, ItemCount: t.ItemCount, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DeletedAt: t.DeletedAt,
	}

	if t.ParentID != nil {
		resp.ParentID = new(t.ParentID.String())
	}

	return resp
}

func toItemTypeViewResponse(v port.ItemTypeView) itemTypeViewResponse {
	path := make([]itemTypeRefJSON, 0, len(v.Path))
	for _, t := range v.Path {
		path = append(path, toItemTypeRef(t))
	}

	return itemTypeViewResponse{
		itemTypeResponse:   toItemTypeResponse(v.ItemTypeListItem),
		Path:               path,
		EffectiveDefaults:  toModuleDefaultsJSON(v.EffectiveDefaults),
		EffectiveTagPrefix: v.EffectiveTagPrefix,
	}
}

type createItemTypeRequest struct {
	Name      string              `json:"name"`
	ParentID  *string             `json:"parentId"`
	TagPrefix string              `json:"tagPrefix"`
	Defaults  []moduleDefaultJSON `json:"defaults"`
}

type updateItemTypeRequest struct {
	Name *string `json:"name"`
	// ParentID moves the type when present; null makes it a root.
	ParentID  optional[string] `json:"parentId"`
	TagPrefix *string          `json:"tagPrefix"`
	// Defaults replaces the type's own defaults when present.
	Defaults *[]moduleDefaultJSON `json:"defaults"`
}

// GET /api/item-types?parent=&q=&includeDeleted=&limit=&cursor=. With q it
// searches the whole tree (results carry their path); otherwise it lists
// the children of parent, or the roots without it.
func (h *Handler) listItemTypes(w http.ResponseWriter, r *http.Request, _ domain.User) {
	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	includeDeleted, err := boolParam(q, "includeDeleted")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter := port.ItemTypeFilter{IncludeDeleted: includeDeleted}

	if query := strings.TrimSpace(q.Get("q")); query != "" {
		found, err := h.itemTypes.Search(r.Context(), query, filter, page)
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, toPageResponse(found, toItemTypeViewResponse))

		return
	}

	var parent *domain.ItemTypeID

	if q.Has("parent") {
		if parent, err = parseItemTypeID(q.Get("parent"), "parent"); err != nil {
			h.writeDomainError(w, r, err)
			return
		}
	}

	types, err := h.itemTypes.ListChildren(r.Context(), parent, filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(types, toItemTypeResponse))
}

// POST /api/item-types (admins only).
func (h *Handler) createItemType(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createItemTypeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.CreateItemTypeInput{Name: req.Name, TagPrefix: req.TagPrefix, Defaults: moduleDefaults(req.Defaults)}

	if req.ParentID != nil {
		parent, err := parseItemTypeID(*req.ParentID, "parentId")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.ParentID = parent
	}

	t, err := h.itemTypes.Create(r.Context(), user.ID, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toItemTypeViewResponse(t))
}

// GET /api/item-types/{id}. Deleted types are returned too.
func (h *Handler) getItemType(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := itemTypePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrItemTypeNotFound)
		return
	}

	t, err := h.itemTypes.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toItemTypeViewResponse(t))
}

// PATCH /api/item-types/{id} (admins only).
func (h *Handler) updateItemType(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := itemTypePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrItemTypeNotFound)
		return
	}

	var req updateItemTypeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.UpdateItemTypeInput{Name: req.Name, TagPrefix: req.TagPrefix}

	if req.Defaults != nil {
		in.Defaults = new(moduleDefaults(*req.Defaults))
	}

	if req.ParentID.Set {
		in.Parent = &domain.ItemTypeParentChange{}

		if req.ParentID.Value != nil {
			parent, err := parseItemTypeID(*req.ParentID.Value, "parentId")
			if err != nil {
				h.writeDomainError(w, r, err)
				return
			}

			in.Parent.ID = parent
		}
	}

	t, err := h.itemTypes.Update(r.Context(), user.ID, id, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toItemTypeViewResponse(t))
}

// DELETE /api/item-types/{id} (admins only) marks the type deleted.
func (h *Handler) deleteItemType(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := itemTypePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrItemTypeNotFound)
		return
	}

	if err := h.itemTypes.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/item-types/{id}/restore (admins only).
func (h *Handler) restoreItemType(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := itemTypePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrItemTypeNotFound)
		return
	}

	t, err := h.itemTypes.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toItemTypeViewResponse(t))
}

type nextAssetTagResponse struct {
	AssetTag string `json:"assetTag"`
}

// GET /api/item-types/{id}/next-asset-tag proposes the asset tag for a new
// item of the type without reserving it.
func (h *Handler) nextAssetTag(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := itemTypePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrItemTypeNotFound)
		return
	}

	tag, err := h.itemTypes.NextAssetTag(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, nextAssetTagResponse{AssetTag: tag})
}

// parseItemTypeID parses an item type UUID; field names the input in the
// validation error.
func parseItemTypeID(raw, field string) (*domain.ItemTypeID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, domain.NewValidationError(map[string]string{field: domain.FieldInvalid})
	}

	return new(domain.ItemTypeID(id)), nil
}

func itemTypePathID(r *http.Request) (domain.ItemTypeID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))

	return domain.ItemTypeID(id), err == nil
}
