-- name: ListEmailSignatures :many
-- ListEmailSignatures returns the workspace's signatures, each member's
-- before the addresses', narrowed to one member, one address, or one
-- kind when given: personal signatures name a member, address
-- signatures do not.
SELECT * FROM email_signatures
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(member_id)::uuid IS NULL OR workspace_member_id = sqlc.narg(member_id)::uuid)
  AND (sqlc.narg(address_id)::uuid IS NULL OR email_address_id = sqlc.narg(address_id)::uuid)
  AND (
      sqlc.narg(kind)::text IS NULL
      OR (sqlc.narg(kind)::text = 'personal' AND workspace_member_id IS NOT NULL)
      OR (sqlc.narg(kind)::text = 'address' AND workspace_member_id IS NULL)
  )
ORDER BY workspace_member_id NULLS LAST, email_address_id NULLS FIRST, id
LIMIT 500;

-- name: FindEmailSignature :one
-- FindEmailSignature returns the signature with the id in the workspace.
SELECT * FROM email_signatures WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateEmailSignature :one
-- CreateEmailSignature stores a signature. One for the same member and
-- address fails on email_signatures_scope_key.
INSERT INTO email_signatures (
    workspace_id, workspace_member_id, email_address_id, body, enabled, created_at, updated_at
) VALUES (
    @workspace_id, sqlc.narg(member_id), sqlc.narg(address_id), @body, @enabled, @now, @now
)
RETURNING *;

-- name: UpdateEmailSignature :one
-- UpdateEmailSignature changes the text or whether the signature is used;
-- a missing field stays as it is.
UPDATE email_signatures SET
    body = coalesce(sqlc.narg(body)::text, body),
    enabled = coalesce(sqlc.narg(enabled)::boolean, enabled),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteEmailSignature :exec
-- DeleteEmailSignature removes the signature.
DELETE FROM email_signatures WHERE workspace_id = @workspace_id AND id = @id;

-- name: ResolveEmailSignature :one
-- ResolveEmailSignature returns the signature a reply from the address
-- carries: the member's own for that address, else the member's own for
-- every address, else the address's. Disabled signatures are passed
-- over, and so are the personal ones of a member who left. Without a
-- member only the address's signature counts.
SELECT s.* FROM email_signatures s
LEFT JOIN workspace_members m ON m.id = s.workspace_member_id
WHERE s.workspace_id = @workspace_id
  AND s.enabled
  AND (
      (s.workspace_member_id = sqlc.narg(member_id)::uuid AND m.status = 'active'
          AND (s.email_address_id = @address_id OR s.email_address_id IS NULL))
      OR (s.workspace_member_id IS NULL AND s.email_address_id = @address_id)
  )
ORDER BY s.workspace_member_id IS NULL, s.email_address_id IS NULL
LIMIT 1;
