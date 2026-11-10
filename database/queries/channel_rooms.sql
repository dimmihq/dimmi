-- name: UpsertChannelRoom :one
-- UpsertChannelRoom starts watching a place, or wakes a removed one
-- again with its settings kept, refreshing its name and whether it is
-- shared with another organization.
INSERT INTO channel_rooms (
    workspace_id, channel_connection_id, kind, external_id, name, mode, trigger, is_shared_external, created_at,
    updated_at
)
VALUES (
    @workspace_id, @channel_connection_id, @kind, @external_id, @name, @mode, @trigger, @is_shared_external,
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (channel_connection_id, external_id) DO UPDATE
SET name = excluded.name, is_shared_external = excluded.is_shared_external,
    status = CASE WHEN channel_rooms.status IN ('removed', 'error') THEN 'active' ELSE channel_rooms.status END,
    removed_at = NULL, last_error = NULL, updated_at = excluded.updated_at
RETURNING *;

-- name: FindChannelRoomByExternalID :one
-- FindChannelRoomByExternalID returns the room of the connection at the
-- provider's place, whatever its status.
SELECT * FROM channel_rooms
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id;

-- name: FindChannelRoom :one
-- FindChannelRoom returns the room with the id.
SELECT * FROM channel_rooms WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListChannelRooms :many
-- ListChannelRooms returns the connection's rooms, by name.
SELECT * FROM channel_rooms
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
ORDER BY name, id;

-- name: RemoveChannelRoom :execrows
-- RemoveChannelRoom stops watching a place the bot left or that is gone.
UPDATE channel_rooms
SET status = 'removed', removed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id AND status <> 'removed';

-- name: RemoveConnectionRooms :exec
-- RemoveConnectionRooms stops watching every place of a connection that
-- was disconnected.
UPDATE channel_rooms
SET status = 'removed', removed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND status <> 'removed';

-- name: RenameChannelRoom :exec
-- RenameChannelRoom follows a rename at the provider.
UPDATE channel_rooms SET name = @name, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id;

-- name: MoveChannelRoom :exec
-- MoveChannelRoom follows a place whose provider id changed; its
-- conversations stay, since their thread keys name the room itself.
UPDATE channel_rooms SET external_id = @new_external_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @old_external_id;

-- name: ShareChannelRoom :one
-- ShareChannelRoom records whether the place is shared with another
-- organization. Any change in who it is shared with unlinks its company,
-- so a second organization never reads the first one's data.
UPDATE channel_rooms
SET is_shared_external = @is_shared_external, company_id = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id
RETURNING *;

-- name: UpdateChannelRoom :one
-- UpdateChannelRoom changes how a room is watched.
UPDATE channel_rooms
SET status = @status, mode = @mode, trigger = @trigger, agent_enabled = @agent_enabled, settings = @settings,
    company_id = sqlc.narg(company_id), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchChannelRoom :exec
-- TouchChannelRoom records when the place last delivered an event, so a
-- gap in deliveries shows.
UPDATE channel_rooms SET last_seen_event_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;
