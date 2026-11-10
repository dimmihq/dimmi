-- name: CreateIntegrationInstallation :one
-- CreateIntegrationInstallation stores a new installation of a provider
-- account in the workspace.
INSERT INTO integration_installations (
    id, workspace_id, provider, external_account_id, external_app_id, name, source, scopes, credentials_encrypted,
    metadata, installed_by_member_id, installed_at, created_at, updated_at
) VALUES (
    @id, @workspace_id, @provider, sqlc.narg('external_account_id'), sqlc.narg('external_app_id'), @name, @source,
    @scopes::text[], @credentials_encrypted, @metadata, sqlc.narg('installed_by_member_id'), @now, @now, @now
)
RETURNING *;

-- name: FindIntegrationInstallation :one
-- FindIntegrationInstallation returns one installation of the workspace
-- that was not removed.
SELECT * FROM integration_installations
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockIntegrationInstallation :one
-- LockIntegrationInstallation returns one installation of the workspace
-- that was not removed, locked until the transaction ends.
SELECT * FROM integration_installations
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
FOR UPDATE;

-- name: FindIntegrationInstallationByAccount :one
-- FindIntegrationInstallationByAccount returns the workspace's
-- installation of the provider account through the app, a live one
-- before a disconnected one, the latest first.
SELECT * FROM integration_installations
WHERE workspace_id = @workspace_id AND provider = @provider AND external_app_id = @external_app_id
  AND external_account_id = @external_account_id AND deleted_at IS NULL
ORDER BY (status = 'disconnected'), updated_at DESC, id DESC
LIMIT 1;

-- name: ListIntegrationInstallations :many
-- ListIntegrationInstallations returns the workspace's installations
-- that were not removed, of one provider when one is named, newest
-- first.
SELECT * FROM integration_installations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (sqlc.narg('provider')::text IS NULL OR provider = sqlc.narg('provider')::text)
ORDER BY created_at DESC, id DESC;

-- name: ReinstallIntegrationInstallation :one
-- ReinstallIntegrationInstallation records a fresh grant on an existing
-- installation: its tokens, scopes and provider facts, active again.
UPDATE integration_installations
SET name = @name, scopes = @scopes::text[], credentials_encrypted = @credentials_encrypted, metadata = @metadata,
    source = @source, status = 'active', last_error_code = NULL, last_error = NULL,
    installed_by_member_id = sqlc.narg('installed_by_member_id'), installed_at = @now, disconnected_at = NULL,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: StoreIntegrationCredentials :exec
-- StoreIntegrationCredentials replaces the installation's sealed tokens,
-- after a refresh.
UPDATE integration_installations
SET credentials_encrypted = @credentials_encrypted, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailIntegrationInstallation :one
-- FailIntegrationInstallation marks a live installation broken with the
-- reason.
UPDATE integration_installations
SET status = 'error', last_error_code = @code::text, last_error = @message::text, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status <> 'disconnected'
RETURNING *;

-- name: RecordIntegrationCheck :one
-- RecordIntegrationCheck stores what a health check learned: the account
-- name, provider facts, and that the installation works again when it
-- had failed.
UPDATE integration_installations
SET name = @name, metadata = metadata || @metadata, last_checked_at = @now::timestamptz,
    updated_at = @now::timestamptz,
    status = CASE WHEN status = 'error' THEN 'active' ELSE status END,
    last_error_code = CASE WHEN status = 'error' THEN NULL ELSE last_error_code END,
    last_error = CASE WHEN status = 'error' THEN NULL ELSE last_error END
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchIntegrationCheck :exec
-- TouchIntegrationCheck records that a health check ran without an
-- answer, so the next sweep moves on to others.
UPDATE integration_installations SET last_checked_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DisconnectIntegrationInstallation :one
-- DisconnectIntegrationInstallation marks the installation disconnected
-- and replaces its tokens with sealed empty ones; removed also hides it.
UPDATE integration_installations
SET status = 'disconnected', credentials_encrypted = @credentials_encrypted, disconnected_at = @now::timestamptz,
    deleted_at = CASE WHEN @removed::boolean THEN @now::timestamptz ELSE deleted_at END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RouteIntegrationInstallation :one
-- RouteIntegrationInstallation finds the live installation of the
-- provider account through the app, across every workspace.
SELECT
    r.workspace_id::uuid AS workspace_id,
    r.integration_installation_id::uuid AS integration_installation_id,
    r.status::text AS status
FROM integration_route_installation(@provider::text, @app_id::text, @account_id::text) AS r;

-- name: ListIntegrationInstallationsDue :many
-- ListIntegrationInstallationsDue returns live installations not checked
-- since the time, across every workspace, those never checked first.
SELECT r.workspace_id::uuid AS workspace_id, r.integration_installation_id::uuid AS integration_installation_id
FROM integration_installations_due(@checked_before::timestamptz, @max_rows::integer) AS r;

