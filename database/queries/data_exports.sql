-- name: CreateDataExport :one
INSERT INTO data_exports (
    workspace_id, requested_by_member_id, kind, format, selection, include_notes, created_at, updated_at
) VALUES (
    @workspace_id, @requested_by_member_id, @kind, @format, @selection, @include_notes, @now, @now
)
RETURNING *;

-- name: FindDataExport :one
SELECT * FROM data_exports WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListMemberDataExports :many
-- ListMemberDataExports returns the member's exports, newest first.
SELECT * FROM data_exports
WHERE workspace_id = @workspace_id AND requested_by_member_id = @member_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::integer;

-- name: StartDataExport :one
-- StartDataExport claims a pending export for a run.
UPDATE data_exports SET status = 'running', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('pending', 'running')
RETURNING *;

-- name: FinishDataExport :one
UPDATE data_exports
SET status = 'ready', disk = @disk, path = @path, size_bytes = @size_bytes, row_count = @row_count,
    warnings = @warnings, expires_at = @expires_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FailDataExport :exec
UPDATE data_exports SET status = 'failed', error = @error, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteDataExport :exec
DELETE FROM data_exports WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListExpiredDataExports :many
-- ListExpiredDataExports returns which ready exports are past their
-- expiry, across workspaces, oldest first.
SELECT e.workspace_id::uuid AS workspace_id, e.id::uuid AS id
FROM data_exports_expired(@now::timestamptz, @row_limit::integer) AS e;

-- name: ExpireDataExport :exec
UPDATE data_exports SET status = 'expired', disk = NULL, path = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListDataExportMessages :many
-- ListDataExportMessages returns the messages and, when asked, the notes
-- of a page of conversations, each conversation's oldest first, events
-- and replies taken back before they left out.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@conversation_ids::uuid[]) AND type <> 'event'
  AND undone_at IS NULL AND deleted_at IS NULL
  AND (type = 'message' OR @include_notes::boolean)
ORDER BY conversation_id, created_at, id;

-- name: ExportContactsByIDs :many
-- ExportContactsByIDs returns the live, unmerged contacts among the ids
-- that nobody asked to erase, most recently seen first.
SELECT c.* FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.id = ANY (@ids::uuid[])
  AND c.merged_into_id IS NULL AND c.deleted_at IS NULL
ORDER BY coalesce(c.last_seen_at, c.created_at) DESC, c.id DESC;
