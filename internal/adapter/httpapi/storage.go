package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type storageResponse struct {
	ID string `json:"id"`
	// ParentID is null for a root storage.
	ParentID   *string   `json:"parentId"`
	Name       string    `json:"name"`
	ChildCount int64     `json:"childCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func toStorageResponse(s domain.StorageListItem) storageResponse {
	resp := storageResponse{
		ID:         s.ID.String(),
		Name:       s.Name,
		ChildCount: s.ChildCount,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}

	if s.ParentID != nil {
		parent := s.ParentID.String()
		resp.ParentID = &parent
	}

	return resp
}

type createStorageRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

type updateStorageRequest struct {
	Name *string `json:"name"`
	// ParentID moves the storage when present; null makes it a root.
	ParentID optional[string] `json:"parentId"`
}

// GET /api/storages?parent=&limit=&cursor=. Without parent it lists the roots.
func (h *Handler) listStorages(w http.ResponseWriter, r *http.Request, _ domain.User) {
	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	var parent *domain.StorageID

	if q.Has("parent") {
		if parent, err = parseStorageID(q.Get("parent"), "parent"); err != nil {
			h.writeDomainError(w, r, err)
			return
		}
	}

	storages, err := h.storages.List(r.Context(), parent, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(storages, toStorageResponse))
}

// POST /api/storages.
func (h *Handler) createStorage(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createStorageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.CreateStorageInput{Name: req.Name}

	if req.ParentID != nil {
		parent, err := parseStorageID(*req.ParentID, "parentId")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.ParentID = parent
	}

	storage, err := h.storages.Create(r.Context(), user.ID, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toStorageResponse(storage))
}

// GET /api/storages/{id}.
func (h *Handler) getStorage(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := storagePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageNotFound)
		return
	}

	storage, err := h.storages.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toStorageResponse(storage))
}

// PATCH /api/storages/{id}.
func (h *Handler) updateStorage(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storagePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageNotFound)
		return
	}

	var req updateStorageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.UpdateStorageInput{Name: req.Name}

	if req.ParentID.Set {
		in.Parent = &domain.StorageParentChange{}

		if req.ParentID.Value != nil {
			parent, err := parseStorageID(*req.ParentID.Value, "parentId")
			if err != nil {
				h.writeDomainError(w, r, err)
				return
			}

			in.Parent.ID = parent
		}
	}

	storage, err := h.storages.Update(r.Context(), user.ID, id, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toStorageResponse(storage))
}

// parseStorageID parses a storage UUID; field names the input in the
// validation error.
func parseStorageID(raw, field string) (*domain.StorageID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, domain.NewValidationError(map[string]string{field: domain.FieldInvalid})
	}

	sid := domain.StorageID(id)

	return &sid, nil
}

func storagePathID(r *http.Request) (domain.StorageID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))

	return domain.StorageID(id), err == nil
}
