package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type genericStorageItemResponse struct {
	ID          string    `json:"id"`
	StorageID   string    `json:"storageId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	// DeletedAt is null unless the item is deleted.
	DeletedAt *time.Time `json:"deletedAt"`
}

func toGenericStorageItemResponse(i domain.GenericStorageItem) genericStorageItemResponse {
	return genericStorageItemResponse{
		ID:          i.ID.String(),
		StorageID:   i.StorageID.String(),
		Name:        i.Name,
		Description: i.Description,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
		DeletedAt:   i.DeletedAt,
	}
}

type createGenericStorageItemRequest struct {
	StorageID   string `json:"storageId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateGenericStorageItemRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	// StorageID moves the item when present.
	StorageID *string `json:"storageId"`
}

// GET /api/generic-storage-items?q=&deleted=&limit=&cursor=.
func (h *Handler) searchGenericStorageItems(w http.ResponseWriter, r *http.Request, _ domain.User) {
	searchItems(h, w, r, h.genericItems, toGenericStorageItemResponse)
}

// GET /api/storages/{id}/generic-storage-items?recursive=&deleted=&limit=&cursor=.
func (h *Handler) listGenericStorageItemsInStorage(w http.ResponseWriter, r *http.Request, _ domain.User) {
	listItemsInStorage(h, w, r, h.genericItems, toGenericStorageItemResponse)
}

// POST /api/generic-storage-items.
func (h *Handler) createGenericStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createGenericStorageItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	storage, err := parseStorageID(req.StorageID, "storageId")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	item, err := h.genericItems.Create(r.Context(), user.ID, port.CreateGenericStorageItemInput{
		StorageID: *storage, Name: req.Name, Description: req.Description,
	})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toGenericStorageItemResponse(item))
}

// GET /api/generic-storage-items/{id}. Deleted items are returned too.
func (h *Handler) getGenericStorageItem(w http.ResponseWriter, r *http.Request, _ domain.User) {
	getItem(h, w, r, h.genericItems, toGenericStorageItemResponse)
}

// PATCH /api/generic-storage-items/{id}.
func (h *Handler) updateGenericStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	var req updateGenericStorageItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.UpdateGenericStorageItemInput{Name: req.Name, Description: req.Description}

	if req.StorageID != nil {
		storage, err := parseStorageID(*req.StorageID, "storageId")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.StorageID = storage
	}

	item, err := h.genericItems.Update(r.Context(), user.ID, id, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toGenericStorageItemResponse(item))
}

// DELETE /api/generic-storage-items/{id} marks the item deleted.
func (h *Handler) deleteGenericStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	if err := h.genericItems.Delete(r.Context(), user.ID, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/generic-storage-items/{id}/restore.
func (h *Handler) restoreGenericStorageItem(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	item, err := h.genericItems.Restore(r.Context(), user.ID, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toGenericStorageItemResponse(item))
}

// searchItems answers ?q=&deleted=&limit=&cursor= with one page of items
// of any kind across all storages.
func searchItems[T, R any](h *Handler, w http.ResponseWriter, r *http.Request, items port.StorageItemReader[T], toResponse func(T) R) {
	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	deleted, err := boolParam(q, "deleted")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter := port.StorageItemFilter{Query: strings.TrimSpace(q.Get("q")), Deleted: deleted}

	found, err := items.Search(r.Context(), filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(found, toResponse))
}

// listItemsInStorage answers ?recursive=&deleted=&limit=&cursor= with one
// page of the items of any kind in the storage {id}.
func listItemsInStorage[T, R any](
	h *Handler, w http.ResponseWriter, r *http.Request, items port.StorageItemReader[T], toResponse func(T) R,
) {
	storage, ok := storagePathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageNotFound)
		return
	}

	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	var filter port.StorageItemFilter

	if filter.Recursive, err = boolParam(q, "recursive"); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	if filter.Deleted, err = boolParam(q, "deleted"); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	listed, err := items.ListInStorage(r.Context(), storage, filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(listed, toResponse))
}

// getItem answers with the item {id} of any kind, deleted or not.
func getItem[T, R any](h *Handler, w http.ResponseWriter, r *http.Request, items port.StorageItemReader[T], toResponse func(T) R) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	item, err := items.Get(r.Context(), id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toResponse(item))
}

func storageItemPathID(r *http.Request) (domain.StorageItemID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))

	return domain.StorageItemID(id), err == nil
}

// boolParam reads an optional "true"/"false" query parameter; absent or
// empty is false.
func boolParam(q url.Values, name string) (bool, error) {
	switch q.Get(name) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, domain.NewValidationError(map[string]string{name: domain.FieldInvalid})
	}
}
