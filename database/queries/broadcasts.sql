-- name: CreateBroadcast :one
-- CreateBroadcast stores a new draft.
INSERT INTO broadcasts (workspace_id, name, channel, sender_member_id, created_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @name, @channel, @member_id, @member_id, @now::timestamptz, @now::timestamptz)
RETURNING *;

-- name: FindBroadcast :one
-- FindBroadcast returns one broadcast.
SELECT * FROM broadcasts WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockBroadcast :one
-- LockBroadcast returns one broadcast, locked until the transaction ends.
SELECT * FROM broadcasts WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: ListBroadcastPage :many
-- ListBroadcastPage returns a page of the workspace's broadcasts, newest
-- first, narrowed by status, channel and name.
SELECT * FROM broadcasts
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(channel)::text IS NULL OR channel = sqlc.narg(channel)::text)
  AND (sqlc.narg(search)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(search)::text)) > 0)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: SaveBroadcastDraft :one
-- SaveBroadcastDraft stores what the composer holds, while the broadcast
-- has not started sending.
UPDATE broadcasts SET name = @name, audience_id = sqlc.narg(audience_id), rules = sqlc.narg(rules),
    room_ids = @room_ids, topic_id = sqlc.narg(topic_id), service = @service, subject = @subject, preview = @preview,
    body = @body, sender_member_id = sqlc.narg(sender_member_id), email_address_id = sqlc.narg(email_address_id),
    slack_as = @slack_as, reply_team_id = sqlc.narg(reply_team_id), reply_member_id = sqlc.narg(reply_member_id),
    ai_replies = @ai_replies, track_clicks = @track_clicks, skip_frequency_cap = @skip_frequency_cap,
    schedule_kind = @schedule_kind, scheduled_at = sqlc.narg(scheduled_at), goal = sqlc.narg(goal), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status IN ('draft', 'scheduled')
RETURNING *;

-- name: SetBroadcastStatus :one
-- SetBroadcastStatus moves a broadcast to a status, keeping why it
-- paused and when it started and finished.
UPDATE broadcasts SET status = @status, pause_reason = sqlc.narg(pause_reason),
    sent_by_member_id = COALESCE(sqlc.narg(sent_by_member_id), sent_by_member_id),
    started_at = COALESCE(started_at, sqlc.narg(started_at)), finished_at = sqlc.narg(finished_at), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteBroadcast :one
-- DeleteBroadcast removes a draft.
UPDATE broadcasts SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status = 'draft'
RETURNING id;

-- name: SetBroadcastCounts :exec
-- SetBroadcastCounts keeps the latest tally on the broadcast.
UPDATE broadcasts SET counts = @counts WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListActiveBroadcasts :many
-- ListActiveBroadcasts returns the workspace's broadcasts that are
-- scheduled or sending, so a sweep can restart any whose job was lost.
SELECT * FROM broadcasts WHERE workspace_id = @workspace_id AND status IN ('scheduled', 'sending') AND deleted_at IS NULL;

-- name: InsertBroadcastLink :exec
-- InsertBroadcastLink records a link a broadcast tracks.
INSERT INTO broadcast_links (workspace_id, broadcast_id, n, url) VALUES (@workspace_id, @broadcast_id, @n, @url)
ON CONFLICT (broadcast_id, n) DO UPDATE SET url = EXCLUDED.url;

-- name: ListBroadcastLinks :many
-- ListBroadcastLinks returns a broadcast's tracked links in order.
SELECT * FROM broadcast_links WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id ORDER BY n;

-- name: FindBroadcastLink :one
-- FindBroadcastLink returns one tracked link.
SELECT * FROM broadcast_links WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND n = @n;

-- name: CountBroadcastLinkClick :exec
-- CountBroadcastLinkClick counts one click on a tracked link.
UPDATE broadcast_links SET clicks = clicks + 1
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND n = @n;

-- name: FindBroadcastSender :one
-- FindBroadcastSender returns the teammate a broadcast comes from, with
-- the name and picture people see.
SELECT m.id, m.status, coalesce(u.name, '')::text AS name, coalesce(u.email::text, '')::text AS email,
    u.avatar_path
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.id = @id;

-- name: ListGoalBroadcasts :many
-- ListGoalBroadcasts returns the broadcasts with a goal that started
-- since the time, whose goal window may still be open.
SELECT * FROM broadcasts
WHERE workspace_id = @workspace_id AND goal IS NOT NULL AND started_at >= @since::timestamptz AND deleted_at IS NULL;
