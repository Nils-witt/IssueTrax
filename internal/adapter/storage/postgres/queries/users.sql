-- name: CreateUser :exec
INSERT INTO users (id, issuer, subject, email, display_name, is_admin, created_at, last_login_at)
VALUES (@id, @issuer, @subject, @email, @display_name, @is_admin, @created_at, @last_login_at);

-- name: UpdateUser :execrows
UPDATE users
SET email = @email, display_name = @display_name, is_admin = @is_admin, last_login_at = @last_login_at
WHERE id = @id;

-- name: GetUser :one
SELECT id, issuer, subject, email, display_name, created_at, last_login_at, is_admin FROM users WHERE id = @id;

-- name: GetUserByIdentity :one
SELECT id, issuer, subject, email, display_name, created_at, last_login_at, is_admin
FROM users
WHERE issuer = @issuer AND subject = @subject;

-- name: GetUsers :many
SELECT id, issuer, subject, email, display_name, created_at, last_login_at, is_admin FROM users WHERE id = ANY (@ids::uuid[]);

-- name: ListUsers :many
SELECT id, issuer, subject, email, display_name, created_at, last_login_at, is_admin
FROM users
ORDER BY lower(display_name), id;

-- name: ListUsersByIssuer :many
SELECT id, issuer, subject, email, display_name, created_at, last_login_at, is_admin
FROM users
WHERE issuer = @issuer
ORDER BY lower(display_name), id;
