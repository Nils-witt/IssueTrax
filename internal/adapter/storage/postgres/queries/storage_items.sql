-- name: CreateStorageItem :exec
INSERT INTO storage_items (id, storage_id, kind, name, description, created_at, updated_at, deleted_at)
VALUES (@id, @storage_id, @kind, @name, @description, @created_at, @updated_at, @deleted_at);

-- name: UpdateStorageItem :execrows
UPDATE storage_items
SET storage_id = @storage_id, name = @name, description = @description,
    updated_at = @updated_at, deleted_at = @deleted_at
WHERE id = @id AND kind = @kind;

-- name: GetStorageItem :one
SELECT id, storage_id, kind, name, description, created_at, updated_at, deleted_at
FROM storage_items
WHERE id = @id AND kind = @kind;

-- name: ListStorageItemsInStorage :many
-- Items of one kind in @storage_id, or in its whole subtree when @recursive.
-- @deleted selects deleted items instead of live ones.
WITH RECURSIVE subtree (id) AS (
    SELECT @storage_id::uuid
    UNION
    SELECT s.id FROM storages s JOIN subtree t ON s.parent_id = t.id WHERE @recursive::boolean
)
SELECT i.id, i.storage_id, i.kind, i.name, i.description, i.created_at, i.updated_at, i.deleted_at
FROM storage_items i
WHERE i.storage_id IN (SELECT id FROM subtree)
  AND i.kind = @kind
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean
  AND (NOT @has_cursor::boolean OR (lower(i.name), i.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(i.name), i.id
LIMIT @row_limit;

-- name: CountStorageItemsInStorage :one
WITH RECURSIVE subtree (id) AS (
    SELECT @storage_id::uuid
    UNION
    SELECT s.id FROM storages s JOIN subtree t ON s.parent_id = t.id WHERE @recursive::boolean
)
SELECT count(*) FROM storage_items i
WHERE i.storage_id IN (SELECT id FROM subtree)
  AND i.kind = @kind
  AND (i.deleted_at IS NOT NULL) = @deleted::boolean;

-- name: SearchStorageItems :many
-- @pattern is a pre-escaped ILIKE pattern ('%term%'), empty for no search.
SELECT id, storage_id, kind, name, description, created_at, updated_at, deleted_at
FROM storage_items
WHERE kind = @kind
  AND (deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR name ILIKE @pattern::text OR description ILIKE @pattern::text)
  AND (NOT @has_cursor::boolean OR (lower(name), id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(name), id
LIMIT @row_limit;

-- name: CountSearchStorageItems :one
SELECT count(*) FROM storage_items
WHERE kind = @kind
  AND (deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR name ILIKE @pattern::text OR description ILIKE @pattern::text);

-- name: GetStorageItemsByIDs :many
-- Items of any kind, deleted ones included.
SELECT id, storage_id, kind, name, description, created_at, updated_at, deleted_at
FROM storage_items
WHERE id = ANY (@ids::uuid[]);

-- name: SearchAllStorageItems :many
-- Like SearchStorageItems, across all kinds.
SELECT id, storage_id, kind, name, description, created_at, updated_at, deleted_at
FROM storage_items
WHERE (deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR name ILIKE @pattern::text OR description ILIKE @pattern::text)
  AND (NOT @has_cursor::boolean OR (lower(name), id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(name), id
LIMIT @row_limit;

-- name: CountSearchAllStorageItems :one
SELECT count(*) FROM storage_items
WHERE (deleted_at IS NOT NULL) = @deleted::boolean
  AND (@pattern::text = '' OR name ILIKE @pattern::text OR description ILIKE @pattern::text);
