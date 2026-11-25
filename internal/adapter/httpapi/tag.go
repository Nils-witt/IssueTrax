package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type tagResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func toTagResponse(t domain.Tag) tagResponse {
	return tagResponse{ID: t.ID.String(), Name: string(t.Name), Color: string(t.Color)}
}

// updateTagRequest is a partial update; absent fields stay unchanged.
type updateTagRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// GET /api/projects/{key}/tags.
func (h *Handler) listTags(w http.ResponseWriter, r *http.Request, _ domain.User) {
	tags, err := h.tags.List(r.Context(), projectKey(r))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		resp = append(resp, toTagResponse(t))
	}

	writeJSON(w, http.StatusOK, resp)
}

// PATCH /api/projects/{key}/tags/{tagId}.
func (h *Handler) updateTag(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, err := uuid.Parse(r.PathValue("tagId"))
	if err != nil {
		h.writeDomainError(w, r, domain.ErrTagNotFound)
		return
	}

	var req updateTagRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	tag, err := h.tags.Update(r.Context(), projectKey(r), domain.TagID(id), port.UpdateTagInput{Name: req.Name, Color: req.Color})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toTagResponse(tag))
}
