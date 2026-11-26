package httpapi

import (
	"net/http"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type projectResponse struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toProjectResponse(p domain.Project) projectResponse {
	return projectResponse{
		ID:          p.ID.String(),
		Key:         string(p.Key),
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

type createProjectRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	// Key is accepted only to reject it with a clear error: keys are immutable.
	Key         *string `json:"key"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// GET /api/projects?limit=&cursor=.
func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request, _ domain.User) {
	page, err := pageRequest(r.URL.Query())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	projects, err := h.projects.List(r.Context(), page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(projects, toProjectResponse))
}

// POST /api/projects.
func (h *Handler) createProject(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createProjectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	project, err := h.projects.Create(r.Context(), user.ID, port.CreateProjectInput{Key: req.Key, Name: req.Name, Description: req.Description})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toProjectResponse(project))
}

// GET /api/projects/{key}.
func (h *Handler) getProject(w http.ResponseWriter, r *http.Request, _ domain.User) {
	project, err := h.projects.Get(r.Context(), projectKey(r))
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toProjectResponse(project))
}

// PATCH /api/projects/{key}.
func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req updateProjectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	if req.Key != nil {
		h.writeDomainError(w, r, domain.NewValidationError(map[string]string{"key": domain.FieldImmutable}))
		return
	}

	project, err := h.projects.Update(r.Context(), user.ID, projectKey(r), port.UpdateProjectInput{Name: req.Name, Description: req.Description})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toProjectResponse(project))
}

func projectKey(r *http.Request) domain.ProjectKey { return domain.ProjectKey(r.PathValue("key")) }
