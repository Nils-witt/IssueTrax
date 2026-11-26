-- name: CreateStorage :exec
INSERT INTO storages (id, parent_id, name, created_at, updated_at)
VALUES (@id, @parent_id, @name, @created_at, @updated_at);

-- name: UpdateStorage :execrows
UPDATE storages
SET parent_id = @parent_id, name = @name, updated_at = @updated_at
WHERE id = @id;

-- name: GetStorage :one
SELECT s.id, s.parent_id, s.name, s.created_at, s.updated_at,
       (SELECT count(*) FROM storages c WHERE c.parent_id = s.id)::bigint AS child_count
FROM storages s
WHERE s.id = @id;

-- name: ListStorageChildren :many
SELECT s.id, s.parent_id, s.name, s.created_at, s.updated_at,
       (SELECT count(*) FROM storages c WHERE c.parent_id = s.id)::bigint AS child_count
FROM storages s
WHERE s.parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid
  AND (NOT @has_cursor::boolean OR (lower(s.name), s.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(s.name), s.id
LIMIT @row_limit;

-- name: CountStorageChildren :one
SELECT count(*) FROM storages
WHERE parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid;

-- name: StorageIsDescendant :one
-- Walks candidate's ancestors; UNION (not UNION ALL) stops on a cycle.
WITH RECURSIVE ancestors (id) AS (
    SELECT s.parent_id FROM storages s WHERE s.id = @candidate::uuid AND s.parent_id IS NOT NULL
    UNION
    SELECT s.parent_id FROM storages s JOIN ancestors a ON s.id = a.id WHERE s.parent_id IS NOT NULL
)
SELECT EXISTS (SELECT 1 FROM ancestors WHERE ancestors.id = @ancestor::uuid);

-- name: LockStorageTree :exec
-- Serializes storage moves until the transaction ends. The key is an
-- arbitrary constant reserved for the storage tree.
SELECT pg_advisory_xact_lock(7320414101);
