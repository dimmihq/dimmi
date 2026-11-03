-- name: CreateChannelConnection :one
-- CreateChannelConnection connects a channel to an inbox of the
-- workspace. A provider key another live connection of the type holds
-- fails on channel_connections_type_external_id_key.
INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status, external_id, config, created_at, updated_at)
VALUES (@workspace_id, @inbox_id, @type, @name, @status, @external_id, @config, @now, @now)
RETURNING *;

-- name: FindChannelConnection :one
-- FindChannelConnection returns the connection with the id in the
-- workspace, deleted or not.
SELECT * FROM channel_connections WHERE workspace_id = @workspace_id AND id = @id;
