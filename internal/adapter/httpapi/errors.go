package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// Transport-level error codes (domain errors carry their own codes).
const (
	codeNotFound      = "not_found"
	codeInvalidJSON   = "invalid_json"
	codeInvalidUpload = "invalid_upload"
	codeInternal      = "internal_error"
	codeCrossOrigin   = "cross_origin_rejected"
)

// errorResponse is the JSON shape of every API error:
//
//	{"error": {"code": "issue_archived", "message": "...", "fields": {"title": "too_long"}}}
//
// code is stable for clients to translate; message is English; fields only
// appears on validation errors.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}

// writeDomainError maps a core error to its HTTP status; anything that is
// not a domain error becomes a logged, generic 500.
func (h *Handler) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	de, ok := errors.AsType[*domain.Error](err)
	if !ok {
		h.internalError(w, r, err)
		return
	}

	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrIssueArchived):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrUnauthenticated):
		status = http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	}

	writeJSON(w, status, errorResponse{Error: errorDetail{Code: de.Code, Message: de.Message, Fields: de.Fields}})
}

// internalError logs err and returns a generic 500 so internals don't leak.
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "request failed",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Any("error", err),
	)
	writeError(w, http.StatusInternalServerError, codeInternal, "internal server error")
}
