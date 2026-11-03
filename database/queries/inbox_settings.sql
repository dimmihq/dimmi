-- name: UpdateInbox :one
-- UpdateInbox renames the inbox and replaces its settings.
UPDATE inboxes SET name = @name, settings = @settings, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindInboxAPIConnection :one
-- FindInboxAPIConnection returns the inbox's live api connection, the
-- one conversations started through the REST API arrive on.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id AND type = 'api' AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: InboxesByIDs :many
-- InboxesByIDs returns the inboxes of the workspace with the ids,
-- deleted ones included, since conversations keep pointing at them.
SELECT * FROM inboxes WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: ChannelConnectionsByIDs :many
-- ChannelConnectionsByIDs returns the connections of the workspace with
-- the ids, deleted ones included.
SELECT * FROM channel_connections WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: FindTeam :one
-- FindTeam returns the live team with the id in the workspace.
SELECT * FROM teams WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: TeamsByIDs :many
-- TeamsByIDs returns the teams of the workspace with the ids, deleted
-- ones included.
SELECT * FROM teams WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: MembersByIDs :many
-- MembersByIDs returns the members of the workspace with the ids and
-- their accounts, active or not.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.id = ANY (@ids::uuid[]);
