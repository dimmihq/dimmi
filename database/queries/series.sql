-- name: CreateSeries :one
-- CreateSeries stores a new draft for contacts or for companies.
INSERT INTO series (workspace_id, name, subject, working, working_hash, sender_member_id, created_by_member_id,
    updated_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @name, @subject, @working, @working_hash, @member_id, @member_id, @member_id,
    @now::timestamptz, @now::timestamptz)
RETURNING *;

-- name: FindSeries :one
-- FindSeries returns one series.
SELECT * FROM series WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockSeries :one
-- LockSeries returns one series, locked until the transaction ends.
SELECT * FROM series WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListSeriesPage :many
-- ListSeriesPage returns a page of the workspace's series, most recently
-- changed first. Archived series show only when asked for by status.
SELECT * FROM series
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(status)::text IS NOT NULL OR status <> 'archived')
  AND (sqlc.narg(search)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(search)::text)) > 0)
ORDER BY updated_at DESC, id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: SaveSeries :one
-- SaveSeries stores the working copy and the sending settings when the
-- editor saved over the version it read, and moves the lock on.
UPDATE series SET name = @name, description = @description, working = @working, working_hash = @working_hash,
    topic_id = sqlc.narg(topic_id), service = @service, email_address_id = sqlc.narg(email_address_id),
    sender_member_id = sqlc.narg(sender_member_id), slack_as = @slack_as, reply_team_id = sqlc.narg(reply_team_id),
    reply_member_id = sqlc.narg(reply_member_id), ai_replies = @ai_replies,
    updated_by_member_id = sqlc.narg(member_id), lock_version = lock_version + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND lock_version = @lock_version AND status <> 'archived'
RETURNING *;

-- name: SetSeriesStatus :one
-- SetSeriesStatus moves a series to a status, keeping when it was last
-- published, paused and archived.
UPDATE series SET status = @status, live_version_id = COALESCE(sqlc.narg(live_version_id), live_version_id),
    published_at = COALESCE(sqlc.narg(published_at), published_at), paused_at = sqlc.narg(paused_at),
    archived_at = sqlc.narg(archived_at), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteSeries :execrows
-- DeleteSeries removes a draft that never went live.
DELETE FROM series WHERE workspace_id = @workspace_id AND id = @id AND status = 'draft' AND live_version_id IS NULL;

-- name: ListLiveSeries :many
-- ListLiveSeries returns the workspace's live series with the version
-- each runs.
SELECT sqlc.embed(s), sqlc.embed(v) FROM series s
JOIN series_versions v ON v.id = s.live_version_id
WHERE s.workspace_id = @workspace_id AND s.status = 'live'
ORDER BY s.created_at, s.id;

-- name: SetSeriesCounts :exec
-- SetSeriesCounts keeps the latest totals on the series.
UPDATE series SET counts = @counts WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateSeriesVersion :one
-- CreateSeriesVersion freezes a working copy as the next version.
INSERT INTO series_versions (workspace_id, series_id, number, spec, spec_hash, note, moves, published_by_member_id,
    restored_from_version_id, created_at)
SELECT @workspace_id, @series_id, coalesce(max(number), 0) + 1, @spec, @spec_hash, @note, @moves,
    sqlc.narg(member_id), sqlc.narg(restored_from_version_id), @now::timestamptz
FROM series_versions WHERE series_id = @series_id
RETURNING *;

-- name: FindSeriesVersion :one
-- FindSeriesVersion returns one version of a series.
SELECT * FROM series_versions WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListSeriesVersions :many
-- ListSeriesVersions returns a series' versions, newest first.
SELECT * FROM series_versions WHERE workspace_id = @workspace_id AND series_id = @series_id
ORDER BY number DESC;

-- name: UpsertSeriesBroadcast :one
-- UpsertSeriesBroadcast keeps the broadcast a message step sends
-- through in step with the step and the series' sending settings.
INSERT INTO broadcasts (workspace_id, name, channel, status, kind, series_id, series_node_id, topic_id, service,
    subject, preview, body, sender_member_id, email_address_id, slack_as, reply_team_id, reply_member_id,
    ai_replies, track_clicks, skip_frequency_cap, started_at, created_at, updated_at)
VALUES (@workspace_id, @name, @channel, @status, 'series', @series_id, @series_node_id, sqlc.narg(topic_id), @service,
    @subject, @preview, @body, sqlc.narg(sender_member_id), sqlc.narg(email_address_id), @slack_as,
    sqlc.narg(reply_team_id), sqlc.narg(reply_member_id), @ai_replies, @track_clicks, true, @now::timestamptz,
    @now::timestamptz, @now::timestamptz)
ON CONFLICT (series_id, series_node_id) WHERE series_id IS NOT NULL DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, status = EXCLUDED.status, topic_id = EXCLUDED.topic_id,
    service = EXCLUDED.service, subject = EXCLUDED.subject, preview = EXCLUDED.preview, body = EXCLUDED.body,
    sender_member_id = EXCLUDED.sender_member_id, email_address_id = EXCLUDED.email_address_id,
    slack_as = EXCLUDED.slack_as, reply_team_id = EXCLUDED.reply_team_id,
    reply_member_id = EXCLUDED.reply_member_id, ai_replies = EXCLUDED.ai_replies,
    track_clicks = EXCLUDED.track_clicks, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: SetSeriesBroadcastsStatus :exec
-- SetSeriesBroadcastsStatus moves every message step of a series to
-- sending, paused or stopped with the series.
UPDATE broadcasts SET status = @status, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND series_id = @series_id AND kind = 'series';

-- name: FindSeriesBroadcast :one
-- FindSeriesBroadcast returns the broadcast a message step sends through.
SELECT * FROM broadcasts
WHERE workspace_id = @workspace_id AND series_id = @series_id AND series_node_id = @series_node_id;

-- name: ListSeriesBroadcasts :many
-- ListSeriesBroadcasts returns the broadcasts of a series' message steps.
SELECT * FROM broadcasts WHERE workspace_id = @workspace_id AND series_id = @series_id ORDER BY created_at;
