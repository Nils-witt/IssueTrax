-- +goose Up
-- API keys let service accounts (users with the synthetic issuer
-- urn:issuetrax:service-account) call the API. Only token hashes are stored.
CREATE TABLE api_keys (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id),
    name         text NOT NULL,
    token_hash   bytea NOT NULL,
    prefix       text NOT NULL,
    created_by   uuid NOT NULL REFERENCES users (id),
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    CONSTRAINT api_keys_token_hash_key UNIQUE (token_hash)
);

CREATE INDEX api_keys_user_id_idx ON api_keys (user_id, created_at DESC);

-- +goose Down
DROP TABLE api_keys;
