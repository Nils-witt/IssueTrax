package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// maxUserAgentBytes bounds the user agent stored with audit entries.
const maxUserAgentBytes = 512

// auditActorResponse names who acted.
type auditActorResponse struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	ServiceAccount bool   `json:"serviceAccount"`
}

// auditChangeResponse is one changed field. Old is absent for fields set
// on creation; redacted fields (long text) carry neither value.
type auditChangeResponse struct {
	Old      *string `json:"old,omitempty"`
	New      *string `json:"new,omitempty"`
	Redacted bool    `json:"redacted,omitempty"`
}

// auditEntryResponse is the JSON representation of an audit entry.
type auditEntryResponse struct {
	ID          string                         `json:"id"`
	At          time.Time                      `json:"at"`
	Actor       *auditActorResponse            `json:"actor"`
	Action      string                         `json:"action"`
	TargetType  string                         `json:"targetType"`
	TargetID    string                         `json:"targetId"`
	TargetLabel string                         `json:"targetLabel"`
	Changes     map[string]auditChangeResponse `json:"changes"`
	IP          string                         `json:"ip"`
	UserAgent   string                         `json:"userAgent"`
}

func toAuditEntryResponse(v port.AuditEntryView) auditEntryResponse {
	e := v.Entry

	changes := make(map[string]auditChangeResponse, len(e.Changes))
	for field, c := range e.Changes {
		changes[field] = auditChangeResponse(c)
	}

	resp := auditEntryResponse{
		ID: e.ID.String(), At: e.At, Action: string(e.Action), TargetType: e.TargetType, TargetID: e.TargetID,
		TargetLabel: e.TargetLabel, Changes: changes, IP: e.IP, UserAgent: e.UserAgent,
	}

	if v.Actor != nil {
		resp.Actor = &auditActorResponse{ID: v.Actor.ID.String(), DisplayName: v.Actor.DisplayName, ServiceAccount: v.Actor.IsServiceAccount()}
	}

	return resp
}

// GET /api/audit-log?actor=&action=&targetType=&limit=&cursor= (admins only).
func (h *Handler) listAuditLog(w http.ResponseWriter, r *http.Request, user domain.User) {
	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter, err := auditFilter(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	entries, err := h.audit.List(r.Context(), user, filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(entries, toAuditEntryResponse))
}

func auditFilter(q url.Values) (port.AuditFilter, error) {
	filter := port.AuditFilter{TargetType: q.Get("targetType")}

	if raw := q.Get("actor"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return port.AuditFilter{}, domain.NewValidationError(map[string]string{"actor": domain.FieldInvalid})
		}

		actor := domain.UserID(id)
		filter.ActorID = &actor
	}

	for _, a := range q["action"] {
		filter.Actions = append(filter.Actions, domain.AuditAction(a))
	}

	return filter, nil
}

// withRequestInfo records the client address and user agent for audit
// entries. Forwarding headers are not trusted: behind a proxy the address
// is the proxy's.
func withRequestInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		// Postgres text must be valid UTF-8, also after truncating.
		ua := r.UserAgent()
		if len(ua) > maxUserAgentBytes {
			ua = ua[:maxUserAgentBytes]
		}

		ua = strings.ToValidUTF8(ua, "")

		ctx := port.WithRequestInfo(r.Context(), port.RequestInfo{IP: ip, UserAgent: ua})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
