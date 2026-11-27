-- name: CreateProject :exec
INSERT INTO projects (id, key, name, description, created_at, updated_at)
VALUES (@id, @key, @name, @description, @created_at, @updated_at);

-- name: UpdateProject :execrows
UPDATE projects
SET name = @name, description = @description, updated_at = @updated_at
WHERE id = @id;

-- name: GetProjectByKey :one
SELECT id, key, name, description, created_at, updated_at
FROM projects
WHERE key = @key;

-- name: GetProjectsByIDs :many
SELECT id, key, name, description, created_at, updated_at
FROM projects
WHERE id = ANY (@ids::uuid[]);

-- name: ListProjects :many
SELECT id, key, name, description, created_at, updated_at
FROM projects
WHERE NOT @has_cursor::boolean OR (lower(name), id) > (@cursor_name::text, @cursor_id::uuid)
ORDER BY lower(name), id
LIMIT @row_limit;

-- name: CountProjects :one
SELECT count(*) FROM projects;

-- name: AllocateIssueNumber :one
-- The row lock taken here serializes concurrent issue creation per project
-- until the surrounding transaction ends, keeping numbers gap-free.
UPDATE projects
SET next_issue_number = next_issue_number + 1
WHERE id = @id
RETURNING (next_issue_number - 1)::bigint;
