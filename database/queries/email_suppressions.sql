-- name: SuppressEmail :exec
-- SuppressEmail stops mail to the address for the scope. Suppressing it
-- again keeps the first reason and refreshes when it lapses.
INSERT INTO email_suppressions (
    workspace_id, email, reason, scope, source, details, outbound_email_id, expires_at, created_at, updated_at
) VALUES (
    @workspace_id, @email, @reason, @scope, @source, @details, sqlc.narg(outbound_email_id), sqlc.narg(expires_at),
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (workspace_id, email, scope) DO UPDATE SET expires_at = excluded.expires_at, updated_at = excluded.updated_at;

-- name: ListSuppressedEmails :many
-- ListSuppressedEmails returns which of the addresses may not be mailed
-- at all right now.
SELECT email::text AS email FROM email_suppressions
WHERE workspace_id = @workspace_id AND email = ANY (@emails::citext[]) AND scope = 'all'
  AND (expires_at IS NULL OR expires_at > @now::timestamptz);
