-- name: CreateChannelInboundEvent :one
-- CreateChannelInboundEvent keeps a verified delivery. One the provider
-- delivered before returns no row.
INSERT INTO channel_inbound_events (
    workspace_id, channel_connection_id, provider, transport, dedupe_key, serial_key, kind, payload, status,
    received_at, created_at, updated_at
)
VALUES (
    @workspace_id, @channel_connection_id, @provider, @transport, @dedupe_key, sqlc.narg(serial_key), @kind,
    @payload, @status, @now::timestamptz, @now::timestamptz, @now::timestamptz
)
ON CONFLICT (workspace_id, provider, dedupe_key) DO NOTHING
RETURNING *;

-- name: StartChannelInboundEvent :one
-- StartChannelInboundEvent claims a delivery for processing, counting
-- the attempt. A delivery already finished returns no row.
UPDATE channel_inbound_events
SET attempts = attempts + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('pending', 'failed')
RETURNING *;

-- name: FinishChannelInboundEvent :exec
-- FinishChannelInboundEvent records how processing a delivery ended.
UPDATE channel_inbound_events
SET status = @status, error = sqlc.narg(error), processed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindChannelInboundEvent :one
-- FindChannelInboundEvent returns the delivery with the id.
SELECT * FROM channel_inbound_events WHERE workspace_id = @workspace_id AND id = @id;
