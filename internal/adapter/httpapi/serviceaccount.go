package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// serviceAccountResponse is the JSON representation of a service account.
type serviceAccountResponse struct {
	userResponse

	// Name is the account name without the "bot-" prefix.
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func toServiceAccountResponse(u domain.User) serviceAccountResponse {
	return serviceAccountResponse{userResponse: toUserResponse(u), Name: u.ServiceAccountName(), CreatedAt: u.CreatedAt}
}

// apiKeyResponse is the JSON representation of an API key. The token is
// never included; see createdAPIKeyResponse.
type apiKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedBy  string     `json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

func toAPIKeyResponse(k domain.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID: k.ID.String(), Name: k.Name, Prefix: k.Prefix, CreatedBy: k.CreatedBy.String(),
		CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt,
	}
}

// createdAPIKeyResponse adds the token, which is only ever shown here.
type createdAPIKeyResponse struct {
	apiKeyResponse

	Token string `json:"token"`
}

type createServiceAccountRequest struct {
	Name string `json:"name"`
}

type createAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// GET /api/service-accounts.
func (h *Handler) listServiceAccounts(w http.ResponseWriter, r *http.Request, _ domain.User) {
	accounts, err := h.accounts.List(r.Context())
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]serviceAccountResponse, 0, len(accounts))
	for _, a := range accounts {
		resp = append(resp, toServiceAccountResponse(a))
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /api/service-accounts.
func (h *Handler) createServiceAccount(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createServiceAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	account, err := h.accounts.Create(r.Context(), user, req.Name)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toServiceAccountResponse(account))
}

// GET /api/service-accounts/{id}/keys.
func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request, _ domain.User) {
	account, ok := pathID[domain.UserID](r, "id")
	if !ok {
		h.writeDomainError(w, r, domain.ErrServiceAccountNotFound)
		return
	}

	keys, err := h.accounts.ListKeys(r.Context(), account)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toAPIKeyResponse(k))
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /api/service-accounts/{id}/keys. The response carries the token.
func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request, user domain.User) {
	account, ok := pathID[domain.UserID](r, "id")
	if !ok {
		h.writeDomainError(w, r, domain.ErrServiceAccountNotFound)
		return
	}

	var req createAPIKeyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	key, token, err := h.accounts.CreateKey(r.Context(), user, account, req.Name, req.ExpiresAt)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	h.logger.InfoContext(r.Context(), "api key created",
		slog.String("key_id", key.ID.String()), slog.String("service_account_id", account.String()),
		slog.String("by_user_id", user.ID.String()))

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, createdAPIKeyResponse{apiKeyResponse: toAPIKeyResponse(key), Token: token})
}

// DELETE /api/service-accounts/{id}/keys/{keyId} revokes the key.
func (h *Handler) revokeAPIKey(w http.ResponseWriter, r *http.Request, user domain.User) {
	account, ok := pathID[domain.UserID](r, "id")
	if !ok {
		h.writeDomainError(w, r, domain.ErrServiceAccountNotFound)
		return
	}

	keyID, ok := pathID[domain.APIKeyID](r, "keyId")
	if !ok {
		h.writeDomainError(w, r, domain.ErrAPIKeyNotFound)
		return
	}

	if _, err := h.accounts.RevokeKey(r.Context(), user, account, keyID); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	h.logger.InfoContext(r.Context(), "api key revoked",
		slog.String("key_id", keyID.String()), slog.String("service_account_id", account.String()),
		slog.String("by_user_id", user.ID.String()))

	w.WriteHeader(http.StatusNoContent)
}

// pathID parses the UUID path parameter name.
func pathID[T ~[16]byte](r *http.Request, name string) (T, bool) {
	id, err := uuid.Parse(r.PathValue(name))

	return T(id), err == nil
}
