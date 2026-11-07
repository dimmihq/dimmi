-- name: CreateActionConnection :one
-- CreateActionConnection stores a new connection; its credentials are
-- sealed in a second statement once the row has its id.
INSERT INTO ai_action_connections (
    workspace_id, name, kind, base_url, auth_type, auth_config, default_headers, variables, timeout_ms,
    created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @name, @kind, @base_url, @auth_type, @auth_config, @default_headers, @variables, @timeout_ms,
    @created_by_member_id, @now, @now
)
RETURNING *;

-- name: FindActionConnection :one
-- FindActionConnection returns the live connection with the id.
SELECT * FROM ai_action_connections
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListActionConnections :many
-- ListActionConnections returns the workspace's live connections.
SELECT * FROM ai_action_connections
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: UpdateActionConnection :one
-- UpdateActionConnection replaces the editable fields of a connection.
UPDATE ai_action_connections SET
    name = @name, base_url = @base_url, auth_type = @auth_type, auth_config = @auth_config,
    default_headers = @default_headers, variables = @variables, timeout_ms = @timeout_ms, status = @status,
    status_reason = @status_reason, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetActionConnectionCredentials :one
-- SetActionConnectionCredentials replaces the sealed secret and its hint;
-- null clears both.
UPDATE ai_action_connections SET
    credentials_encrypted = @credentials_encrypted, secret_hint = @secret_hint, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RecordActionConnectionSuccess :exec
-- RecordActionConnectionSuccess notes a call or test that reached the
-- connection, clearing an error status it was in.
UPDATE ai_action_connections SET
    last_success_at = @now, consecutive_auth_failures = 0,
    status = CASE WHEN status = 'error' THEN 'active' ELSE status END,
    status_reason = CASE WHEN status = 'error' THEN NULL ELSE status_reason END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: RecordActionConnectionFailure :exec
-- RecordActionConnectionFailure notes a failed call or test. Three
-- refused credentials in a row, or a blocked host, put the connection in
-- error so its actions stop being offered until a test passes.
UPDATE ai_action_connections SET
    last_error = @last_error, last_error_at = @now,
    consecutive_auth_failures = CASE WHEN @auth_failure::boolean THEN consecutive_auth_failures + 1 ELSE 0 END,
    status = CASE
        WHEN status = 'disabled' THEN status
        WHEN @blocked::boolean OR (@auth_failure::boolean AND consecutive_auth_failures + 1 >= 3) THEN 'error'
        ELSE status END,
    status_reason = CASE
        WHEN status = 'disabled' THEN status_reason
        WHEN @blocked::boolean THEN 'host_blocked'
        WHEN @auth_failure::boolean AND consecutive_auth_failures + 1 >= 3 THEN 'auth'
        ELSE status_reason END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: RecordActionConnectionSync :exec
-- RecordActionConnectionSync notes a finished MCP tool sync and what the
-- server said about itself.
UPDATE ai_action_connections SET tools_synced_at = @now, mcp_server_info = @mcp_server_info, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: DeleteActionConnection :exec
-- DeleteActionConnection soft-deletes the connection and shreds its
-- secret.
UPDATE ai_action_connections SET
    credentials_encrypted = NULL, secret_hint = NULL, deleted_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: CountConnectionActions :one
-- CountConnectionActions counts the live actions that use the connection.
SELECT count(*) FROM ai_actions
WHERE workspace_id = @workspace_id AND connection_id = @connection_id AND deleted_at IS NULL;
