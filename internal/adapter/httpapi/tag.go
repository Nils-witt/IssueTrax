package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

type tagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toTagResponse(t domain.Tag) tagResponse {
	return tagResponse{ID: t.ID.String(), Name: string(t.Name)}
}

type renameTagRequest struct {
	Name string `json:"name"`
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
func (h *Handler) renameTag(w http.ResponseWriter, r *http.Request, _ domain.User) {
	id, err := uuid.Parse(r.PathValue("tagId"))
	if err != nil {
		h.writeDomainError(w, r, domain.ErrTagNotFound)
		return
	}

	var req renameTagRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	tag, err := h.tags.Rename(r.Context(), projectKey(r), domain.TagID(id), req.Name)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toTagResponse(tag))
}
