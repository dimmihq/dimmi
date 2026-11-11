-- name: NextWebhookSequence :one
-- NextWebhookSequence returns the next ordering number of snapshots.
SELECT nextval('webhook_event_sequence')::bigint AS sequence;

-- name: LockWebhookObject :exec
-- LockWebhookObject holds one object's snapshots in order until the transaction ends.
SELECT pg_advisory_xact_lock(hashtextextended(@object_key::text, 0));

-- name: CreateWebhookDelivery :one
-- CreateWebhookDelivery stores one event for one endpoint, once.
INSERT INTO webhook_deliveries (
    workspace_id, endpoint_id, event_id, event_type, sequence, object_id, contact_id, payload, payload_bytes,
    thin, test, note, status, next_attempt_at, created_at, updated_at
) VALUES (
    @workspace_id, @endpoint_id, @event_id, @event_type, @sequence, sqlc.narg(object_id), sqlc.narg(contact_id),
    @payload, @payload_bytes, @thin, @test, @note, 'pending', @now::timestamptz, @now, @now
)
ON CONFLICT (endpoint_id, event_id) DO NOTHING
RETURNING *;

-- name: FindWebhookDelivery :one
-- FindWebhookDelivery returns one delivery of the workspace.
SELECT * FROM webhook_deliveries
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockWebhookDelivery :one
-- LockWebhookDelivery returns one delivery, locked until the transaction ends.
SELECT * FROM webhook_deliveries
WHERE workspace_id = @workspace_id AND id = @id
FOR UPDATE;

-- name: ListWebhookDeliveries :many
-- ListWebhookDeliveries returns an endpoint's deliveries, newest first, after the cursor.
SELECT id, workspace_id, endpoint_id, event_id, event_type, sequence, payload_bytes, thin, test, status,
    attempt_count, next_attempt_at, last_attempt_at, succeeded_at, last_response_status, last_error_code,
    created_at, updated_at
FROM webhook_deliveries
WHERE workspace_id = @workspace_id AND endpoint_id = @endpoint_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(event_type)::text IS NULL OR event_type = sqlc.narg(event_type))
  AND (sqlc.narg(event_id)::uuid IS NULL OR event_id = sqlc.narg(event_id))
  AND (sqlc.narg(before_id)::uuid IS NULL OR (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)))
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: SaveWebhookDeliveryOutcome :one
-- SaveWebhookDeliveryOutcome records what the latest attempt left the delivery as.
UPDATE webhook_deliveries
SET status = @status, attempt_count = @attempt_count, next_attempt_at = sqlc.narg(next_attempt_at),
    first_attempt_at = coalesce(first_attempt_at, @attempted_at::timestamptz), last_attempt_at = @attempted_at::timestamptz,
    succeeded_at = sqlc.narg(succeeded_at), last_response_status = sqlc.narg(last_response_status),
    last_error_code = sqlc.narg(last_error_code), updated_at = @attempted_at::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CancelWebhookDeliveries :execrows
-- CancelWebhookDeliveries cancels an endpoint's deliveries still waiting to be sent.
UPDATE webhook_deliveries
SET status = 'canceled', next_attempt_at = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND endpoint_id = @endpoint_id AND status IN ('pending', 'retrying');

-- name: CancelWebhookNoteDeliveries :execrows
-- CancelWebhookNoteDeliveries cancels an endpoint's waiting deliveries of internal notes.
UPDATE webhook_deliveries
SET status = 'canceled', next_attempt_at = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND endpoint_id = @endpoint_id AND note AND status IN ('pending', 'retrying');

-- name: ListDueWebhookDeliveries :many
-- ListDueWebhookDeliveries returns the deliveries of any workspace due another attempt.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM webhook_deliveries_due(@due_at::timestamptz, @row_limit::integer) AS d;

-- name: CountRecentWebhookFailures :one
-- CountRecentWebhookFailures counts an endpoint's failed attempts since the time.
SELECT count(*) FROM webhook_delivery_attempts a
JOIN webhook_deliveries d ON d.id = a.delivery_id
WHERE d.workspace_id = @workspace_id::uuid AND d.endpoint_id = @endpoint_id::uuid AND a.trigger <> 'test'
  AND a.requested_at >= @since::timestamptz AND (a.response_status IS NULL OR a.response_status NOT BETWEEN 200 AND 299);

-- name: PruneWebhookDeliveries :execrows
-- PruneWebhookDeliveries deletes a batch of the workspace's deliveries older than the time.
DELETE FROM webhook_deliveries
WHERE id IN (
    SELECT old.id FROM webhook_deliveries old
    WHERE old.workspace_id = @workspace_id::uuid AND old.created_at < @before::timestamptz
    LIMIT @row_limit::integer
);

-- name: DeleteWebhookContactDeliveries :execrows
-- DeleteWebhookContactDeliveries deletes the deliveries whose payload is about the contact.
DELETE FROM webhook_deliveries WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: WebhookEndpointStats :one
-- WebhookEndpointStats sums an endpoint's attempts since the time.
SELECT count(*)::bigint AS attempts,
    count(*) FILTER (WHERE a.response_status BETWEEN 200 AND 299)::bigint AS succeeded,
    coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY a.duration_ms), 0)::bigint AS p50_ms,
    coalesce(percentile_cont(0.95) WITHIN GROUP (ORDER BY a.duration_ms), 0)::bigint AS p95_ms
FROM webhook_delivery_attempts a
JOIN webhook_deliveries d ON d.id = a.delivery_id
WHERE d.workspace_id = @workspace_id::uuid AND d.endpoint_id = @endpoint_id::uuid
  AND a.requested_at >= @since::timestamptz;
