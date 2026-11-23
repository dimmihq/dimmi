-- name: SuppressEmail :exec
-- SuppressEmail stops mail to the address for the scope. Suppressing it
-- again never turns a lasting suppression into one that lapses: a
-- lasting reason replaces a lapsing one, and two lapsing ones keep the
-- later end.
INSERT INTO email_suppressions (
    workspace_id, email, reason, scope, source, details, outbound_email_id, expires_at, created_at, updated_at
) VALUES (
    @workspace_id, @email::citext, @reason, @scope, @source, @details, sqlc.narg(outbound_email_id), sqlc.narg(expires_at),
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
WHERE workspace_id = @workspace_id AND email = @email::citext AND reason IN ('hard_bounce', 'soft_bounce');

-- name: ListSuppressedEmails :many
-- ListSuppressedEmails returns which of the addresses may not be mailed
-- at all right now, as they were asked about. A suppression kept as a
-- hash after its customer was erased stops the address like any other.
SELECT e.email::text AS email FROM unnest(@emails::citext[]) AS e (email)
WHERE EXISTS (
    SELECT 1 FROM email_suppressions s
    WHERE s.workspace_id = @workspace_id AND s.scope = 'all'
      AND (s.expires_at IS NULL OR s.expires_at > @now::timestamptz)
      AND (s.email = e.email OR s.email_hash = dimmi_email_suppression_hash(@workspace_id, e.email::text))
);

-- name: ListContactSuppressions :many
-- ListContactSuppressions returns the live suppressions on the contact's
-- address, newest first, with the address they were found by: one kept
-- as a hash no longer holds it.
SELECT sqlc.embed(s), c.email::text AS contact_email FROM email_suppressions s
JOIN contacts c ON c.workspace_id = s.workspace_id
    AND (c.email = s.email OR s.email_hash = dimmi_email_suppression_hash(c.workspace_id, c.email::text))
WHERE s.workspace_id = @workspace_id AND c.id = @contact_id
  AND (s.expires_at IS NULL OR s.expires_at > @now::timestamptz)
ORDER BY s.created_at DESC;

-- name: FindEmailSuppression :one
-- FindEmailSuppression returns one suppression of the workspace.
SELECT * FROM email_suppressions WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteEmailSuppression :exec
-- DeleteEmailSuppression lets mail reach the address again.
DELETE FROM email_suppressions WHERE workspace_id = @workspace_id AND id = @id;

-- name: HashErasedSuppressions :execrows
-- HashErasedSuppressions keeps the erased customer's wish to hear nothing
-- more without keeping their address: suppressions made because they
-- unsubscribed or complained hold a hash in its place and lose every
-- other detail. One already kept for the same address and scope is left
-- as it is.
UPDATE email_suppressions s
SET email = NULL, email_hash = dimmi_email_suppression_hash(s.workspace_id, s.email::text), details = NULL,
    outbound_email_id = NULL, created_by_member_id = NULL, expires_at = NULL, updated_at = @now::timestamptz
WHERE s.workspace_id = @workspace_id AND s.email = ANY (@emails::citext[])
  AND s.reason IN ('unsubscribed', 'complaint')
  AND NOT EXISTS (
      SELECT 1 FROM email_suppressions kept
      WHERE kept.workspace_id = s.workspace_id AND kept.scope = s.scope
        AND kept.email_hash = dimmi_email_suppression_hash(s.workspace_id, s.email::text)
  );

-- name: DeleteErasedSuppressions :execrows
-- DeleteErasedSuppressions removes what is left of the erased customer's
-- suppressions that still name their address.
DELETE FROM email_suppressions WHERE workspace_id = @workspace_id AND email = ANY (@emails::citext[]);
