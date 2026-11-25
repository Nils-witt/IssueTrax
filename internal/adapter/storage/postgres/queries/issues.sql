-- name: CreateIssue :exec
INSERT INTO issues (id, project_id, number, title, description, status, created_by, created_at, updated_at)
VALUES (@id, @project_id, @number, @title, @description, @status, @created_by, @created_at, @updated_at);

-- name: UpdateIssue :execrows
UPDATE issues
SET title = @title, description = @description, status = @status, updated_at = @updated_at
WHERE id = @id;

-- name: GetIssue :one
SELECT id, project_id, number, title, description, status, created_by, created_at, updated_at
FROM issues
WHERE project_id = @project_id AND number = @number;

-- name: GetIssueForUpdate :one
-- Locks the row so concurrent updates of one issue apply one after another.
SELECT id, project_id, number, title, description, status, created_by, created_at, updated_at
FROM issues
WHERE project_id = @project_id AND number = @number
FOR UPDATE;

-- The filter predicate below is shared verbatim by ListIssues and
-- CountIssues; keep them in sync. Match modes: an issue matches "all" when
-- it carries every requested tag/assignee, "any" when it carries one.
-- @pattern is a pre-escaped ILIKE pattern ('%term%'), empty for no search.

-- name: ListIssues :many
SELECT i.id, i.project_id, i.number, i.title, i.description, i.status, i.created_by, i.created_at, i.updated_at
FROM issues i
WHERE i.project_id = @project_id
  AND i.status = ANY (@statuses::issue_status[])
  AND (cardinality(@tag_ids::uuid[]) = 0 OR (
        SELECT CASE WHEN @tags_all::boolean THEN count(*) = cardinality(@tag_ids::uuid[]) ELSE count(*) > 0 END
        FROM issue_tags it WHERE it.issue_id = i.id AND it.tag_id = ANY (@tag_ids::uuid[])))
  AND (cardinality(@assignee_ids::uuid[]) = 0 OR (
        SELECT CASE WHEN @assignees_all::boolean THEN count(*) = cardinality(@assignee_ids::uuid[]) ELSE count(*) > 0 END
        FROM issue_assignees ia WHERE ia.issue_id = i.id AND ia.user_id = ANY (@assignee_ids::uuid[])))
  AND (@pattern::text = '' OR i.title ILIKE @pattern::text OR i.description ILIKE @pattern::text)
  AND (@before_number::bigint = 0 OR i.number < @before_number::bigint)
ORDER BY i.number DESC
LIMIT @row_limit;

-- name: CountIssues :one
SELECT count(*)
FROM issues i
WHERE i.project_id = @project_id
  AND i.status = ANY (@statuses::issue_status[])
  AND (cardinality(@tag_ids::uuid[]) = 0 OR (
        SELECT CASE WHEN @tags_all::boolean THEN count(*) = cardinality(@tag_ids::uuid[]) ELSE count(*) > 0 END
        FROM issue_tags it WHERE it.issue_id = i.id AND it.tag_id = ANY (@tag_ids::uuid[])))
  AND (cardinality(@assignee_ids::uuid[]) = 0 OR (
        SELECT CASE WHEN @assignees_all::boolean THEN count(*) = cardinality(@assignee_ids::uuid[]) ELSE count(*) > 0 END
        FROM issue_assignees ia WHERE ia.issue_id = i.id AND ia.user_id = ANY (@assignee_ids::uuid[])))
  AND (@pattern::text = '' OR i.title ILIKE @pattern::text OR i.description ILIKE @pattern::text);

-- name: ListIssueTags :many
SELECT issue_id, tag_id FROM issue_tags WHERE issue_id = ANY (@issue_ids::uuid[]);

-- name: ListIssueAssignees :many
SELECT issue_id, user_id FROM issue_assignees WHERE issue_id = ANY (@issue_ids::uuid[]);

-- name: DeleteIssueTags :exec
DELETE FROM issue_tags WHERE issue_id = @issue_id;

-- name: InsertIssueTags :exec
INSERT INTO issue_tags (issue_id, tag_id)
SELECT @issue_id, unnest(@tag_ids::uuid[]);

-- name: DeleteIssueAssignees :exec
DELETE FROM issue_assignees WHERE issue_id = @issue_id;

-- name: InsertIssueAssignees :exec
INSERT INTO issue_assignees (issue_id, user_id)
SELECT @issue_id, unnest(@user_ids::uuid[]);
