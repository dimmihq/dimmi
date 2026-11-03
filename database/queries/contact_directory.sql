-- name: ListContacts :many
-- ListContacts returns a page of the workspace's live contacts that were
-- not merged away, narrowed by kind, email, external id, blocked state
-- and a search of names, emails, phones and external ids, most recently
-- seen first, newest first or by name.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id
  AND merged_into_id IS NULL
  AND deleted_at IS NULL
  AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(email)::citext IS NULL OR email = sqlc.narg(email)::citext)
  AND (sqlc.narg(external_id)::text IS NULL OR external_id = sqlc.narg(external_id)::text)
  AND (sqlc.narg(blocked)::boolean IS NULL OR (blocked_at IS NOT NULL) = sqlc.narg(blocked)::boolean)
  AND (
      @search::text = ''
      OR search @@ to_tsquery('simple', @terms::text)
      OR name ILIKE '%' || @search::text || '%'
      OR email ILIKE @search::text || '%'
  )
ORDER BY
    CASE WHEN @sort::text = 'name' THEN lower(name) END,
    CASE WHEN @sort::text = 'created_at' THEN created_at END DESC,
    CASE WHEN @sort::text = 'last_seen_at' THEN coalesce(last_seen_at, created_at) END DESC,
    id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: FindContactForEmail :one
-- FindContactForEmail returns the contact a trusted email address
-- belongs to: a live, unmerged user or verified contact with it, users
-- first, then the one seen most recently. A lead who only typed the
-- address is never chosen, since typing proves nothing.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id
  AND email = @email
  AND merged_into_id IS NULL
  AND deleted_at IS NULL
  AND (kind = 'user' OR email_verified)
ORDER BY (kind = 'user') DESC, last_seen_at DESC NULLS LAST, id DESC
LIMIT 1;

-- name: UpdateContact :one
-- UpdateContact writes the contact's editable fields as given.
UPDATE contacts
SET kind = @kind, external_id = sqlc.narg(external_id), email = sqlc.narg(email), name = sqlc.narg(name),
    phone = sqlc.narg(phone), custom_attributes = @custom_attributes,
    trusted_attribute_keys = @trusted_attribute_keys, blocked_at = sqlc.narg(blocked_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: LockContact :one
-- LockContact returns the contact and holds its row until the
-- transaction ends.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ContactsByIDs :many
-- ContactsByIDs returns the contacts of the workspace with the ids.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);
