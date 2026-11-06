-- name: CreateProviderKey :one
-- CreateProviderKey stores a new provider connection with its sealed
-- credentials.
INSERT INTO ai_provider_keys (
    workspace_id, name, provider, base_url, region, auth_type, credentials_encrypted, key_hint,
    status, status_reason, last_error, last_validated_at, models, provider_info, config, source,
    created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @name, @provider, @base_url, @region, @auth_type, @credentials_encrypted, @key_hint,
    @status, @status_reason, @last_error, @last_validated_at, @models, @provider_info, @config, @source,
    @created_by_member_id, @now, @now
)
RETURNING *;

-- name: FindProviderKey :one
-- FindProviderKey returns the live key with the id in the workspace.
SELECT * FROM ai_provider_keys
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListProviderKeys :many
-- ListProviderKeys returns the workspace's live keys.
SELECT * FROM ai_provider_keys
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: CountProviderKeys :one
-- CountProviderKeys counts the workspace's live keys.
SELECT count(*) FROM ai_provider_keys
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: UpdateProviderKey :one
-- UpdateProviderKey replaces the mutable fields of a key, including a
-- rotated credential, and bumps its version so cached drivers expire.
UPDATE ai_provider_keys SET
    name = @name, base_url = @base_url, region = @region, auth_type = @auth_type,
    credentials_encrypted = @credentials_encrypted, key_hint = @key_hint, config = @config,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: LockProviderKeyCredentials :one
-- LockProviderKeyCredentials returns the live key and holds its row
-- until the transaction ends, so two renewals of one sign-in never run
-- at once.
SELECT * FROM ai_provider_keys
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateProviderKeyCredentials :one
-- UpdateProviderKeyCredentials replaces only the sealed credentials of
-- a key, leaving every field an editor may be changing untouched, and
-- bumps its version so cached drivers expire.
UPDATE ai_provider_keys SET credentials_encrypted = @credentials_encrypted, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: UpdateProviderKeyStatus :one
-- UpdateProviderKeyStatus records a validation outcome.
UPDATE ai_provider_keys SET
    status = @status, status_reason = @status_reason, last_error = @last_error,
    last_validated_at = @last_validated_at, last_error_at = @last_error_at,
    models = @models, provider_info = @provider_info, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: TouchProviderKeyUsed :exec
-- TouchProviderKeyUsed marks the key used when it was last touched
-- before the caller's cutoff, so it writes at most once a minute.
UPDATE ai_provider_keys SET last_used_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
    AND (last_used_at IS NULL OR last_used_at < @not_before);

-- name: DeleteProviderKey :exec
-- DeleteProviderKey soft-deletes the key and shreds its credentials.
UPDATE ai_provider_keys SET credentials_encrypted = NULL, deleted_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListProviderKeysForRevalidation :many
-- ListProviderKeysForRevalidation returns live keys due a status check,
-- in batches.
SELECT * FROM ai_provider_keys
WHERE deleted_at IS NULL AND auth_type <> 'managed'
ORDER BY last_validated_at NULLS FIRST, id
LIMIT @batch_limit;

-- name: FindManagedProviderKey :one
-- FindManagedProviderKey returns the workspace's built-in connection
-- when it has one.
SELECT * FROM ai_provider_keys
WHERE workspace_id = @workspace_id AND provider = 'dimmi' AND deleted_at IS NULL
ORDER BY created_at LIMIT 1;
