-- name: ListIdentitySecrets :many
-- ListIdentitySecrets returns the workspace's secrets, newest first,
-- revoked ones included.
SELECT * FROM identity_secrets WHERE workspace_id = @workspace_id ORDER BY created_at DESC, id DESC;

-- name: FindIdentitySecret :one
-- FindIdentitySecret returns the secret with the id in the workspace.
SELECT * FROM identity_secrets WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockIdentitySecret :one
-- LockIdentitySecret returns the secret and holds it until the
-- transaction ends, so two changes to it never interleave.
SELECT * FROM identity_secrets WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: CountLiveIdentitySecrets :one
-- CountLiveIdentitySecrets counts the secrets not yet revoked.
SELECT count(*) FROM identity_secrets WHERE workspace_id = @workspace_id AND status <> 'revoked';

-- name: CreateIdentitySecret :one
-- CreateIdentitySecret stores a new active secret.
INSERT INTO identity_secrets (
    id, workspace_id, name, algorithm, secret_encrypted, public_key_jwk, fingerprint, created_by_member_id,
    created_at, updated_at
) VALUES (
    @id, @workspace_id, @name, @algorithm, @secret_encrypted, @public_key_jwk, @fingerprint, @created_by_member_id,
    @now, @now
)
RETURNING *;

-- name: RenameIdentitySecret :one
-- RenameIdentitySecret changes the secret's name.
UPDATE identity_secrets SET name = @name, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RetireIdentitySecret :one
-- RetireIdentitySecret keeps the secret working until retires_at.
UPDATE identity_secrets SET status = 'retiring', retires_at = @retires_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RevokeIdentitySecret :one
-- RevokeIdentitySecret stops the secret verifying anything, for good.
UPDATE identity_secrets SET
    status = 'revoked', retires_at = NULL, revoked_at = @now::timestamptz, revoked_reason = @reason::text,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchIdentitySecret :exec
-- TouchIdentitySecret notes the secret just verified a token, at most
-- once a minute.
UPDATE identity_secrets SET last_used_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND (last_used_at IS NULL OR last_used_at < @stale_before);

-- name: RevokeWidgetSessionsForSecret :many
-- RevokeWidgetSessionsForSecret ends every live session the secret
-- verified.
UPDATE widget_sessions SET revoked_at = @now::timestamptz, revoke_reason = 'secret_revoked',
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND identity_secret_id = @identity_secret_id AND revoked_at IS NULL
RETURNING id, contact_id;

-- name: VerifyWidgetSession :one
-- VerifyWidgetSession records that a fresh identity token proved who the
-- session belongs to, and extends it.
UPDATE widget_sessions SET
    kind = 'identified', identity_secret_id = @identity_secret_id, external_id = @external_id,
    verified_at = @now::timestamptz, expires_at = @expires_at, last_seen_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ApplyContactProfile :one
-- ApplyContactProfile writes what an identity token says about the
-- customer that the general contact update does not carry.
UPDATE contacts SET
    avatar_url = coalesce(sqlc.narg(avatar_url), avatar_url),
    locale = coalesce(sqlc.narg(locale), locale),
    timezone = coalesce(sqlc.narg(timezone), timezone),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UpsertContactIdentity :exec
-- UpsertContactIdentity records a verified identity of the contact; one
-- another contact already holds stays with them.
INSERT INTO contact_identities (workspace_id, contact_id, type, value, verified, created_at, updated_at)
VALUES (@workspace_id, @contact_id, @type, @value, true, @now, @now)
ON CONFLICT (workspace_id, type, value) DO UPDATE SET verified = true, updated_at = excluded.updated_at
WHERE contact_identities.contact_id = excluded.contact_id;

-- name: MoveLeadConversations :exec
-- MoveLeadConversations hands a merged lead's conversations to the
-- customer it turned out to be.
UPDATE conversations SET contact_id = @into_contact_id, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: MoveLeadParticipations :exec
-- MoveLeadParticipations moves a merged lead's places in conversations,
-- dropping those the customer already holds.
UPDATE conversation_participants p SET contact_id = @into_contact_id
WHERE p.workspace_id = @workspace_id AND p.contact_id = @contact_id
  AND NOT EXISTS (
      SELECT 1 FROM conversation_participants q
      WHERE q.conversation_id = p.conversation_id AND q.contact_id = @into_contact_id
  );

-- name: MoveLeadActionCalls :exec
-- MoveLeadActionCalls keeps a merged lead's agent action calls on the
-- customer.
UPDATE ai_action_calls SET contact_id = @into_contact_id
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: MarkContactMerged :exec
-- MarkContactMerged leaves the lead as a tombstone pointing at the
-- customer it became.
UPDATE contacts SET merged_into_id = @into_contact_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockContactExternalID :exec
-- LockContactExternalID serializes first sign-ins of one customer, so two
-- tabs signing in at once make one contact.
SELECT pg_advisory_xact_lock(hashtext(@workspace_id::text || ':' || @external_id::text));
