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

-- name: ListTrashedConversationsBefore :many
-- ListTrashedConversationsBefore returns conversations in the trash since
-- before the cutoff, oldest first.
SELECT id FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at < @cutoff::timestamptz
  AND NOT (id = ANY (@skipped::uuid[]))
ORDER BY deleted_at, id
LIMIT @row_limit;

-- name: ListSpamConversationsBefore :many
-- ListSpamConversationsBefore returns conversations marked spam since
-- before the cutoff: since the last time they were marked, or since
-- they arrived when they arrived as spam.
SELECT c.id FROM conversations c
WHERE c.workspace_id = @workspace_id AND c.spam AND c.deleted_at IS NULL
  AND coalesce((
      SELECT max(e.occurred_at) FROM conversation_events e
      WHERE e.conversation_id = c.id AND e.kind = 'spam'
  ), c.created_at) < @cutoff::timestamptz
  AND NOT (c.id = ANY (@skipped::uuid[]))
ORDER BY c.id
LIMIT @row_limit;

-- name: ListConversationFiles :many
-- ListConversationFiles returns the stored files of the conversations'
-- messages.
SELECT a.id, a.disk, a.path FROM attachments a
JOIN messages m ON m.id = a.message_id AND m.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND m.conversation_id = ANY (@ids::uuid[]);

-- name: ListConversationRawEmails :many
-- ListConversationRawEmails returns the stored source of the mail the
-- conversations received.
SELECT id, raw_disk, raw_path FROM inbound_emails
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[]) AND raw_path IS NOT NULL;

-- name: DeleteConversationInboundEmails :execrows
DELETE FROM inbound_emails WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[]);

-- name: DeleteConversationOutboundEmails :execrows
DELETE FROM outbound_emails WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[]);

-- name: PurgeConversations :execrows
-- PurgeConversations deletes the conversations for good, and with them
-- their messages, files' rows and everything else that hangs off them.
DELETE FROM conversations WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: ListExpiredRawEmails :many
-- ListExpiredRawEmails returns mail read before the cutoff that still has
-- its source stored or never became a message, oldest first.
SELECT id, raw_disk, raw_path, message_id FROM inbound_emails
WHERE workspace_id = @workspace_id AND status IN ('processed', 'ignored', 'failed', 'quarantined')
  AND coalesce(processed_at, received_at) < @cutoff::timestamptz
  AND (raw_path IS NOT NULL OR message_id IS NULL)
ORDER BY received_at, id
LIMIT @row_limit;

-- name: ClearRawEmails :execrows
-- ClearRawEmails records that the mail's source is gone. The row stays
-- for the message it became.
UPDATE inbound_emails SET raw_path = NULL, raw_deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: DeleteInboundEmails :execrows
-- DeleteInboundEmails deletes the records of mail that never became a
-- message.
DELETE FROM inbound_emails WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND message_id IS NULL;

-- name: ListWorkspaceRawEmails :many
-- ListWorkspaceRawEmails returns a batch of the workspace's stored mail
-- sources.
SELECT id, raw_disk, raw_path FROM inbound_emails
WHERE workspace_id = @workspace_id AND raw_path IS NOT NULL
ORDER BY id
LIMIT @row_limit;

-- name: ListHelpCenterImages :many
-- ListHelpCenterImages returns a batch of the pictures the workspace
-- placed in articles.
SELECT id, path FROM help_center_images WHERE workspace_id = @workspace_id ORDER BY id LIMIT @row_limit;

-- name: DeleteHelpCenterImages :execrows
DELETE FROM help_center_images WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: RecordHelpCenterImage :exec
-- RecordHelpCenterImage notes which workspace kept the picture.
INSERT INTO help_center_images (workspace_id, path, created_at) VALUES (@workspace_id, @path, @now::timestamptz)
ON CONFLICT (path) DO NOTHING;

-- name: RawEmailShared :one
-- RawEmailShared reports whether a record other than the excepted ones,
-- in any workspace, still points at the stored email.
SELECT dimmi_raw_email_shared(@raw_disk::text, @raw_path::text, @except_ids::uuid[])::boolean;
