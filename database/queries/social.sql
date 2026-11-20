-- name: ListChannelConnectionsOfType :many
-- ListChannelConnectionsOfType returns the workspace's live connections
-- of one type, such as its Facebook Pages, oldest first.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = @type AND deleted_at IS NULL
ORDER BY created_at, id;
