-- name: RouteChannelConnection :one
-- RouteChannelConnection finds the live connection of the type holding
-- the provider key, across every workspace, before any workspace is set.
SELECT
    r.workspace_id::uuid AS workspace_id,
    r.channel_connection_id::uuid AS channel_connection_id,
    r.status::text AS status,
    (r.suspended_at IS NOT NULL)::boolean AS suspended
FROM channel_route_connection(@connection_type::text, @provider_key::text) AS r;

-- name: RouteChannelConnectionID :one
-- RouteChannelConnectionID finds the live connection of the type with the
-- id, across every workspace, before any workspace is set.
SELECT
    r.workspace_id::uuid AS workspace_id,
    r.channel_connection_id::uuid AS channel_connection_id,
    r.status::text AS status,
    (r.suspended_at IS NOT NULL)::boolean AS suspended
FROM channel_route_connection_id(@connection_type::text, @connection_id::uuid) AS r;

-- name: ListSocketChannelConnections :many
-- ListSocketChannelConnections returns every live connection of the type
-- that receives over a socket, across workspaces.
SELECT r.workspace_id::uuid AS workspace_id, r.channel_connection_id::uuid AS channel_connection_id
FROM channel_socket_connections(@connection_type::text) AS r;

-- name: ConnectChannelConnection :one
-- ConnectChannelConnection records what the provider said the connection
-- is and makes it live.
UPDATE channel_connections
SET external_id = @external_id, status = 'active', config = config || @config::jsonb,
    credentials_encrypted = @credentials_encrypted, last_error = NULL, last_error_at = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MarkChannelConnection :one
-- MarkChannelConnection moves the connection to the status, with the
-- error that caused it.
UPDATE channel_connections
SET status = @status, last_error = sqlc.narg(last_error),
    last_error_at = CASE WHEN sqlc.narg(last_error)::text IS NULL THEN NULL ELSE @now::timestamptz END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindLiveChannelConnectionByExternalID :one
-- FindLiveChannelConnectionByExternalID returns the workspace's live
-- connection of the type holding the provider key.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = @type AND external_id = @external_id AND deleted_at IS NULL;

-- name: TouchChannelConnection :exec
-- TouchChannelConnection records the connection's latest activity.
UPDATE channel_connections SET last_activity_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateSlackConnection :one
-- CreateSlackConnection connects Slack to an inbox of the workspace,
-- through an installation when the install's shared app made it.
INSERT INTO channel_connections (
    workspace_id, inbox_id, type, name, status, external_id, config, integration_installation_id, created_at,
    updated_at
)
VALUES (
    @workspace_id, @inbox_id, 'slack', @name, @status, sqlc.narg(external_id), @config,
    sqlc.narg(integration_installation_id), @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindInstallationChannelConnection :one
-- FindInstallationChannelConnection returns the workspace's live Slack
-- connection fed by the installation.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = 'slack' AND integration_installation_id = @integration_installation_id
  AND deleted_at IS NULL;

-- name: FindAdoptableSlackConnection :one
-- FindAdoptableSlackConnection returns the workspace's disconnected Slack
-- connection of the team that no live installation feeds, the latest
-- first, so a new installation of the same team carries on its rooms and
-- conversations.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = 'slack' AND status = 'disconnected' AND deleted_at IS NULL
  AND config->>'team_id' = @team_id::text
ORDER BY updated_at DESC, id DESC
LIMIT 1;

-- name: AdoptChannelConnection :exec
-- AdoptChannelConnection hands the connection to the installation that
-- now feeds it.
UPDATE channel_connections
SET integration_installation_id = @integration_installation_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ReleaseChannelConnectionAddress :exec
-- ReleaseChannelConnectionAddress frees the provider address of a
-- disconnected connection, so the same Slack team can be connected again
-- here or by another workspace.
UPDATE channel_connections
SET external_id = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RouteRoomConnections :many
-- RouteRoomConnections returns the provider keys of every live
-- connection of the type watching the room, across every workspace,
-- before any workspace is set.
SELECT r.provider_key::text AS provider_key
FROM channel_route_room_connections(@connection_type::text, @room_external_id::text) AS r;
