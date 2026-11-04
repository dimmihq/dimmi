-- name: CreateAttachment :one
-- CreateAttachment records a file stored on a disk. An upload has no
-- message until one is sent with it; a file that arrives with a message,
-- such as an inbound email's, names it at once.
INSERT INTO attachments (
    workspace_id, message_id, uploaded_by_type, uploaded_by_id, disk, path, filename, content_type, size_bytes,
    sha256, width, height, inline, content_id, scan_status, created_at, updated_at
) VALUES (
    @workspace_id, @message_id, @uploaded_by_type, @uploaded_by_id, @disk, @path, @filename, @content_type,
    @size_bytes, @sha256, @width, @height, @inline, @content_id, @scan_status, @now, @now
)
RETURNING *;

-- name: FindAttachment :one
-- FindAttachment returns the attachment with the id in the workspace.
SELECT * FROM attachments WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListAttachmentsByID :many
-- ListAttachmentsByID returns the workspace's attachments with the ids,
-- oldest first, so a caller linking uploads to a message can check each
-- one before linking. Ids of another workspace are simply missing.
SELECT * FROM attachments
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[])
ORDER BY created_at, id;

-- name: LinkAttachments :execrows
-- LinkAttachments attaches the uploads with the ids to the message and
-- returns how many it attached. An upload already attached to a message
-- is left alone, so a count short of the ids asked for means one was.
UPDATE attachments SET message_id = @message_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND message_id IS NULL;

-- name: ListMessageAttachments :many
-- ListMessageAttachments returns the attachments of the messages, by
-- message and then in the order they were added, so a page of a
-- timeline loads every attachment in one query.
SELECT * FROM attachments
WHERE workspace_id = @workspace_id AND message_id = ANY (@message_ids::uuid[])
ORDER BY message_id, created_at, id;

-- name: SetAttachmentScanStatus :one
-- SetAttachmentScanStatus records what the malware scan said about the
-- file.
UPDATE attachments SET scan_status = @scan_status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteUnattachedAttachment :execrows
-- DeleteUnattachedAttachment removes an upload no message uses yet and
-- reports whether it did. An attached file stays with its message.
DELETE FROM attachments WHERE workspace_id = @workspace_id AND id = @id AND message_id IS NULL;

-- name: ListUnattachedAttachments :many
-- ListUnattachedAttachments returns uploads of the workspace no message
-- used, created before the cutoff, oldest first, so a purge can remove
-- their files and then their rows.
SELECT * FROM attachments
WHERE workspace_id = @workspace_id AND message_id IS NULL AND created_at < @before
ORDER BY created_at, id
LIMIT @row_limit;
