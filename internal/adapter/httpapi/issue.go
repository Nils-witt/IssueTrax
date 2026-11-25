package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// issueResponse is the JSON representation of a domain.Issue. Keeping it
// separate from the domain type keeps the wire format an adapter concern.
type issueResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type createIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func toIssueResponse(i domain.Issue) issueResponse {
	return issueResponse{
		ID:          string(i.ID),
		Title:       i.Title,
		Description: i.Description,
		Status:      string(i.Status),
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
}

func (h *Handler) listIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := h.issues.List(r.Context())
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	resp := make([]issueResponse, 0, len(issues))
	for _, i := range issues {
		resp = append(resp, toIssueResponse(i))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getIssue(w http.ResponseWriter, r *http.Request) {
	issue, err := h.issues.Get(r.Context(), domain.IssueID(r.PathValue("id")))
	if errors.Is(err, domain.ErrIssueNotFound) {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	if err != nil {
		h.internalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toIssueResponse(issue))
}

func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request) {
	var req createIssueRequest

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	issue, err := h.issues.Create(r.Context(), port.CreateIssueInput{
		Title:       req.Title,
		Description: req.Description,
	})
	if errors.Is(err, domain.ErrInvalidIssue) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if err != nil {
		h.internalError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toIssueResponse(issue))
}

// internalError logs err and returns a generic 500 so internals don't leak.
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "request failed",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Any("error", err),
	)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
