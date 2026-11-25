package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that SessionRepository satisfies the driven port.
var _ port.SessionRepository = (*SessionRepository)(nil)

// SessionRepository implements port.SessionRepository.
type SessionRepository struct{ db *DB }

// NewSessionRepository returns a SessionRepository on db.
func NewSessionRepository(db *DB) *SessionRepository { return &SessionRepository{db: db} }

// Create inserts session.
func (r *SessionRepository) Create(ctx context.Context, s domain.Session) error {
	err := r.db.queries(ctx).CreateSession(ctx, sqlc.CreateSessionParams{
		TokenHash: s.TokenHash,
		UserID:    uuid.UUID(s.UserID),
		IDToken:   s.IDToken,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

// Get returns the session with tokenHash.
func (r *SessionRepository) Get(ctx context.Context, tokenHash []byte) (domain.Session, error) {
	row, err := r.db.queries(ctx).GetSession(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNoSession
	}

	if err != nil {
		return domain.Session{}, fmt.Errorf("select session: %w", err)
	}

	return domain.Session{
		TokenHash: row.TokenHash,
		UserID:    domain.UserID(row.UserID),
		IDToken:   row.IDToken,
		CreatedAt: row.CreatedAt.UTC(),
		ExpiresAt: row.ExpiresAt.UTC(),
	}, nil
}

// Delete removes the session with tokenHash, if any.
func (r *SessionRepository) Delete(ctx context.Context, tokenHash []byte) error {
	if err := r.db.queries(ctx).DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// DeleteExpired removes sessions that expired at or before now.
func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	n, err := r.db.queries(ctx).DeleteExpiredSessions(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	return n, nil
}
