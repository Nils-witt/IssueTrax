-- name: AppendAuditEntry :exec
INSERT INTO audit_log (id, at, actor_id, action, target_type, target_id, target_label, changes, ip, user_agent)
VALUES (@id, @at, @actor_id, @action, @target_type, @target_id, @target_label, @changes, @ip, @user_agent);

-- The filter predicate below is shared verbatim by ListAuditEntries and
-- CountAuditEntries; keep them in sync.

-- name: ListAuditEntries :many
SELECT id, at, actor_id, action, target_type, target_id, target_label, changes, ip, user_agent
FROM audit_log
WHERE (NOT @filter_actor::boolean OR actor_id = @actor_id::uuid)
  AND (cardinality(@actions::text[]) = 0 OR action = ANY (@actions::text[]))
  AND (@target_type::text = '' OR target_type = @target_type::text)
  AND (NOT @has_cursor::boolean OR (at, id) < (@cursor_at::timestamptz, @cursor_id::uuid))
ORDER BY at DESC, id DESC
LIMIT @row_limit;

-- name: CountAuditEntries :one
SELECT count(*)
FROM audit_log
WHERE (NOT @filter_actor::boolean OR actor_id = @actor_id::uuid)
  AND (cardinality(@actions::text[]) = 0 OR action = ANY (@actions::text[]))
  AND (@target_type::text = '' OR target_type = @target_type::text);

-- name: DeleteAuditEntriesBefore :execrows
DELETE FROM audit_log WHERE at < @before;
