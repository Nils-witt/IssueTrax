-- name: ListTagsByProject :many
SELECT id, project_id, name, created_at, color FROM tags WHERE project_id = @project_id ORDER BY name;

-- name: GetTag :one
SELECT id, project_id, name, created_at, color FROM tags WHERE project_id = @project_id AND id = @id;

-- name: GetTags :many
SELECT id, project_id, name, created_at, color FROM tags WHERE id = ANY (@ids::uuid[]);

-- name: FindTagsByNames :many
SELECT id, project_id, name, created_at, color FROM tags WHERE project_id = @project_id AND name = ANY (@names::text[]);

-- name: InsertTagIfAbsent :many
-- Returns no row when the project already has a tag with that name.
INSERT INTO tags (id, project_id, name, created_at, color)
VALUES (@id, @project_id, @name, @created_at, @color)
ON CONFLICT (project_id, name) DO NOTHING
RETURNING id, project_id, name, created_at, color;

-- name: GetTagByName :one
SELECT id, project_id, name, created_at, color FROM tags WHERE project_id = @project_id AND name = @name;

-- name: UpdateTag :execrows
UPDATE tags SET name = @name, color = @color WHERE id = @id;
