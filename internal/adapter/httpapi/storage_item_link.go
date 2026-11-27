package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// storageItemRefResponse is a storage item as linked from issues.
type storageItemRefResponse struct {
	ID        string `json:"id"`
	AssetTag  string `json:"assetTag"`
	StorageID string `json:"storageId"`
	Name      string `json:"name"`
	Deleted   bool   `json:"deleted"`
}

func toStorageItemRefResponse(i domain.StorageItem) storageItemRefResponse {
	return storageItemRefResponse{
		ID:        i.ID.String(),
		AssetTag:  i.AssetTag,
		StorageID: i.StorageID.String(),
		Name:      i.Name,
		Deleted:   i.IsDeleted(),
	}
}

// GET /api/storage-items?q=&deleted=&limit=&cursor= searches items.
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

	writeJSON(w, http.StatusOK, toPageResponse(found, func(v port.StorageItemView) storageItemRefResponse {
		return toStorageItemRefResponse(v.StorageItem)
	}))
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
