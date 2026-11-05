-- name: CreateContactExport :one
INSERT INTO contact_exports (workspace_id, contact_id, requested_by_member_id, include_notes, created_at, updated_at)
VALUES (@workspace_id, @contact_id, @requested_by_member_id, @include_notes, @now, @now)
RETURNING *;

-- name: FindContactExport :one
SELECT * FROM contact_exports WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND id = @id;

-- name: FindContactExportByID :one
SELECT * FROM contact_exports WHERE workspace_id = @workspace_id AND id = @id;

-- name: StartContactExport :one
-- StartContactExport claims a pending export for a run.
UPDATE contact_exports SET status = 'running', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('pending', 'running')
RETURNING *;

-- name: FinishContactExport :one
UPDATE contact_exports
SET status = 'ready', disk = @disk, path = @path, size_bytes = @size_bytes, expires_at = @expires_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FailContactExport :exec
UPDATE contact_exports SET status = 'failed', error = @error, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListExpiredContactExports :many
-- ListExpiredContactExports returns which ready exports are past their
-- expiry, across workspaces, oldest first.
SELECT e.workspace_id::uuid AS workspace_id, e.id::uuid AS id
FROM contact_exports_expired(@now::timestamptz, @row_limit::integer) AS e;

-- name: ExpireContactExport :exec
UPDATE contact_exports SET status = 'expired', disk = NULL, path = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListExportConversations :many
-- ListExportConversations returns the contact's conversations, oldest
-- first.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND contact_id = @contact_id
ORDER BY created_at, id;

-- name: ListExportMessages :many
-- ListExportMessages returns a conversation's messages and, when asked,
-- its notes, oldest first, events left out.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type <> 'event'
  AND (type = 'message' OR @include_notes::boolean)
ORDER BY created_at, id;

-- name: ListExportAttachments :many
SELECT a.* FROM attachments a
JOIN messages m ON m.id = a.message_id AND m.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND m.conversation_id = @conversation_id
  AND (m.type = 'message' OR @include_notes::boolean)
ORDER BY a.created_at, a.id;
