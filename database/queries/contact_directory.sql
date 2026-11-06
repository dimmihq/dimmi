-- name: ListContacts :many
-- ListContacts returns a page of the workspace's live contacts that were
-- not merged away, narrowed by kind, email, external id, blocked state,
-- company, tags, when they were last seen or added, custom attributes and
-- a search of names, emails, phones and external ids, most recently seen
-- first, newest first or by name.
SELECT c.* FROM contacts AS c
WHERE c.workspace_id = @workspace_id
  AND c.merged_into_id IS NULL
  AND c.deleted_at IS NULL
  AND (sqlc.narg(kind)::text IS NULL OR c.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(email)::citext IS NULL OR c.email = sqlc.narg(email)::citext)
  AND (sqlc.narg(external_id)::text IS NULL OR c.external_id = sqlc.narg(external_id)::text)
  AND (sqlc.narg(blocked)::boolean IS NULL OR (c.blocked_at IS NOT NULL) = sqlc.narg(blocked)::boolean)
  AND (sqlc.narg(company_id)::uuid IS NULL OR c.company_id = sqlc.narg(company_id)::uuid)
  AND (sqlc.narg(last_seen_after)::timestamptz IS NULL OR c.last_seen_at >= sqlc.narg(last_seen_after)::timestamptz)
  AND (sqlc.narg(last_seen_before)::timestamptz IS NULL OR c.last_seen_at < sqlc.narg(last_seen_before)::timestamptz)
  AND (sqlc.narg(created_after)::timestamptz IS NULL OR c.created_at >= sqlc.narg(created_after)::timestamptz)
  AND (sqlc.narg(created_before)::timestamptz IS NULL OR c.created_at < sqlc.narg(created_before)::timestamptz)
  AND (sqlc.narg(custom)::jsonb IS NULL OR c.custom_attributes @> sqlc.narg(custom)::jsonb)
  AND (
      coalesce(cardinality(@tag_ids::uuid[]), 0) = 0
      OR c.id IN (
          SELECT ct.contact_id FROM contact_tag ct
          WHERE ct.workspace_id = @workspace_id AND ct.tag_id = ANY (@tag_ids::uuid[])
          GROUP BY ct.contact_id
          HAVING count(*) = cardinality(@tag_ids::uuid[])
      )
  )
  AND (
      @search::text = ''
      OR c.search @@ to_tsquery('simple', @terms::text)
      OR c.name ILIKE '%' || @search::text || '%'
      OR c.email ILIKE @search::text || '%'
  )
ORDER BY
    CASE WHEN @sort::text = 'name' THEN lower(c.name) END,
    CASE WHEN @sort::text = 'created_at' THEN c.created_at END DESC,
    CASE WHEN @sort::text = 'last_seen_at' THEN coalesce(c.last_seen_at, c.created_at) END DESC,
    c.id DESC
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
    trusted_attribute_keys = @trusted_attribute_keys, blocked_at = sqlc.narg(blocked_at),
    notes = sqlc.narg(notes), company_id = sqlc.narg(company_id), email_verified = @email_verified, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: LockContact :one
-- LockContact returns the contact and holds its row until the
-- transaction ends.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ContactsByIDs :many
-- ContactsByIDs returns the contacts of the workspace with the ids.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: FindUpsertCandidateByEmail :one
-- FindUpsertCandidateByEmail returns the contact a trusted upsert may
-- attach an external id to: live, unmerged, without an external id, and a
-- user or a contact whose address was proven, users first, then the one
-- seen most recently.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id
  AND email = @email
  AND external_id IS NULL
  AND merged_into_id IS NULL
  AND deleted_at IS NULL
  AND (kind = 'user' OR email_verified)
ORDER BY (kind = 'user') DESC, last_seen_at DESC NULLS LAST, id DESC
LIMIT 1;

-- name: ListUnverifiedEmailLeads :many
-- ListUnverifiedEmailLeads returns live leads who only typed the address,
-- never chosen by an upsert and offered for a manual merge instead.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id
  AND email = @email
  AND id <> @id
  AND kind = 'lead'
  AND NOT email_verified
  AND merged_into_id IS NULL
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT 20;

-- name: CountContactConversations :one
-- CountContactConversations returns how many conversations, trashed ones
-- included, the contact has.
SELECT count(*) FROM conversations WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: SoftDeleteContact :exec
-- SoftDeleteContact hides a contact.
UPDATE contacts SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindDuplicateContacts :many
-- FindDuplicateContacts returns live contacts that look like the same
-- person: the same address without regard to case, the same phone number,
-- a messaging identity on that number, or the same name at the same
-- company, the weakest signal, which sorts last.
SELECT
    sqlc.embed(c),
    COALESCE(c.email = sqlc.narg(email)::citext, false)::boolean AS same_email,
    COALESCE(c.phone = sqlc.narg(phone)::text, false)::boolean AS same_phone,
    COALESCE(sqlc.narg(phone)::text IS NOT NULL AND EXISTS (
        SELECT 1 FROM contact_identities AS i
        WHERE i.workspace_id = c.workspace_id AND i.contact_id = c.id AND i.type = 'whatsapp'
          AND i.value = ltrim(sqlc.narg(phone)::text, '+')
    ), false)::boolean AS same_whatsapp,
    COALESCE(c.name = sqlc.narg(name)::text AND c.company_id = sqlc.narg(company_id)::uuid, false)::boolean
        AS same_name_company
FROM contacts AS c
WHERE c.workspace_id = @workspace_id
  AND c.id <> @id
  AND c.merged_into_id IS NULL
  AND c.deleted_at IS NULL
  AND (
      (sqlc.narg(email)::citext IS NOT NULL AND c.email = sqlc.narg(email)::citext)
      OR (sqlc.narg(phone)::text IS NOT NULL AND c.phone = sqlc.narg(phone)::text)
      OR (sqlc.narg(phone)::text IS NOT NULL AND EXISTS (
          SELECT 1 FROM contact_identities AS i
          WHERE i.workspace_id = c.workspace_id AND i.contact_id = c.id AND i.type = 'whatsapp'
            AND i.value = ltrim(sqlc.narg(phone)::text, '+')
      ))
      OR (sqlc.narg(name)::text IS NOT NULL AND sqlc.narg(company_id)::uuid IS NOT NULL
          AND c.name = sqlc.narg(name)::text AND c.company_id = sqlc.narg(company_id)::uuid)
  )
ORDER BY same_email DESC, same_phone DESC, same_whatsapp DESC, c.id DESC
LIMIT @row_limit::bigint;
