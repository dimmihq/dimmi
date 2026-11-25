-- name: CountEmailSent :exec
-- CountEmailSent counts one more customer email the workspace sent
-- today.
INSERT INTO email_sending_stats (workspace_id, day, stream, sent, created_at, updated_at)
VALUES (@workspace_id, timezone('UTC', @now::timestamptz)::date, 'customer', 1, @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id, day, stream) DO UPDATE
SET sent = email_sending_stats.sent + 1, updated_at = excluded.updated_at;

-- name: CountEmailHardBounce :exec
-- CountEmailHardBounce counts one more of the workspace's customer
-- emails that bounced for good, on the day the report arrived.
INSERT INTO email_sending_stats (workspace_id, day, stream, hard_bounced, created_at, updated_at)
VALUES (@workspace_id, timezone('UTC', @now::timestamptz)::date, 'customer', 1, @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id, day, stream) DO UPDATE
SET hard_bounced = email_sending_stats.hard_bounced + 1, updated_at = excluded.updated_at;

-- name: CountEmailComplaint :exec
-- CountEmailComplaint counts one more of the workspace's customer emails
-- reported as spam, on the day the report arrived.
INSERT INTO email_sending_stats (workspace_id, day, stream, complained, created_at, updated_at)
VALUES (@workspace_id, timezone('UTC', @now::timestamptz)::date, 'customer', 1, @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id, day, stream) DO UPDATE
SET complained = email_sending_stats.complained + 1, updated_at = excluded.updated_at;

-- name: SumEmailSendingStats :one
-- SumEmailSendingStats adds up what the workspace sent to customers
-- since the day, and how much of it bounced for good or was reported as
-- spam.
SELECT coalesce(sum(sent), 0)::bigint AS sent, coalesce(sum(hard_bounced), 0)::bigint AS hard_bounced,
    coalesce(sum(complained), 0)::bigint AS complained
FROM email_sending_stats
WHERE workspace_id = @workspace_id AND stream = 'customer' AND day::timestamp >= date_trunc('day', timezone('UTC', @since::timestamptz));

-- name: ResetEmailSendingStats :execrows
-- ResetEmailSendingStats forgets the workspace's counts, so a workspace
-- whose sending was resumed is judged on what it sends from then on.
DELETE FROM email_sending_stats WHERE workspace_id = @workspace_id;
