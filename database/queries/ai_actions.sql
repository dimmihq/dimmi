-- name: CreateAction :one
-- CreateAction stores a new draft action.
INSERT INTO ai_actions (
    workspace_id, connection_id, name, title, description, kind, side_effect, confirmation, approval,
    approval_timeout_minutes, method, path_template, query, headers, body_template, body_encoding,
    mcp_tool_name, bound_arguments, input_schema, response_fields, error_message_path, response_max_bytes,
    timeout_ms, channels, spec_hash, source, created_by_member_id, updated_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @connection_id, @name, @title, @description, @kind, @side_effect, @confirmation, @approval,
    @approval_timeout_minutes, @method, @path_template, @query, @headers, @body_template, @body_encoding,
    @mcp_tool_name, @bound_arguments, @input_schema, @response_fields, @error_message_path, @response_max_bytes,
    @timeout_ms, @channels, @spec_hash, @source, @member_id, @member_id, @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindAction :one
-- FindAction returns the live action with the id.
SELECT * FROM ai_actions WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindActionByName :one
-- FindActionByName returns the live action with the tool name.
SELECT * FROM ai_actions WHERE workspace_id = @workspace_id AND name = @name AND deleted_at IS NULL;

-- name: FindActionByToolName :one
-- FindActionByToolName returns the live action that calls the remote
-- tool on the connection.
SELECT * FROM ai_actions
WHERE workspace_id = @workspace_id AND connection_id = @connection_id AND mcp_tool_name = @mcp_tool_name
    AND deleted_at IS NULL
LIMIT 1;

-- name: ListActions :many
-- ListActions returns the workspace's live actions, optionally only one
-- status or one connection's, by name.
SELECT * FROM ai_actions
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
    AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
    AND (sqlc.narg(connection_id)::uuid IS NULL OR connection_id = sqlc.narg(connection_id)::uuid)
ORDER BY name;

-- name: CountActions :one
-- CountActions counts the workspace's live actions.
SELECT count(*) FROM ai_actions WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: UpdateAction :one
-- UpdateAction replaces the working copy of an action.
UPDATE ai_actions SET
    name = @name, title = @title, description = @description, side_effect = @side_effect,
    confirmation = @confirmation, approval = @approval, approval_timeout_minutes = @approval_timeout_minutes,
    method = @method, path_template = @path_template, query = @query, headers = @headers,
    body_template = @body_template, body_encoding = @body_encoding, mcp_tool_name = @mcp_tool_name,
    bound_arguments = @bound_arguments, input_schema = @input_schema, response_fields = @response_fields,
    error_message_path = @error_message_path, response_max_bytes = @response_max_bytes, timeout_ms = @timeout_ms,
    channels = @channels, spec_hash = @spec_hash, updated_by_member_id = @member_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetActionStatus :one
-- SetActionStatus moves the action between published and disabled.
UPDATE ai_actions SET status = @status, updated_by_member_id = @member_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: PublishAction :one
-- PublishAction makes the version the one the agent runs.
UPDATE ai_actions SET
    status = 'published', published_version_id = @version_id, updated_by_member_id = @member_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAction :exec
-- DeleteAction soft-deletes the action; past calls keep it.
UPDATE ai_actions SET deleted_at = @now::timestamptz, status = 'disabled', updated_by_member_id = @member_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: CreateActionVersion :one
-- CreateActionVersion snapshots a definition as the action's next
-- version number.
INSERT INTO ai_action_versions (workspace_id, action_id, number, spec, spec_hash, note, published_by_member_id, created_at)
VALUES (
    @workspace_id, @action_id,
    (SELECT coalesce(max(number), 0) + 1 FROM ai_action_versions v
     WHERE v.workspace_id = @workspace_id AND v.action_id = @action_id),
    @spec, @spec_hash, @note, @member_id, @now::timestamptz
)
RETURNING *;

-- name: FindActionVersion :one
-- FindActionVersion returns one version of the workspace's actions.
SELECT * FROM ai_action_versions WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListActionVersions :many
-- ListActionVersions returns an action's versions, newest first.
SELECT * FROM ai_action_versions
WHERE workspace_id = @workspace_id AND action_id = @action_id
ORDER BY number DESC;

-- name: ListLiveActionVersions :many
-- ListLiveActionVersions returns the published version of every
-- published action whose connection is active: what the agent may offer.
SELECT v.*
FROM ai_actions a
JOIN ai_action_versions v ON v.workspace_id = a.workspace_id AND v.id = a.published_version_id
JOIN ai_action_connections c ON c.workspace_id = a.workspace_id AND c.id = a.connection_id
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL AND a.status = 'published'
    AND c.deleted_at IS NULL AND c.status = 'active'
ORDER BY a.name;
