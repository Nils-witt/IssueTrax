package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// maxRequestBodyBytes caps JSON request bodies. Descriptions are limited to
// domain.MaxDescriptionBytes, which fits comfortably.
const maxRequestBodyBytes = 1 << 20

// errBadJSON marks request bodies that are not the expected JSON.
var errBadJSON = errors.New("invalid JSON body")

// decodeJSON reads exactly one JSON object into dst, rejecting unknown
// fields, trailing data and oversized bodies.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %w", errBadJSON, err)
	}

	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data", errBadJSON)
	}

	return nil
}

// writeRequestError answers a decoding or core error.
func (h *Handler) writeRequestError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errBadJSON) {
		writeError(w, http.StatusBadRequest, codeInvalidJSON, "request body is not valid JSON for this endpoint")
		return
	}

	h.writeDomainError(w, r, err)
}

// pageRequest reads ?limit= and ?cursor=.
func pageRequest(q url.Values) (domain.PageRequest, error) {
	limit := 0

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return domain.PageRequest{}, domain.NewValidationError(map[string]string{"limit": domain.FieldInvalid})
		}

		limit = n
	}

	return domain.NewPageRequest(limit, q.Get("cursor"))
}

// parseIDs parses UUID strings; field names the input in validation errors.
func parseIDs[T ~[16]byte](raw []string, field string) ([]T, error) {
	ids := make([]T, 0, len(raw))

	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, domain.NewValidationError(map[string]string{field: domain.FieldInvalid})
		}

		ids = append(ids, T(id))
	}

	return ids, nil
}

// listOfIssue answers a GET of an issue's comments or attachments: the
// whole list from list, each item converted for JSON.
func listOfIssue[V, R any](
	h *Handler, w http.ResponseWriter, r *http.Request,
	list func(ctx context.Context, key domain.ProjectKey, number int64) ([]V, error), convert func(V) R,
) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	items, err := list(r.Context(), projectKey(r), number)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	resp := make([]R, 0, len(items))
	for _, item := range items {
		resp = append(resp, convert(item))
	}

	writeJSON(w, http.StatusOK, resp)
}

// pageResponse is the JSON shape of every paginated list. nextCursor is
// null on the last page.
type pageResponse[T any] struct {
	Items      []T     `json:"items"`
	Total      int64   `json:"total"`
	NextCursor *string `json:"nextCursor"`
}

func toPageResponse[S, T any](p domain.Page[S], convert func(S) T) pageResponse[T] {
	resp := pageResponse[T]{Items: make([]T, 0, len(p.Items)), Total: p.Total}

	for _, item := range p.Items {
		resp.Items = append(resp.Items, convert(item))
	}

	if p.NextCursor != "" {
		resp.NextCursor = &p.NextCursor
	}

	return resp
}
