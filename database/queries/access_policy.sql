-- name: ListLiveTokenAddresses :many
-- ListLiveTokenAddresses returns the last address of every live token of
-- the workspace that was ever used, to warn about tokens a new allowlist
-- would stop.
SELECT last_used_ip::text
FROM personal_access_tokens
WHERE workspace_id = @workspace_id::uuid
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > @now::timestamptz)
  AND last_used_ip IS NOT NULL;

-- name: ListUserIdentityProviders :many
-- ListUserIdentityProviders returns which identity providers each of the
-- users has linked, to judge whether they could still sign in under a
-- list of allowed methods.
SELECT DISTINCT user_id, provider
FROM oauth_identities
WHERE user_id = ANY(@user_ids::uuid[]);

-- name: ListActiveMemberUserIDs :many
-- ListActiveMemberUserIDs returns the account behind every active member
-- of the workspace, whose open connections a change in its rules closes.
SELECT user_id
FROM workspace_members
WHERE workspace_id = @workspace_id AND status = 'active';
