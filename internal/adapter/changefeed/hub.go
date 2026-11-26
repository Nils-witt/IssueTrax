// Package changefeed is an in-process change broker: services notify it of
// committed changes (port.ChangeNotifier) and inbound adapters subscribe to
// them (port.ChangeFeed).
//
// It only reaches subscribers in the same process. Running several
// IssueTrax instances needs a shared broker instead (e.g. Postgres
// LISTEN/NOTIFY) implementing the same ports.
package changefeed

import (
	"context"
	"sync"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// subscriberBuffer is how many changes a subscriber may lag behind before it
// is dropped. Changes arrive at human editing speed, so a full buffer means
// the subscriber is stuck, not busy.
const subscriberBuffer = 64

var (
	_ port.ChangeNotifier = (*Hub)(nil)
	_ port.ChangeFeed     = (*Hub)(nil)
)

// Hub fans changes out to subscribers. The zero value is not usable; call New.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch chan domain.Change
	// stop unregisters the context.AfterFunc that removes the subscriber.
	stop func() bool
}

// New returns an empty Hub.
func New() *Hub {
	return &Hub{subs: map[*subscriber]struct{}{}}
}

// Notify delivers change to every subscriber without blocking. A subscriber
// whose buffer is full is dropped: its channel is closed so it knows it
// missed changes.
func (h *Hub) Notify(_ context.Context, change domain.Change) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for s := range h.subs {
		select {
		case s.ch <- change:
		default:
			h.removeLocked(s)
		}
	}
}

// Subscribe implements port.ChangeFeed.
func (h *Hub) Subscribe(ctx context.Context) <-chan domain.Change {
	s := &subscriber{ch: make(chan domain.Change, subscriberBuffer)}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.subs[s] = struct{}{}
	s.stop = context.AfterFunc(ctx, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		h.removeLocked(s)
	})

	return s.ch
}

// removeLocked drops s and closes its channel, once. h.mu must be held.
func (h *Hub) removeLocked(s *subscriber) {
	if _, ok := h.subs[s]; !ok {
		return
	}

	delete(h.subs, s)
	close(s.ch)
	s.stop()
}
