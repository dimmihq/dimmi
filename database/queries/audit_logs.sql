-- name: CreateAuditLog :one
-- CreateAuditLog appends an entry to the workspace's audit log.
INSERT INTO audit_logs (
    workspace_id, event, actor_type, actor_id, via_kind, via_id, actor_label, subject_type, subject_id,
    subject_label, changes, ip, user_agent, request_id, created_at
) VALUES (
    @workspace_id, @event, @actor_type, @actor_id, @via_kind, @via_id, @actor_label, @subject_type, @subject_id,
    @subject_label, @changes, @ip, @user_agent, @request_id, @now
)
RETURNING *;

-- name: ListAuditLogs :many
-- ListAuditLogs returns a page of the workspace's audit log, newest first,
-- after the cursor position when one is given, narrowed to the events,
-- actor, channel, subject and time range asked for. Every narrowing is
-- optional: an empty list or a null value leaves it out.
SELECT * FROM audit_logs
WHERE workspace_id = @workspace_id
  AND (coalesce(cardinality(@events::text[]), 0) = 0 OR event = ANY (@events::text[]))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(via_id)::uuid IS NULL OR via_id = sqlc.narg(via_id)::uuid)
  AND (sqlc.narg(subject_id)::uuid IS NULL OR subject_id = sqlc.narg(subject_id)::uuid)
  AND (sqlc.narg(created_after)::timestamptz IS NULL OR created_at >= sqlc.narg(created_after)::timestamptz)
  AND (sqlc.narg(created_before)::timestamptz IS NULL OR created_at < sqlc.narg(created_before)::timestamptz)
  AND (
      sqlc.narg(cursor_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::bigint;

-- name: ListAuditLogsAscending :many
-- ListAuditLogsAscending returns a page of the workspace's audit log,
-- oldest first, for programs that follow the log as it grows. It takes
-- the same narrowings as ListAuditLogs.
SELECT * FROM audit_logs
WHERE workspace_id = @workspace_id
  AND (coalesce(cardinality(@events::text[]), 0) = 0 OR event = ANY (@events::text[]))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(via_id)::uuid IS NULL OR via_id = sqlc.narg(via_id)::uuid)
  AND (sqlc.narg(subject_id)::uuid IS NULL OR subject_id = sqlc.narg(subject_id)::uuid)
  AND (sqlc.narg(created_after)::timestamptz IS NULL OR created_at >= sqlc.narg(created_after)::timestamptz)
  AND (sqlc.narg(created_before)::timestamptz IS NULL OR created_at < sqlc.narg(created_before)::timestamptz)
  AND (
      sqlc.narg(cursor_at)::timestamptz IS NULL
      OR (created_at, id) > (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
ORDER BY created_at, id
LIMIT @row_limit::bigint;

-- name: FindAuditLog :one
-- FindAuditLog returns one entry of the workspace's audit log.
SELECT * FROM audit_logs WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindAuditActor :one
-- FindAuditActor returns the member with the id in the workspace and the
-- name and email of their account, to label an audit entry.
SELECT m.id, u.name, u.email
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.id = @id;

-- name: FindAuditActorByUser :one
-- FindAuditActorByUser returns the user's membership of the workspace and
-- the name and email of their account, to label an audit entry.
SELECT m.id, u.name, u.email
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.user_id = @user_id;

-- name: FindAuditToken :one
-- FindAuditToken returns the name and hint of the access token, to label
-- an audit entry made through it.
SELECT name, hint FROM personal_access_tokens WHERE id = @id;

-- name: ListAuditLogWorkspacesBefore :many
-- ListAuditLogWorkspacesBefore returns the workspaces that have audit
-- entries written before the cutoff.
SELECT DISTINCT workspace_id FROM audit_logs WHERE created_at < @cutoff;

-- name: PruneAuditLogs :execrows
-- PruneAuditLogs deletes up to a batch of audit entries of the
-- workspaces, written before the cutoff, oldest first.
DELETE FROM audit_logs
WHERE id IN (
    SELECT a.id FROM audit_logs AS a
    WHERE a.created_at < @cutoff AND a.workspace_id = ANY (@workspace_ids::uuid[])
    ORDER BY a.created_at LIMIT @batch::bigint
);
