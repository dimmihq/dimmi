-- name: CreateChannelInboundEvent :one
-- CreateChannelInboundEvent keeps a verified delivery, for its connection
-- or, without one, its installation. One the provider delivered before
-- returns no row.
INSERT INTO channel_inbound_events (
    workspace_id, channel_connection_id, integration_installation_id, provider, transport, dedupe_key, serial_key,
    kind, payload, status, received_at, created_at, updated_at
)
VALUES (
    @workspace_id, sqlc.narg(channel_connection_id), sqlc.narg(integration_installation_id), @provider, @transport, @dedupe_key, sqlc.narg(serial_key), @kind,
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

-- name: PruneChannelInboundEvents :execrows
-- PruneChannelInboundEvents deletes up to a batch of the workspace's
-- deliveries in one of the statuses received before the cutoff.
DELETE FROM channel_inbound_events WHERE id IN (
    SELECT e.id FROM channel_inbound_events AS e
    WHERE e.workspace_id = @workspace_id AND e.status = ANY (@statuses::text[])
        AND e.received_at < @cutoff::timestamptz
    ORDER BY e.received_at LIMIT @batch::bigint
);

-- name: ReleaseHeldChannelInboundEvents :many
-- ReleaseHeldChannelInboundEvents sets up to a batch of the workspace's
-- held deliveries back to pending, oldest first, and returns their ids
-- in the order they were received. A workspace still suspended releases
-- nothing.
WITH released AS (
    UPDATE channel_inbound_events SET status = 'pending', updated_at = @now::timestamptz
    WHERE id IN (
        SELECT e.id FROM channel_inbound_events AS e
        WHERE e.workspace_id = @workspace_id AND e.status = 'held'
            AND EXISTS (SELECT 1 FROM workspaces AS w WHERE w.id = @workspace_id AND w.suspended_at IS NULL)
        ORDER BY e.received_at, e.id LIMIT @batch::bigint
        FOR UPDATE
    )
    RETURNING id, received_at
)
SELECT id FROM released ORDER BY received_at, id;

-- name: EraseChannelInboundEventsMentioning :execrows
-- EraseChannelInboundEventsMentioning deletes the workspace's deliveries
-- from the provider, received up to the time, whose payload holds one of
-- the values as a whole JSON string.
DELETE FROM channel_inbound_events AS e
WHERE e.workspace_id = @workspace_id AND e.provider = @provider AND e.received_at <= @before::timestamptz
    AND EXISTS (
        SELECT 1 FROM unnest(@needles::text[]) AS n(value)
        WHERE n.value <> '' AND strpos(e.payload::text, to_jsonb(n.value)::text) > 0
    );
