-- name: RevokeMemberTokens :execrows
-- RevokeMemberTokens ends every live access token the member minted in
-- the workspace, recording who ended them.
UPDATE personal_access_tokens
SET revoked_at = @now::timestamptz, revoked_by_member_id = @revoked_by::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id::uuid AND owner_id = @owner_id AND revoked_at IS NULL;
