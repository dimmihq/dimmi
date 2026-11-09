-- name: TagContactsInBulk :execrows
-- TagContactsInBulk puts one tag on many contacts at once, skipping those
-- that already carry it and those that already hold 50 tags, and counts
-- those it tagged.
INSERT INTO contact_tag (workspace_id, contact_id, tag_id, created_by_type, created_by_id, created_at)
SELECT @workspace_id, c.id, @tag_id, @created_by_type, @created_by_id, @now FROM contacts AS c
WHERE c.workspace_id = @workspace_id AND c.id = ANY (@contact_ids::uuid[])
  AND (SELECT count(*) FROM contact_tag AS x WHERE x.workspace_id = @workspace_id AND x.contact_id = c.id) < 50
ON CONFLICT (contact_id, tag_id) DO NOTHING;

-- name: ContactsByEmails :many
-- ContactsByEmails returns the live, unmerged contacts with the addresses.
SELECT id, email FROM contacts
WHERE workspace_id = @workspace_id AND email = ANY (@emails::citext[]) AND merged_into_id IS NULL
  AND deleted_at IS NULL;

-- name: ContactsByExternalIDs :many
-- ContactsByExternalIDs returns the live, unmerged contacts with the user
-- ids.
SELECT id, external_id FROM contacts
WHERE workspace_id = @workspace_id AND external_id = ANY (@external_ids::text[]) AND merged_into_id IS NULL
  AND deleted_at IS NULL;
