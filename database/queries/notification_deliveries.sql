-- name: RecordNotificationDelivery :exec
-- RecordNotificationDelivery writes how one delivery on one channel
-- ended. A retry of the same delivery updates its row and counts the
-- attempt.
INSERT INTO notification_deliveries (
    workspace_id, workspace_member_id, notification_destination_id, delivery_id, trigger, channel, status,
    reason, detail, title, link, occurred_at, created_at, updated_at
) VALUES (
    @workspace_id, sqlc.narg(workspace_member_id), sqlc.narg(notification_destination_id), @delivery_id,
    @trigger, @channel, @status, sqlc.narg(reason), sqlc.narg(detail), @title, @link, @occurred_at,
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (delivery_id, channel) DO UPDATE SET
    status = excluded.status, reason = excluded.reason, detail = excluded.detail,
    attempts = notification_deliveries.attempts + 1, updated_at = excluded.updated_at;

-- name: ListNotificationDeliveries :many
-- ListNotificationDeliveries returns the workspace's deliveries, newest
-- first, of one member when one is named, after the cursor when one is
-- given.
SELECT * FROM notification_deliveries
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(member_id)::uuid IS NULL OR workspace_member_id = sqlc.narg(member_id))
  AND workspace_member_id IS NOT NULL
  AND (sqlc.narg(channel)::text IS NULL OR channel = sqlc.narg(channel))
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(before_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(before_at), sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: ListDeliveriesOfNotification :many
-- ListDeliveriesOfNotification returns a member's deliveries of one bell
-- row: the delivery that made it and the latest one folded into it.
SELECT d.* FROM notification_deliveries d
JOIN notifications n ON n.tenant_id = d.workspace_id::text AND n.id = @notification_id
  AND n.notifiable_type = 'workspace_member' AND n.notifiable_id = @notifiable_id
WHERE d.workspace_id = @workspace_id AND d.workspace_member_id = @member_id::uuid
  AND (d.delivery_id = n.id OR d.delivery_id::text = n.last_delivery_id)
ORDER BY d.created_at, d.id;

-- name: ListQuietHeldDeliveries :many
-- ListQuietHeldDeliveries returns what quiet hours held back from a
-- member and no summary has covered yet, one row per notification,
-- oldest first.
SELECT DISTINCT ON (delivery_id) * FROM notification_deliveries
WHERE workspace_id = @workspace_id AND workspace_member_id = @member_id::uuid
  AND reason = 'quiet_hours' AND summarized_at IS NULL
ORDER BY delivery_id, created_at;

-- name: MarkQuietDeliveriesSummarized :exec
-- MarkQuietDeliveriesSummarized stamps what a summary covered.
UPDATE notification_deliveries SET summarized_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND workspace_member_id = @member_id::uuid
  AND reason = 'quiet_hours' AND summarized_at IS NULL;

-- name: ListQuietHeldMembers :many
-- ListQuietHeldMembers returns the members, across every workspace, who
-- have deliveries held by quiet hours that no summary covered yet.
SELECT h.workspace_id::uuid AS workspace_id, h.workspace_member_id::uuid AS workspace_member_id
FROM notification_quiet_held(@max_rows::integer) AS h;

-- name: PruneNotificationDeliveries :one
-- PruneNotificationDeliveries deletes a batch of deliveries, across
-- every workspace, recorded before the time.
SELECT prune_notification_deliveries(@before::timestamptz, @max_rows::integer)::bigint AS deleted;
