-- name: CreateAttachment :exec
INSERT INTO attachments (id, issue_id, uploader_id, file_name, media_type, size, content, created_at)
VALUES (@id, @issue_id, @uploader_id, @file_name, @media_type, @size, @content, @created_at);

-- name: GetAttachment :one
SELECT id, issue_id, uploader_id, file_name, media_type, size, created_at
FROM attachments
WHERE issue_id = @issue_id AND id = @id;

-- name: GetAttachmentContent :one
SELECT id, issue_id, uploader_id, file_name, media_type, size, created_at, content
FROM attachments
WHERE issue_id = @issue_id AND id = @id;

-- name: ListAttachmentsByIssue :many
SELECT id, issue_id, uploader_id, file_name, media_type, size, created_at
FROM attachments
WHERE issue_id = @issue_id
ORDER BY created_at, id;

-- name: DeleteAttachment :execrows
DELETE FROM attachments WHERE id = @id;
