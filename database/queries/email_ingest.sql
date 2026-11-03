-- name: FindEmailSenderMember :one
-- FindEmailSenderMember returns the active member of the workspace whose
-- verified account address is the sender's, with their name. An
-- unverified account address proves nothing and finds nobody.
SELECT m.id, u.name FROM workspace_members AS m
JOIN users AS u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.status = 'active' AND u.email = @email
  AND u.email_verified_at IS NOT NULL AND u.deleted_at IS NULL;

-- name: FindLatestThreadConversation :one
-- FindLatestThreadConversation returns the newest live conversation of
-- the connection with the thread key, whatever its status.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_thread_key = @external_thread_key AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: RecordEmailConversationMessage :one
-- RecordEmailConversationMessage counts a new customer message on the
-- conversation. A substantive one wakes a snoozed or closed
-- conversation when asked to and counts as the customer's latest word;
-- an automatic one only moves the timeline.
UPDATE conversations
SET status = CASE WHEN @wake::boolean THEN 'open' ELSE status END,
    snoozed_until = CASE WHEN @wake::boolean THEN NULL ELSE snoozed_until END,
    closed_at = CASE WHEN @wake::boolean THEN NULL ELSE closed_at END,
    first_customer_message_at = CASE
        WHEN @substantive::boolean THEN coalesce(first_customer_message_at, @now::timestamptz) ELSE first_customer_message_at
    END,
    last_customer_message_at = CASE WHEN @substantive::boolean THEN @now::timestamptz ELSE last_customer_message_at END,
    last_message_at = @now::timestamptz, message_count = message_count + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: VerifyEmailContactIdentity :exec
-- VerifyEmailContactIdentity marks the contact's address as proven by
-- an authenticated email.
WITH identity AS (
    UPDATE contact_identities SET verified = true, updated_at = @now::timestamptz
    WHERE contact_identities.workspace_id = @workspace_id AND contact_identities.contact_id = @contact_id
      AND contact_identities.type = 'email' AND contact_identities.value = @email
)
UPDATE contacts SET email_verified = true, updated_at = @now::timestamptz
WHERE contacts.workspace_id = @workspace_id AND contacts.id = @contact_id AND contacts.email = @email;

-- name: SeeEmailContact :exec
-- SeeEmailContact notes that the contact wrote, and takes their display
-- name only when the contact has none.
UPDATE contacts
SET name = coalesce(nullif(name, ''), sqlc.narg(name)), last_seen_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateEmailAttachment :one
-- CreateEmailAttachment records a file that arrived with an email,
-- stored on a disk and waiting for its scan.
INSERT INTO attachments (
    workspace_id, message_id, uploaded_by_type, uploaded_by_id, disk, path, filename, content_type, size_bytes,
    sha256, inline, content_id, scan_status, created_at, updated_at
) VALUES (
    @workspace_id, @message_id, 'contact', @uploaded_by_id, @disk, @path, @filename, @content_type, @size_bytes,
    @sha256, @inline, sqlc.narg(content_id), @scan_status, @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FailEmailMessageDelivery :exec
-- FailEmailMessageDelivery marks a sent reply as not delivered, unless
-- the customer already read it.
UPDATE messages SET delivery_status = 'failed', delivery_error = @delivery_error, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND delivery_status IS DISTINCT FROM 'read';

-- name: FindMessageByDedupeKey :one
-- FindMessageByDedupeKey returns the message the workspace already
-- accepted under the dedupe key.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND dedupe_key = @dedupe_key;

-- name: MarkEmailConversationSpam :exec
-- MarkEmailConversationSpam sets a new conversation aside as spam.
UPDATE conversations SET spam = true, updated_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: CloseEmailConversation :exec
-- CloseEmailConversation closes a conversation nobody needs to answer,
-- such as one a newsletter started.
UPDATE conversations SET status = 'closed', snoozed_until = NULL, closed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: AddEmailMessageMetadata :exec
-- AddEmailMessageMetadata merges keys into a message's metadata.
UPDATE messages SET metadata = metadata || @metadata::jsonb, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindEmailMessage :one
-- FindEmailMessage returns the message with the id, deleted or not.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND id = @id;
