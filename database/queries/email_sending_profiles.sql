-- name: EnsureEmailSendingProfile :one
-- EnsureEmailSendingProfile creates the workspace's sending profile when
-- it has none, and names its SES tenant when it has no name yet.
INSERT INTO email_sending_profiles (workspace_id, ses_tenant_name, created_at, updated_at)
VALUES (@workspace_id, sqlc.narg(ses_tenant_name), @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id) DO UPDATE
SET ses_tenant_name = coalesce(email_sending_profiles.ses_tenant_name, excluded.ses_tenant_name),
    updated_at = CASE WHEN email_sending_profiles.ses_tenant_name IS NULL THEN excluded.updated_at
        ELSE email_sending_profiles.updated_at END
RETURNING *;

-- name: FindEmailSendingProfile :one
-- FindEmailSendingProfile returns the workspace's sending profile.
SELECT * FROM email_sending_profiles WHERE workspace_id = @workspace_id;

-- name: PauseEmailSending :one
-- PauseEmailSending pauses the workspace's sending for the reason. A
-- workspace paused already returns no row, so it is paused, and its
-- people told, once.
INSERT INTO email_sending_profiles (workspace_id, status, paused_at, paused_reason, created_at, updated_at)
VALUES (@workspace_id, 'paused', @now::timestamptz, @reason, @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id) DO UPDATE
SET status = 'paused', paused_at = excluded.paused_at, paused_reason = excluded.paused_reason,
    updated_at = excluded.updated_at
WHERE email_sending_profiles.status <> 'paused'
RETURNING *;

-- name: ResumeEmailSending :one
-- ResumeEmailSending lets a paused workspace send again. A workspace
-- that is not paused returns no row.
UPDATE email_sending_profiles
SET status = 'active', paused_at = NULL, paused_reason = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND status = 'paused'
RETURNING *;

-- name: PauseSendingEmailBroadcasts :execrows
-- PauseSendingEmailBroadcasts pauses the workspace's email broadcasts
-- that are going out, since its sending as a whole paused. Each waits
-- for a teammate to resume it.
UPDATE broadcasts SET status = 'paused', pause_reason = @reason, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel = 'email' AND status = 'sending' AND deleted_at IS NULL;

-- name: ListSendingAlertRecipients :many
-- ListSendingAlertRecipients returns the active owners and admins who
-- hear that the workspace's sending paused.
SELECT sqlc.embed(u) FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.role IN ('owner', 'admin') AND m.status = 'active'
ORDER BY m.created_at, m.id;
