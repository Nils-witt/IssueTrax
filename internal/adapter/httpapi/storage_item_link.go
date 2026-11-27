package httpapi

import (
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
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

// optionalBoolParam reads an optional "true"/"false" query parameter;
// absent or empty is nil.
func optionalBoolParam(q url.Values, name string) (*bool, error) {
	if q.Get(name) == "" {
		return nil, nil //nolint:nilnil // nil means "no filter"
	}

	v, err := boolParam(q, name)

	return &v, err
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
