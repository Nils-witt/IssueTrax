-- name: CreateManufacturer :exec
INSERT INTO manufacturers (id, name, created_at, updated_at, deleted_at)
VALUES (@id, @name, @created_at, @updated_at, @deleted_at);

-- name: UpdateManufacturer :execrows
UPDATE manufacturers SET name = @name, updated_at = @updated_at, deleted_at = @deleted_at WHERE id = @id;

-- name: GetManufacturer :one
SELECT m.id, m.name, m.created_at, m.updated_at, m.deleted_at,
       (SELECT count(*) FROM hardware_models hm WHERE hm.manufacturer_id = m.id AND hm.deleted_at IS NULL)::bigint AS model_count,
       (SELECT count(*) FROM module_hardware h
            JOIN storage_item_modules sm ON sm.id = h.module_id
            JOIN storage_items i ON i.id = sm.item_id
        WHERE h.manufacturer_id = m.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM manufacturers m
WHERE m.id = @id;

-- name: GetManufacturersByIDs :many
SELECT * FROM manufacturers WHERE id = ANY (@ids::uuid[]);

-- name: ListManufacturers :many
-- @pattern is a pre-escaped ILIKE pattern ('%term%'), empty for no search.
SELECT m.id, m.name, m.created_at, m.updated_at, m.deleted_at,
       (SELECT count(*) FROM hardware_models hm WHERE hm.manufacturer_id = m.id AND hm.deleted_at IS NULL)::bigint AS model_count,
       (SELECT count(*) FROM module_hardware h
            JOIN storage_item_modules sm ON sm.id = h.module_id
            JOIN storage_items i ON i.id = sm.item_id
        WHERE h.manufacturer_id = m.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM manufacturers m
WHERE (@pattern::text = '' OR m.name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR m.deleted_at IS NULL)
  AND (NOT @has_cursor::boolean OR (lower(m.name), m.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(m.name), m.id
LIMIT @row_limit;

-- name: CountManufacturers :one
SELECT count(*) FROM manufacturers
WHERE (@pattern::text = '' OR name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR deleted_at IS NULL);

-- name: CreateHardwareModel :exec
INSERT INTO hardware_models (id, manufacturer_id, name, created_at, updated_at, deleted_at)
VALUES (@id, @manufacturer_id, @name, @created_at, @updated_at, @deleted_at);

-- name: UpdateHardwareModel :execrows
-- A model never changes its manufacturer.
UPDATE hardware_models SET name = @name, updated_at = @updated_at, deleted_at = @deleted_at WHERE id = @id;

-- name: GetHardwareModel :one
SELECT hm.id, hm.manufacturer_id, hm.name, hm.created_at, hm.updated_at, hm.deleted_at,
       (SELECT count(*) FROM module_hardware h
            JOIN storage_item_modules sm ON sm.id = h.module_id
            JOIN storage_items i ON i.id = sm.item_id
        WHERE h.model_id = hm.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM hardware_models hm
WHERE hm.id = @id;

-- name: GetHardwareModelsByIDs :many
SELECT * FROM hardware_models WHERE id = ANY (@ids::uuid[]);

-- name: ListHardwareModels :many
-- One page of the models of @manufacturer_id. @pattern is a pre-escaped
-- ILIKE pattern ('%term%'), empty for no search.
SELECT hm.id, hm.manufacturer_id, hm.name, hm.created_at, hm.updated_at, hm.deleted_at,
       (SELECT count(*) FROM module_hardware h
            JOIN storage_item_modules sm ON sm.id = h.module_id
            JOIN storage_items i ON i.id = sm.item_id
        WHERE h.model_id = hm.id AND i.deleted_at IS NULL)::bigint AS item_count
FROM hardware_models hm
WHERE hm.manufacturer_id = @manufacturer_id
  AND (@pattern::text = '' OR hm.name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR hm.deleted_at IS NULL)
  AND (NOT @has_cursor::boolean OR (lower(hm.name), hm.id) > (@cursor_name::text, @cursor_id::uuid))
ORDER BY lower(hm.name), hm.id
LIMIT @row_limit;

-- name: CountHardwareModels :one
SELECT count(*) FROM hardware_models
WHERE manufacturer_id = @manufacturer_id
  AND (@pattern::text = '' OR name ILIKE @pattern::text)
  AND (@include_deleted::boolean OR deleted_at IS NULL);
