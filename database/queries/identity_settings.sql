-- name: FindIdentitySettings :one
-- FindIdentitySettings returns how strict the workspace's sign-in is.
SELECT * FROM identity_settings WHERE workspace_id = @workspace_id;

-- name: UpsertIdentitySettings :one
-- UpsertIdentitySettings stores the workspace's sign-in settings.
INSERT INTO identity_settings (
    workspace_id, mode, client_writable_attributes, updated_by_member_id, created_at, updated_at
) VALUES (@workspace_id, @mode, @client_writable_attributes, @updated_by_member_id, @now, @now)
ON CONFLICT (workspace_id) DO UPDATE SET
    mode = excluded.mode, client_writable_attributes = excluded.client_writable_attributes,
    updated_by_member_id = excluded.updated_by_member_id, updated_at = excluded.updated_at
RETURNING *;

-- name: AddIdentityVerificationStat :exec
-- AddIdentityVerificationStat adds to a day's count of one sign-in
-- outcome. Days travel as YYYY-MM-DD text, always UTC.
INSERT INTO identity_verification_stats (workspace_id, day, outcome, count)
VALUES (@workspace_id, to_date(@day::text, 'YYYY-MM-DD'), @outcome, @count)
ON CONFLICT (workspace_id, day, outcome) DO UPDATE SET count = identity_verification_stats.count + excluded.count;

-- name: ListIdentityVerificationStats :many
-- ListIdentityVerificationStats returns the workspace's daily counts
-- from the day on.
SELECT day::text AS day, outcome, count
FROM identity_verification_stats
WHERE workspace_id = @workspace_id AND day >= to_date(@since::text, 'YYYY-MM-DD')
ORDER BY day, outcome;

-- name: PruneIdentityVerificationStats :one
-- PruneIdentityVerificationStats drops every workspace's counts older
-- than the day.
SELECT identity_stats_prune(to_date(@before::text, 'YYYY-MM-DD'))::bigint AS pruned;
