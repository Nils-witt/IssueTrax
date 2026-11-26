-- +goose Up
-- Admins are members of the configured identity provider group; the flag
-- is refreshed on every sign-in.
ALTER TABLE users ADD COLUMN is_admin boolean NOT NULL DEFAULT false;

-- Append-only record of sign-ins and changes, readable by admins.
CREATE TABLE audit_log (
    id           uuid PRIMARY KEY,
    at           timestamptz NOT NULL,
    actor_id     uuid REFERENCES users (id),
    action       text NOT NULL,
    target_type  text NOT NULL DEFAULT '',
    target_id    text NOT NULL DEFAULT '',
    target_label text NOT NULL DEFAULT '',
    changes      jsonb NOT NULL DEFAULT '{}',
    ip           text NOT NULL DEFAULT '',
    user_agent   text NOT NULL DEFAULT ''
);

CREATE INDEX audit_log_at_idx ON audit_log (at DESC, id DESC);
CREATE INDEX audit_log_actor_at_idx ON audit_log (actor_id, at DESC, id DESC);

-- +goose Down
DROP TABLE audit_log;
ALTER TABLE users DROP COLUMN is_admin;
