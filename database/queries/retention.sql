-- name: ListRetainingWorkspaces :many
-- ListRetainingWorkspaces returns the live workspaces that set a period
-- for closed conversations.
SELECT * FROM workspaces
WHERE deleted_at IS NULL AND settings -> 'retention' ->> 'closed_conversation_days' IS NOT NULL
ORDER BY id;

-- name: ListStaleClosedConversations :many
-- ListStaleClosedConversations returns closed conversations with nothing
-- new since the cutoff, oldest first.
SELECT id FROM conversations
WHERE workspace_id = @workspace_id AND status = 'closed' AND deleted_at IS NULL
  AND closed_at < @cutoff::timestamptz AND coalesce(last_activity_at, closed_at) < @cutoff::timestamptz
  AND NOT (id = ANY (@skipped::uuid[]))
ORDER BY closed_at, id
LIMIT @row_limit;

-- name: CountStaleClosedConversations :one
SELECT count(*) FROM conversations
WHERE workspace_id = @workspace_id AND status = 'closed' AND deleted_at IS NULL
  AND closed_at < @cutoff::timestamptz AND coalesce(last_activity_at, closed_at) < @cutoff::timestamptz;

-- name: TrashStaleConversations :execrows
UPDATE conversations SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;
