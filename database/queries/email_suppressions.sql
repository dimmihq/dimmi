-- name: SuppressEmail :exec
-- SuppressEmail stops mail to the address for the scope. Suppressing it
-- again never turns a lasting suppression into one that lapses: a
-- lasting reason replaces a lapsing one, and two lapsing ones keep the
-- later end.
INSERT INTO email_suppressions (
    workspace_id, email, reason, scope, source, details, outbound_email_id, expires_at, created_at, updated_at
) VALUES (
    @workspace_id, @email, @reason, @scope, @source, @details, sqlc.narg(outbound_email_id), sqlc.narg(expires_at),
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (workspace_id, email, scope) DO UPDATE SET
    reason = CASE WHEN excluded.expires_at IS NULL THEN excluded.reason ELSE email_suppressions.reason END,
    expires_at = CASE
        WHEN email_suppressions.expires_at IS NULL OR excluded.expires_at IS NULL THEN NULL
        ELSE greatest(email_suppressions.expires_at, excluded.expires_at)
    END,
    updated_at = excluded.updated_at;

-- name: LiftBounceSuppressions :exec
-- LiftBounceSuppressions lets mail reach the address again after it
-- bounced, once its owner has written in from it.
DELETE FROM email_suppressions
WHERE workspace_id = @workspace_id AND email = @email AND reason IN ('hard_bounce', 'soft_bounce');

-- name: ListSuppressedEmails :many
-- ListSuppressedEmails returns which of the addresses may not be mailed
-- at all right now.
SELECT email::text AS email FROM email_suppressions
WHERE workspace_id = @workspace_id AND email = ANY (@emails::citext[]) AND scope = 'all'
  AND (expires_at IS NULL OR expires_at > @now::timestamptz);

-- name: ListContactSuppressions :many
-- ListContactSuppressions returns the live suppressions on the contact's
-- address, newest first.
SELECT s.* FROM email_suppressions s
JOIN contacts c ON c.workspace_id = s.workspace_id AND c.email = s.email
WHERE s.workspace_id = @workspace_id AND c.id = @contact_id
  AND (s.expires_at IS NULL OR s.expires_at > @now::timestamptz)
ORDER BY s.created_at DESC;

-- name: FindEmailSuppression :one
-- FindEmailSuppression returns one suppression of the workspace.
SELECT * FROM email_suppressions WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteEmailSuppression :exec
-- DeleteEmailSuppression lets mail reach the address again.
DELETE FROM email_suppressions WHERE workspace_id = @workspace_id AND id = @id;
