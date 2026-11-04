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
