package httpapi

import (
	"net/http"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// userResponse is the JSON representation of a domain.User.
type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{ID: u.ID.String(), Email: u.Email, DisplayName: u.DisplayName}
}

// GET /api/users: everyone who has signed in at least once (assignee picker).
func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request, _ domain.User) {
	users, err := h.users.List(r.Context())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}

	writeJSON(w, http.StatusOK, resp)
}
