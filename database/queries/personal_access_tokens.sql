-- name: RevokeMemberTokens :execrows
-- RevokeMemberTokens ends every live access token the member minted in
-- the workspace, recording who ended them.
UPDATE personal_access_tokens
SET revoked_at = @now::timestamptz, revoked_by_member_id = @revoked_by::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id::uuid AND owner_id = @owner_id AND revoked_at IS NULL;

-- name: RevokeWorkspaceTokens :execrows
-- RevokeWorkspaceTokens ends every live access token of the workspace,
-- recording who ended them, nobody when the system did.
UPDATE personal_access_tokens
SET revoked_at = @now::timestamptz, revoked_by_member_id = sqlc.narg(revoked_by)::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id::uuid AND revoked_at IS NULL;
-- name: ListWorkspaceTokens :many
-- ListWorkspaceTokens returns a page of the workspace's tokens, newest
-- first, each with the member it acts as: one member's when owner_id is
-- set, every member's otherwise. Tokens revoked before revoked_after are
-- left out; the rest stay listed so their owner sees that they ended.
SELECT
    t.id, t.owner_id, t.name, t.hint, t.abilities, t.expires_at, t.last_used_at, t.last_used_ip, t.revoked_at,
    t.created_at, m.user_id AS member_user_id, u.name AS member_name
FROM personal_access_tokens t
JOIN workspace_members m ON m.id = t.owner_id AND m.workspace_id = t.workspace_id
JOIN users u ON u.id = m.user_id
WHERE t.workspace_id = @workspace_id::uuid
  AND (sqlc.narg(owner_id)::uuid IS NULL OR t.owner_id = sqlc.narg(owner_id)::uuid)
  AND (t.revoked_at IS NULL OR t.revoked_at > @revoked_after::timestamptz)
ORDER BY t.created_at DESC, t.id DESC
LIMIT @page_limit::bigint OFFSET @page_offset::bigint;

-- name: FindWorkspaceToken :one
-- FindWorkspaceToken returns the workspace's token with the id, with the
-- member it acts as.
SELECT
    t.id, t.owner_id, t.name, t.hint, t.abilities, t.expires_at, t.last_used_at, t.last_used_ip, t.revoked_at,
    t.created_at, m.user_id AS member_user_id, u.name AS member_name
FROM personal_access_tokens t
JOIN workspace_members m ON m.id = t.owner_id AND m.workspace_id = t.workspace_id
JOIN users u ON u.id = m.user_id
WHERE t.workspace_id = @workspace_id::uuid AND t.id = @id;

-- name: RecordTokenMintedRole :exec
-- RecordTokenMintedRole remembers the role the token's member held when
-- it was minted, the most the token may ever do.
UPDATE personal_access_tokens SET minted_as_role = @role
WHERE workspace_id = @workspace_id::uuid AND id = @id;

-- name: FindTokenMintedRole :one
-- FindTokenMintedRole returns the role the token's member held when it
-- was minted, NULL when none was recorded.
SELECT minted_as_role FROM personal_access_tokens
WHERE workspace_id = @workspace_id::uuid AND id = @id;

-- name: CountLiveMemberTokens :one
-- CountLiveMemberTokens counts the member's tokens that still work: not
-- revoked and not expired.
SELECT count(*)
FROM personal_access_tokens
WHERE workspace_id = @workspace_id::uuid AND owner_id = @owner_id AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > @now::timestamptz);

-- name: RenameWorkspaceToken :execrows
-- RenameWorkspaceToken gives the workspace's token a new name.
UPDATE personal_access_tokens
SET name = @name, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id::uuid AND id = @id;

-- name: RevokeWorkspaceToken :execrows
-- RevokeWorkspaceToken ends the workspace's token if it still works,
-- recording who ended it.
UPDATE personal_access_tokens
SET revoked_at = @now::timestamptz, revoked_by_member_id = @revoked_by::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id::uuid AND id = @id AND revoked_at IS NULL;

-- name: ListTokensExpiringSoon :many
-- ListTokensExpiringSoon returns live tokens of active members, across
-- workspaces, that expire in the window and whose owner was not warned
-- yet, soonest first, with what the warning needs.
SELECT
    t.id::uuid AS id,
    t.name::text AS name,
    t.expires_at::timestamptz AS expires_at,
    t.workspace_name::text AS workspace_name,
    t.user_name::text AS user_name,
    t.user_email::citext AS user_email,
    coalesce(t.user_locale, '')::text AS user_locale
FROM auth_tokens_expiring(@expiring_after::timestamptz, @expiring_before::timestamptz, @max_rows::integer) AS t;

-- name: MarkTokenExpiryNotified :one
-- MarkTokenExpiryNotified records that the token's owner was warned, and
-- returns 1, or 0 when someone already did.
SELECT auth_mark_token_expiry_notified(@token_id::uuid, @now::timestamptz)::bigint;

-- name: FindTokenWorkspace :one
-- FindTokenWorkspace returns the workspace of the token the digest
-- belongs to. It runs through a security definer function, since the
-- workspace is only known once the token is found.
SELECT t.workspace_id::uuid AS workspace_id
FROM auth_find_token(@token_digest) AS t;
