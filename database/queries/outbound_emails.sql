-- name: CreateOutboundEmail :one
-- CreateOutboundEmail records an email about to be handed to the
-- provider, with the Message-ID it carries. An idempotency key the
-- workspace already used inserts nothing and returns no row.
INSERT INTO outbound_emails (
    workspace_id, message_id, conversation_id, email_address_id, kind, from_address, to_addresses, cc_addresses,
    bcc_addresses, subject, message_id_header, transport, status, metadata, created_at, updated_at
) VALUES (
    @workspace_id, sqlc.narg(message_id), sqlc.narg(conversation_id), sqlc.narg(email_address_id), @kind,
    @from_address, @to_addresses, @cc_addresses, @bcc_addresses, @subject, @message_id_header, @transport, 'queued',
    @metadata, @now::timestamptz, @now::timestamptz
)
ON CONFLICT (workspace_id, (metadata ->> 'idempotency_key')) WHERE metadata ->> 'idempotency_key' IS NOT NULL
DO NOTHING
RETURNING *;

-- name: FindOutboundEmailByIdempotencyKey :one
-- FindOutboundEmailByIdempotencyKey returns the email recorded for the
-- idempotency key.
SELECT * FROM outbound_emails WHERE workspace_id = @workspace_id AND metadata ->> 'idempotency_key' = @idempotency_key::text;

-- name: FindOutboundEmailByMessageIDHeader :one
-- FindOutboundEmailByMessageIDHeader returns the newest email the
-- workspace sent with the Message-ID.
SELECT * FROM outbound_emails WHERE workspace_id = @workspace_id AND message_id_header = @message_id_header
ORDER BY created_at DESC
LIMIT 1;

-- name: StartOutboundEmailAttempt :exec
-- StartOutboundEmailAttempt counts one more attempt at handing the
-- email over.
UPDATE outbound_emails SET attempts = attempts + 1, updated_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkOutboundEmailSent :one
-- MarkOutboundEmailSent records that the provider accepted the email.
UPDATE outbound_emails
SET status = 'sent', provider_message_id = sqlc.narg(provider_message_id), size_bytes = @size_bytes, sent_at = @now::timestamptz,
    last_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MarkOutboundEmailFailed :exec
-- MarkOutboundEmailFailed records why the email will never be sent.
UPDATE outbound_emails SET status = @status, last_error = @last_error, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: NoteOutboundEmailError :exec
-- NoteOutboundEmailError keeps the last error of an attempt that will
-- be retried.
UPDATE outbound_emails SET last_error = @last_error, updated_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordOutboundRecipientStatus :one
-- RecordOutboundRecipientStatus stores what became of one recipient,
-- and the status of the whole email when it changes.
UPDATE outbound_emails
SET recipient_statuses = recipient_statuses || jsonb_build_object(@recipient::text, @recipient_status::jsonb),
    status = @status, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindQuotableMessage :one
-- FindQuotableMessage returns the customer's latest message in the
-- conversation, with who wrote it, for quoting below a reply.
SELECT m.body, m.body_format, m.created_at, c.name, c.email
FROM messages AS m
JOIN contacts AS c ON c.id = m.author_id AND c.workspace_id = m.workspace_id
WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id AND m.type = 'message'
  AND m.author_type = 'contact' AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT 1;
