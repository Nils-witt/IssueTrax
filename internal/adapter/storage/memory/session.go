package memory

import (
	"context"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that SessionRepository satisfies the driven port.
var _ port.SessionRepository = (*SessionRepository)(nil)

// SessionRepository implements port.SessionRepository on a Store.
type SessionRepository struct{ s *Store }

// NewSessionRepository returns a SessionRepository backed by s.
func NewSessionRepository(s *Store) *SessionRepository { return &SessionRepository{s: s} }

// Create inserts session.
func (r *SessionRepository) Create(_ context.Context, session domain.Session) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.sessions[string(session.TokenHash)] = session

	return nil
}

// Get returns the session with tokenHash.
func (r *SessionRepository) Get(_ context.Context, tokenHash []byte) (domain.Session, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	s, ok := r.s.data.sessions[string(tokenHash)]
	if !ok {
		return domain.Session{}, domain.ErrNoSession
	}

	return s, nil
}

// Delete removes the session with tokenHash, if any.
func (r *SessionRepository) Delete(_ context.Context, tokenHash []byte) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	delete(r.s.data.sessions, string(tokenHash))

	return nil
}

// DeleteExpired removes sessions that expired at or before now.
func (r *SessionRepository) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	var n int64

	for k, s := range r.s.data.sessions {
		if s.Expired(now) {
			delete(r.s.data.sessions, k)

			n++
		}
	}

	return n, nil
}
