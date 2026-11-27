package httpapi

import (
	"net/http"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// storageItemRefResponse is the shared part of a storage item of any kind,
// as linked from issues and found by the kind-agnostic search.
type storageItemRefResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	StorageID string `json:"storageId"`
	Name      string `json:"name"`
	Deleted   bool   `json:"deleted"`
}

func toStorageItemRefResponse(i domain.StorageItem) storageItemRefResponse {
	return storageItemRefResponse{
		ID:        i.ID.String(),
		Kind:      string(i.Kind),
		StorageID: i.StorageID.String(),
		Name:      i.Name,
		Deleted:   i.IsDeleted(),
	}
}

// GET /api/storage-items?q=&deleted=&limit=&cursor= searches items of every kind.
func (h *Handler) searchStorageItems(w http.ResponseWriter, r *http.Request, _ domain.User) {
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

	found, err := h.storageItems.Search(r.Context(), filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(found, toStorageItemRefResponse))
}

// GET /api/storage-items/{id}/issues?limit=&cursor= lists the issues of
// every project linking the item, newest first.
func (h *Handler) listStorageItemIssues(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, ok := storageItemPathID(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrStorageItemNotFound)
		return
	}

	page, err := pageRequest(r.URL.Query())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	issues, err := h.issues.ListByStorageItem(r.Context(), id, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(issues, toIssueResponse))
}
