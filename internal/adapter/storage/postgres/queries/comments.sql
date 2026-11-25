-- name: CreateComment :exec
INSERT INTO comments (id, issue_id, author_id, body, created_at, updated_at)
VALUES (@id, @issue_id, @author_id, @body, @created_at, @updated_at);

-- name: GetComment :one
SELECT id, issue_id, author_id, body, created_at, updated_at
FROM comments
WHERE issue_id = @issue_id AND id = @id;

-- name: ListCommentsByIssue :many
SELECT id, issue_id, author_id, body, created_at, updated_at
FROM comments
WHERE issue_id = @issue_id
ORDER BY created_at, id;

-- name: UpdateComment :execrows
UPDATE comments SET body = @body, updated_at = @updated_at WHERE id = @id;

-- name: DeleteComment :execrows
DELETE FROM comments WHERE id = @id;
