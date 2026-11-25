-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, user_id, name, token_hash, prefix, created_by, created_at, expires_at, last_used_at, revoked_at)
VALUES (@id, @user_id, @name, @token_hash, @prefix, @created_by, @created_at, @expires_at, @last_used_at, @revoked_at);

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = @id;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE token_hash = @token_hash;

-- name: ListAPIKeysByUser :many
SELECT * FROM api_keys WHERE user_id = @user_id ORDER BY created_at DESC, id DESC;

-- name: UpdateAPIKey :execrows
UPDATE api_keys SET last_used_at = @last_used_at, revoked_at = @revoked_at WHERE id = @id;
