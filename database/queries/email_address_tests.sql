-- name: CreateEmailAddressTest :one
-- CreateEmailAddressTest stores a forwarding test about to be sent.
INSERT INTO email_address_tests (
    workspace_id, email_address_id, token, expires_at, created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @email_address_id, @token, @expires_at::timestamptz, sqlc.narg(created_by_member_id),
    @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindEmailAddressTest :one
-- FindEmailAddressTest returns the test of the address with the id and
-- the Message-ID of the mail it sent, empty before it was sent.
SELECT sqlc.embed(t), coalesce(o.message_id_header, '')::text AS message_id
FROM email_address_tests AS t
LEFT JOIN outbound_emails AS o ON o.id = t.outbound_email_id AND o.workspace_id = t.workspace_id
WHERE t.workspace_id = @workspace_id AND t.email_address_id = @email_address_id AND t.id = @id;

-- name: FindEmailAddressTestByID :one
-- FindEmailAddressTestByID returns the workspace's test with the id.
SELECT *
FROM email_address_tests
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindEmailAddressTestByToken :one
-- FindEmailAddressTestByToken returns the address's test carrying the
-- token, locked so that two copies of one test mail settle it once.
SELECT *
FROM email_address_tests
WHERE workspace_id = @workspace_id AND email_address_id = @email_address_id AND token = @token
FOR UPDATE;

-- name: FindEmailAddressTestByOutbound :one
-- FindEmailAddressTestByOutbound returns the test the sent email carried.
SELECT *
FROM email_address_tests
WHERE workspace_id = @workspace_id AND outbound_email_id = @outbound_email_id
FOR UPDATE;

-- name: MarkEmailAddressTestWaiting :one
-- MarkEmailAddressTestWaiting records that the test mail left, unless the
-- test settled already.
UPDATE email_address_tests
SET status = 'waiting', outbound_email_id = sqlc.narg(outbound_email_id), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status = 'sending'
RETURNING *;

-- name: SettleEmailAddressTest :one
-- SettleEmailAddressTest ends a test still sending or waiting.
UPDATE email_address_tests
SET status = @status, inbound_email_id = sqlc.narg(inbound_email_id), findings = @findings,
    latency_ms = sqlc.narg(latency_ms), completed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('sending', 'waiting')
RETURNING *;

-- name: PruneEmailAddressTests :one
-- PruneEmailAddressTests deletes every workspace's tests created before
-- the time and returns how many went.
SELECT prune_email_address_tests(@before::timestamptz)::bigint AS pruned;
