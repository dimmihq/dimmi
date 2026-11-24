-- name: DeletePendingAuthFactorsBefore :execrows
-- DeletePendingAuthFactorsBefore removes enrollments started before the
-- cutoff and never finished.
DELETE FROM user_auth_factors WHERE status = 'pending' AND created_at < @before;

-- name: DeleteSpentOneTimeTokensBefore :execrows
-- DeleteSpentOneTimeTokensBefore removes one-time tokens that expired or
-- were used before the cutoff.
DELETE FROM one_time_tokens WHERE expires_at < @before OR consumed_at < @before;

-- name: DeleteExpiredPasswordResets :execrows
-- DeleteExpiredPasswordResets removes password resets past their expiry.
DELETE FROM password_reset_tokens WHERE expires_at <= @now;

-- name: DeleteExpiredSessions :execrows
-- DeleteExpiredSessions removes sessions past their absolute expiry or
-- unused since before the idle cutoff.
DELETE FROM sessions WHERE expires_at <= @now OR last_activity_at < @idle_before;

-- name: PurgeExpiredWidgetSessions :one
-- PurgeExpiredWidgetSessions deletes a batch of anonymous messenger
-- sessions that expired before the cutoff, across every workspace.
SELECT dimmi_purge_expired_widget_sessions(@expired_before::timestamptz, @max_rows::integer)::bigint;

-- name: PurgeExpiredInvitations :one
-- PurgeExpiredInvitations deletes a batch of invitations that expired or
-- were revoked before the cutoff, across every workspace.
SELECT dimmi_purge_expired_invitations(@ended_before::timestamptz, @max_rows::integer)::bigint;

-- name: PurgeDeadAccessTokens :one
-- PurgeDeadAccessTokens deletes a batch of access tokens that expired or
-- were revoked before the cutoff, across every workspace.
SELECT dimmi_purge_dead_access_tokens(@ended_before::timestamptz, @max_rows::integer)::bigint;

-- name: DeleteDeadOAuthTokens :execrows
-- DeleteDeadOAuthTokens removes connected apps' tokens that expired or
-- were revoked before the cutoff.
DELETE FROM oauth_tokens WHERE expires_at < @ended_before::timestamptz OR revoked_at < @ended_before::timestamptz;
