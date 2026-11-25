-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, id_token, created_at, expires_at)
VALUES (@token_hash, @user_id, @id_token, @created_at, @expires_at);

-- name: GetSession :one
SELECT token_hash, user_id, id_token, created_at, expires_at FROM sessions WHERE token_hash = @token_hash;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = @token_hash;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= @now;