-- name: ListIntegrationConsumers :many
-- ListIntegrationConsumers returns the channel connections the
-- installation backs.
SELECT id FROM channel_connections
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND deleted_at IS NULL
ORDER BY id;

-- name: SaveIntegrationMemberGrant :one
-- SaveIntegrationMemberGrant stores the member's grant on the
-- installation, replacing an earlier one.
INSERT INTO integration_member_grants (
    workspace_id, integration_installation_id, member_id, external_user_id, scopes, credentials_encrypted,
    created_at, updated_at
) VALUES (
    @workspace_id, @integration_installation_id, @member_id, @external_user_id, @scopes::text[],
    @credentials_encrypted, @now, @now
)
ON CONFLICT (integration_installation_id, member_id) DO UPDATE SET
    external_user_id = EXCLUDED.external_user_id, scopes = EXCLUDED.scopes,
    credentials_encrypted = EXCLUDED.credentials_encrypted, status = 'active', revoked_at = NULL,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteIntegrationUserGrant :execrows
-- DeleteIntegrationUserGrant removes another member's grant held by the
-- same provider user, which a fresh proof from that user replaces.
DELETE FROM integration_member_grants
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND external_user_id = @external_user_id AND member_id <> @member_id;

-- name: FindIntegrationMemberGrant :one
-- FindIntegrationMemberGrant returns one grant of the installation.
SELECT * FROM integration_member_grants
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id AND id = @id;

-- name: FindIntegrationGrantOfMember :one
-- FindIntegrationGrantOfMember returns the member's grant on the
-- installation.
SELECT * FROM integration_member_grants
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND member_id = @member_id;

-- name: CountIntegrationMemberGrants :one
-- CountIntegrationMemberGrants counts the installation's active grants.
SELECT count(*) FROM integration_member_grants
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND status = 'active';

-- name: ListIntegrationMemberGrants :many
-- ListIntegrationMemberGrants returns the installation's active grants,
-- of the provider users named when any are.
SELECT * FROM integration_member_grants
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND status = 'active'
  AND (cardinality(@external_user_ids::text[]) = 0 OR external_user_id = ANY (@external_user_ids::text[]))
ORDER BY id;

-- name: ListMemberIntegrationGrants :many
-- ListMemberIntegrationGrants returns every grant the member holds in
-- the workspace.
SELECT * FROM integration_member_grants
WHERE workspace_id = @workspace_id AND member_id = @member_id
ORDER BY id;

-- name: RevokeIntegrationMemberGrant :one
-- RevokeIntegrationMemberGrant marks a grant revoked and replaces its
-- token with a sealed empty one.
UPDATE integration_member_grants
SET status = 'revoked', credentials_encrypted = @credentials_encrypted, revoked_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: StoreIntegrationMemberCredentials :exec
-- StoreIntegrationMemberCredentials replaces a grant's sealed token,
-- after a refresh.
UPDATE integration_member_grants
SET credentials_encrypted = @credentials_encrypted, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailIntegrationMemberGrant :exec
-- FailIntegrationMemberGrant marks a grant whose token the provider
-- refused.
UPDATE integration_member_grants SET status = 'error', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteIntegrationMemberGrant :exec
-- DeleteIntegrationMemberGrant removes a grant.
DELETE FROM integration_member_grants WHERE workspace_id = @workspace_id AND id = @id;

-- name: ClearIntegrationMemberLinks :exec
-- ClearIntegrationMemberLinks removes, on every channel connection the
-- installation backs, the member's links and anyone's link to the
-- provider user, before a fresh proof links them.
DELETE FROM channel_member_links l
USING channel_connections c
WHERE c.workspace_id = @workspace_id AND c.integration_installation_id = @integration_installation_id
  AND l.channel_connection_id = c.id
  AND (l.workspace_member_id = @member_id OR l.external_user_id = @external_user_id);

-- name: LinkIntegrationMember :exec
-- LinkIntegrationMember records, on every channel connection the
-- installation backs, that the provider user is the member, proven by
-- the provider.
INSERT INTO channel_member_links (
    workspace_id, channel_connection_id, workspace_member_id, external_user_id, source, created_at, updated_at
)
SELECT @workspace_id, c.id, @member_id, @external_user_id, 'oauth', @now, @now
FROM channel_connections c
WHERE c.workspace_id = @workspace_id AND c.integration_installation_id = @integration_installation_id
  AND c.deleted_at IS NULL;

-- name: UnlinkIntegrationMember :exec
-- UnlinkIntegrationMember removes the member's provider-proven links on
-- every channel connection the installation backs.
DELETE FROM channel_member_links l
USING channel_connections c
WHERE c.workspace_id = @workspace_id AND c.integration_installation_id = @integration_installation_id
  AND l.channel_connection_id = c.id AND l.workspace_member_id = @member_id AND l.source = 'oauth';
