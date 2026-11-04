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
