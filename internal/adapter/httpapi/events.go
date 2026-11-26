package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

const (
	// eventsPingInterval keeps idle connections alive through proxies and
	// detects dead peers.
	eventsPingInterval = 30 * time.Second
	// eventsReauthInterval is how often the caller's credentials are checked
	// again, so a connection ends soon after sign-out, session expiry or key
	// revocation.
	eventsReauthInterval = time.Minute
	// eventsWriteTimeout bounds each message and ping.
	eventsWriteTimeout = 10 * time.Second
)

// Close codes of /api/events besides the standard ones. 4000-4999 are
// reserved for applications.
const (
	// closeUnauthorized: the credentials are no longer valid; reconnecting
	// needs a new sign-in.
	closeUnauthorized websocket.StatusCode = 4401
)

// changeMessage is the JSON text message sent for each committed change. It
// names what changed, not the new data: clients reload through the API.
type changeMessage struct {
	Type    domain.ChangeKind `json:"type"`
	Project domain.ProjectKey `json:"project"`
	Issue   int64             `json:"issue,omitempty"`
}

// events streams change notifications over a WebSocket: GET /api/events.
//
// The connection is server-to-client only; a data message from the client
// closes it. Clients that get disconnected have missed changes and should
// reload what they show after reconnecting.
func (h *Handler) events(w http.ResponseWriter, r *http.Request, _ domain.User) {
	// The server's read/write timeouts are meant for ordinary requests and
	// would cut this long-lived connection.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	// Subscribe before the handshake completes, so no change committed after
	// the client sees the connection open is missed.
	subCtx, unsubscribe := context.WithCancel(r.Context())
	defer unsubscribe()

	changes := h.changes.Subscribe(subCtx)

	// Accept rejects browser handshakes whose Origin is neither the request
	// host nor the public URL (cross-site WebSocket hijacking); GET is not
	// covered by the CSRF middleware.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.wsOrigins})
	if err != nil {
		h.logger.DebugContext(r.Context(), "websocket handshake rejected", slog.Any("error", err))
		return // Accept has answered the request.
	}
	defer func() { _ = conn.CloseNow() }()

	// CloseRead answers pings and close frames; its ctx ends with the connection.
	ctx := conn.CloseRead(subCtx)

	status, reason := h.streamChanges(ctx, r, conn, changes)
	if r.Context().Err() != nil {
		status, reason = websocket.StatusGoingAway, "server shutting down"
	}

	// Fails harmlessly when the client already closed the connection.
	_ = conn.Close(status, reason)
}

// streamChanges writes changes to conn until it should close, and says
// with which status.
func (h *Handler) streamChanges(
	ctx context.Context, r *http.Request, conn *websocket.Conn, changes <-chan domain.Change,
) (websocket.StatusCode, string) {
	ping := time.NewTicker(eventsPingInterval)
	defer ping.Stop()

	reauth := time.NewTicker(eventsReauthInterval)
	defer reauth.Stop()

	for {
		select {
		case <-ctx.Done():
			return websocket.StatusNormalClosure, ""

		case c, ok := <-changes:
			if !ok {
				return websocket.StatusTryAgainLater, "fell behind"
			}

			if err := h.writeChange(ctx, conn, c); err != nil {
				return websocket.StatusInternalError, "write failed"
			}

		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, eventsWriteTimeout)
			err := conn.Ping(pctx)

			cancel()

			if err != nil {
				return websocket.StatusPolicyViolation, "ping timeout"
			}

		case <-reauth.C:
			_, err := h.authenticate(r)
			if _, isDomain := errors.AsType[*domain.Error](err); isDomain {
				return closeUnauthorized, "credentials no longer valid"
			}

			if err != nil {
				h.logger.WarnContext(ctx, "websocket re-authentication failed", slog.Any("error", err))
				return websocket.StatusTryAgainLater, "try again later"
			}
		}
	}
}

func (h *Handler) writeChange(ctx context.Context, conn *websocket.Conn, c domain.Change) error {
	wctx, cancel := context.WithTimeout(ctx, eventsWriteTimeout)
	defer cancel()

	return wsjson.Write(wctx, conn, changeMessage{Type: c.Kind, Project: c.Project, Issue: c.Issue})
}
