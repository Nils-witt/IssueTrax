-- +goose Up
CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id),
    id_token   text NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

-- Periodic cleanup deletes by expiry.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
