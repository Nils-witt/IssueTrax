-- name: CreateItemType :exec
INSERT INTO item_types (id, parent_id, name, tag_prefix, created_at, updated_at, deleted_at)
VALUES (@id, @parent_id, @name, @tag_prefix, @created_at, @updated_at, @deleted_at);

-- name: UpdateItemType :execrows
UPDATE item_types
SET parent_id = @parent_id, name = @name, tag_prefix = @tag_prefix, updated_at = @updated_at, deleted_at = @deleted_at
WHERE id = @id;

-- name: DeleteItemTypeDefaults :exec
DELETE FROM item_type_module_defaults WHERE item_type_id = @item_type_id;

-- name: InsertItemTypeDefaults :exec
INSERT INTO item_type_module_defaults (item_type_id, module, default_count)
SELECT @item_type_id, unnest(@modules::text[]), unnest(@counts::integer[]);

-- name: ListItemTypeDefaults :many
SELECT item_type_id, module, default_count
FROM item_type_module_defaults
WHERE item_type_id = ANY (@item_type_ids::uuid[]);

-- name: GetItemType :one
SELECT t.id, t.parent_id, t.name, t.tag_prefix, t.created_at, t.updated_at, t.deleted_at,
       (SELECT count(*) FROM item_types c WHERE c.parent_id = t.id)::bigint AS child_count,
       (SELECT count(*) FROM storage_items i WHERE i.item_type_id = t.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM item_types t
WHERE t.id = @id;

-- name: GetItemTypesByIDs :many
SELECT id, parent_id, name, tag_prefix, created_at, updated_at, deleted_at
FROM item_types
WHERE id = ANY (@ids::uuid[]);

-- name: ListItemTypes :many
-- One page of the children of @parent_id (roots for NULL), or with
-- @any_parent of the whole tree. @pattern is a pre-escaped ILIKE pattern
-- ('%term%'), empty for no search.
SELECT t.id, t.parent_id, t.name, t.tag_prefix, t.created_at, t.updated_at, t.deleted_at,
       (SELECT count(*) FROM item_types c WHERE c.parent_id = t.id)::bigint AS child_count,
       (SELECT count(*) FROM storage_items i WHERE i.item_type_id = t.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM item_types t
WHERE (@any_parent::boolean OR t.parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid)
  AND (@pattern::text = '' OR t.name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR t.deleted_at IS NULL)
  AND (NOT @has_cursor::boolean OR (lower(t.name), t.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(t.name), t.id
LIMIT @row_limit;

-- name: CountItemTypes :one
SELECT count(*) FROM item_types
WHERE (@any_parent::boolean OR parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid)
  AND (@pattern::text = '' OR name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR deleted_at IS NULL);

-- name: ItemTypeChain :many
-- The type and its ancestors, root first. UNION stops on a cycle.
WITH RECURSIVE chain (id, parent_id, depth) AS (
    SELECT t.id, t.parent_id, 0 FROM item_types t WHERE t.id = @id::uuid
    UNION
    SELECT t.id, t.parent_id, c.depth + 1 FROM item_types t JOIN chain c ON t.id = c.parent_id
    WHERE c.depth < 1000
)
SELECT t.id, t.parent_id, t.name, t.tag_prefix, t.created_at, t.updated_at, t.deleted_at
FROM chain c JOIN item_types t ON t.id = c.id
ORDER BY c.depth DESC;

-- name: ItemTypeIsDescendant :one
-- Walks candidate's ancestors; UNION (not UNION ALL) stops on a cycle.
WITH RECURSIVE ancestors (id) AS (
    SELECT t.parent_id FROM item_types t WHERE t.id = @candidate::uuid AND t.parent_id IS NOT NULL
    UNION
    SELECT t.parent_id FROM item_types t JOIN ancestors a ON t.id = a.id WHERE t.parent_id IS NOT NULL
)
SELECT EXISTS (SELECT 1 FROM ancestors WHERE ancestors.id = @ancestor::uuid);

-- name: LockItemTypeTree :exec
-- Serializes item type moves until the transaction ends. The key is an
-- arbitrary constant reserved for the item type tree.
SELECT pg_advisory_xact_lock(7320414102);

-- name: GetAssetTagCounter :one
SELECT value FROM asset_tag_counters WHERE prefix = lower(@prefix::text);

-- name: SetAssetTagCounter :exec
INSERT INTO asset_tag_counters (prefix, value) VALUES (lower(@prefix::text), @value)
ON CONFLICT (prefix) DO UPDATE SET value = EXCLUDED.value;
