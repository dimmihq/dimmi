-- name: CreateInboundEmail :one
-- CreateInboundEmail records a received email for the workspace. The
-- same provider message arriving again inserts nothing and returns no
-- row, so a redelivery is never processed twice.
INSERT INTO inbound_emails (
    workspace_id, email_address_id, channel_connection_id, source, provider_message_id, envelope_from,
    envelope_to, size_bytes, raw_disk, raw_path, status, ignore_reason, provider_verdicts, received_at,
    processed_at, created_at, updated_at
) VALUES (
    @workspace_id, @email_address_id, @channel_connection_id, @source, @provider_message_id, @envelope_from,
    @envelope_to, @size_bytes, @raw_disk, @raw_path, @status, @ignore_reason, @provider_verdicts, @received_at,
    sqlc.narg(processed_at), @now::timestamptz, @now::timestamptz
)
ON CONFLICT (workspace_id, source, provider_message_id) WHERE provider_message_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: FindInboundEmail :one
-- FindInboundEmail returns the received email with the id.
SELECT * FROM inbound_emails WHERE workspace_id = @workspace_id AND id = @id;

-- name: StartInboundEmailProcessing :one
-- StartInboundEmailProcessing counts one more attempt at processing the
-- email and marks it in progress.
UPDATE inbound_emails SET status = 'processing', attempts = attempts + 1, error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DescribeInboundEmail :exec
-- DescribeInboundEmail records what parsing and checking learned about
-- the email: its headers, authentication and classification.
UPDATE inbound_emails
SET message_id_header = @message_id_header, from_address = @from_address, subject = @subject,
    classification = @classification, auth = @auth, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishInboundEmail :exec
-- FinishInboundEmail records how processing ended: the message the
-- email became, or why it was ignored or set aside.
UPDATE inbound_emails
SET status = @status, ignore_reason = sqlc.narg(ignore_reason), conversation_id = sqlc.narg(conversation_id),
    message_id = sqlc.narg(message_id), processed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailInboundEmail :exec
-- FailInboundEmail records why processing gave up on the email, without
-- any of its content.
UPDATE inbound_emails SET status = 'failed', error = @error, processed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListAddressInboundEmails :many
-- ListAddressInboundEmails returns a page of what arrived at the
-- address, newest first. A page after the first passes the received_at
-- and id of the last email it saw.
SELECT * FROM inbound_emails
WHERE workspace_id = @workspace_id
  AND email_address_id = @email_address_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (received_at, id) < (sqlc.narg(before_received_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
ORDER BY received_at DESC, id DESC
LIMIT @row_limit::bigint;

-- name: FindMessageInboundEmail :one
-- FindMessageInboundEmail returns the received email a message came from.
SELECT * FROM inbound_emails WHERE workspace_id = @workspace_id AND message_id = @message_id LIMIT 1;
