package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type commentResponse struct {
	ID        string       `json:"id"`
	Body      string       `json:"body"`
	Author    userResponse `json:"author"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

func toCommentResponse(v port.CommentView) commentResponse {
	return commentResponse{
		ID:        v.Comment.ID.String(),
		Body:      v.Comment.Body,
		Author:    toUserResponse(v.Author),
		CreatedAt: v.Comment.CreatedAt,
		UpdatedAt: v.Comment.UpdatedAt,
	}
}

// commentRequest is the body of both creating and editing a comment.
type commentRequest struct {
	Body string `json:"body"`
}

// GET /api/projects/{key}/issues/{number}/comments.
func (h *Handler) listComments(w http.ResponseWriter, r *http.Request, _ domain.User) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	comments, err := h.comments.List(r.Context(), projectKey(r), number)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]commentResponse, 0, len(comments))
	for _, c := range comments {
		resp = append(resp, toCommentResponse(c))
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /api/projects/{key}/issues/{number}/comments.
func (h *Handler) createComment(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	var req commentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	view, err := h.comments.Create(r.Context(), user.ID, projectKey(r), number, req.Body)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toCommentResponse(view))
}

// PATCH /api/projects/{key}/issues/{number}/comments/{commentId}.
func (h *Handler) updateComment(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, id, ok := h.commentPath(w, r)
	if !ok {
		return
	}

	var req commentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	view, err := h.comments.Update(r.Context(), user.ID, projectKey(r), number, id, req.Body)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toCommentResponse(view))
}

// DELETE /api/projects/{key}/issues/{number}/comments/{commentId}.
func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, id, ok := h.commentPath(w, r)
	if !ok {
		return
	}

	if err := h.comments.Delete(r.Context(), user.ID, projectKey(r), number, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// commentPath parses {number} and {commentId}, answering 404 when either is
// malformed.
func (h *Handler) commentPath(w http.ResponseWriter, r *http.Request) (int64, domain.CommentID, bool) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return 0, domain.CommentID{}, false
	}

	id, err := uuid.Parse(r.PathValue("commentId"))
	if err != nil {
		h.writeDomainError(w, r, domain.ErrCommentNotFound)
		return 0, domain.CommentID{}, false
	}

	return number, domain.CommentID(id), true
}
