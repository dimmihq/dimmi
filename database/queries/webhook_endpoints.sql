-- name: ListWebhookEndpoints :many
-- ListWebhookEndpoints returns the workspace's endpoints, oldest first.
SELECT * FROM webhook_endpoints
WHERE workspace_id = @workspace_id
ORDER BY created_at, id;

-- name: ListEnabledWebhookEndpoints :many
-- ListEnabledWebhookEndpoints returns the endpoints that receive events.
SELECT * FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND enabled
ORDER BY created_at, id;

-- name: FindWebhookEndpoint :one
-- FindWebhookEndpoint returns one endpoint of the workspace.
SELECT * FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockWebhookEndpoint :one
-- LockWebhookEndpoint returns one endpoint, locked until the transaction ends.
SELECT * FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND id = @id
FOR UPDATE;

-- name: CountWebhookEndpoints :one
-- CountWebhookEndpoints counts the workspace's endpoints.
SELECT count(*) FROM webhook_endpoints WHERE workspace_id = @workspace_id;

-- name: CreateWebhookEndpoint :one
-- CreateWebhookEndpoint stores a new endpoint with its sealed secret.
INSERT INTO webhook_endpoints (
    workspace_id, url, description, event_types, include_notes, inbox_ids, api_version, metadata,
    secret_encrypted, created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @url, @description, @event_types, @include_notes, sqlc.narg(inbox_ids), @api_version, @metadata,
    @secret_encrypted, sqlc.narg(created_by_member_id), @now, @now
)
RETURNING *;

-- name: UpdateWebhookEndpoint :one
-- UpdateWebhookEndpoint saves an endpoint's settings and health.
UPDATE webhook_endpoints
SET url = @url, description = @description, event_types = @event_types, include_notes = @include_notes,
    inbox_ids = sqlc.narg(inbox_ids), api_version = @api_version, metadata = @metadata, enabled = @enabled,
    disabled_reason = sqlc.narg(disabled_reason), disabled_at = sqlc.narg(disabled_at), health = @health,
    failing_since = sqlc.narg(failing_since), failing_notified_at = sqlc.narg(failing_notified_at),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetWebhookEndpointSecrets :one
-- SetWebhookEndpointSecrets replaces the current and previous secrets.
UPDATE webhook_endpoints
SET secret_encrypted = @secret_encrypted, previous_secret_encrypted = sqlc.narg(previous_secret_encrypted),
    previous_secret_expires_at = sqlc.narg(previous_secret_expires_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ExpireWebhookEndpointSecrets :execrows
-- ExpireWebhookEndpointSecrets forgets the previous secrets whose overlap ended.
UPDATE webhook_endpoints
SET previous_secret_encrypted = NULL, previous_secret_expires_at = NULL
WHERE workspace_id = @workspace_id AND previous_secret_expires_at <= @now::timestamptz;

-- name: RecordWebhookEndpointSuccess :exec
-- RecordWebhookEndpointSuccess marks the endpoint healthy after a delivered request.
UPDATE webhook_endpoints
SET health = 'healthy', failing_since = NULL, failing_notified_at = NULL, last_success_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordWebhookEndpointFailure :exec
-- RecordWebhookEndpointFailure marks the endpoint failing since its first failure after the last success.
UPDATE webhook_endpoints
SET health = 'failing', failing_since = coalesce(failing_since, @now), last_failure_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkWebhookEndpointFailingNotified :exec
-- MarkWebhookEndpointFailingNotified records that owners heard the endpoint fails.
UPDATE webhook_endpoints SET failing_notified_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DisableWebhookEndpoint :one
-- DisableWebhookEndpoint turns an enabled endpoint off for the reason.
UPDATE webhook_endpoints
SET enabled = false, disabled_reason = @disabled_reason, disabled_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND enabled
RETURNING *;

-- name: DeleteWebhookEndpoint :execrows
-- DeleteWebhookEndpoint deletes an endpoint with its deliveries.
DELETE FROM webhook_endpoints WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListFailingWebhookEndpoints :many
-- ListFailingWebhookEndpoints returns the enabled endpoints failing since before the time.
SELECT * FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND enabled AND health = 'failing' AND failing_since <= @since;

-- name: ListWorkspacesWithWebhookEndpoints :many
-- ListWorkspacesWithWebhookEndpoints returns every workspace that has an endpoint.
SELECT w.workspace_id::uuid AS workspace_id FROM webhook_workspaces_with_endpoints() AS w;

-- name: HasEnabledWebhookEndpoints :one
-- HasEnabledWebhookEndpoints reports whether any endpoint of the workspace receives events.
SELECT EXISTS (SELECT 1 FROM webhook_endpoints WHERE workspace_id = @workspace_id AND enabled)::boolean AS has;

-- name: ListWebhookAlertRecipients :many
-- ListWebhookAlertRecipients returns the active owners and admins who hear about failing endpoints.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.role IN ('owner', 'admin') AND m.status = 'active'
ORDER BY m.created_at, m.id;
