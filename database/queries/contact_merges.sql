-- name: LockContactsForMerge :many
-- LockContactsForMerge locks the two contacts of a merge in id order, so
-- two merges over the same people wait for each other.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[])
ORDER BY id
FOR UPDATE;

-- name: RetireMergedContact :exec
-- RetireMergedContact points a contact merged away at the survivor and
-- frees its external id for it. The row stays, so old links resolve.
UPDATE contacts SET merged_into_id = @target_id::uuid, external_id = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @source_id::uuid;

-- name: SaveMergedContact :one
-- SaveMergedContact stores the survivor's details after a merge.
UPDATE contacts SET
    kind = @kind, external_id = @external_id, email = @email, email_verified = @email_verified, name = @name,
    phone = @phone, avatar_url = @avatar_url, company_id = @company_id, custom_attributes = @custom_attributes,
    trusted_attribute_keys = @trusted_attribute_keys::text[], locale = @locale, timezone = @timezone,
    location = @location, first_seen_at = @first_seen_at, last_seen_at = @last_seen_at, notes = @notes,
    placeholder = @placeholder,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MoveContactConversations :execrows
-- MoveContactConversations hands every conversation of one contact,
-- trashed ones too, to another.
UPDATE conversations SET contact_id = @target_id::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND contact_id = @source_id::uuid;

-- name: MoveContactParticipations :exec
-- MoveContactParticipations moves where one contact took part to
-- another, dropping what the other already has.
INSERT INTO conversation_participants (workspace_id, conversation_id, contact_id, role, created_at)
SELECT p.workspace_id, p.conversation_id, @target_id::uuid, p.role, p.created_at
FROM conversation_participants p
JOIN conversations c ON c.id = p.conversation_id
WHERE p.workspace_id = @workspace_id AND p.contact_id = @source_id::uuid AND c.contact_id <> @target_id::uuid
ON CONFLICT (conversation_id, contact_id) DO NOTHING;

-- name: DropContactParticipations :exec
-- DropContactParticipations removes a merged contact's places in
-- conversations once the survivor holds them.
DELETE FROM conversation_participants WHERE workspace_id = @workspace_id AND contact_id = @source_id::uuid;

-- name: MoveContactIdentities :execrows
-- MoveContactIdentities hands one contact's identities to another.
UPDATE contact_identities SET contact_id = @target_id::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND contact_id = @source_id::uuid;

-- name: MoveContactTags :exec
-- MoveContactTags puts one contact's tags on another, skipping those it
-- has, and takes them off the first.
WITH moved AS (
    DELETE FROM contact_tag AS d WHERE d.workspace_id = @workspace_id AND d.contact_id = @source_id::uuid
    RETURNING d.*
)
INSERT INTO contact_tag (workspace_id, contact_id, tag_id, created_by_type, created_by_id, created_at)
SELECT moved.workspace_id, @target_id::uuid, moved.tag_id, moved.created_by_type, moved.created_by_id,
    moved.created_at
FROM moved
ON CONFLICT (contact_id, tag_id) DO NOTHING;

-- name: FoldContactEventSummaries :exec
-- FoldContactEventSummaries adds one contact's event summaries to
-- another's: counts add up, the first time is the earlier and the last
-- the later, and the daily counts and metadata follow the newer side.
-- The first contact's summaries are removed.
WITH moved AS (
    DELETE FROM contact_event_summaries AS d
    WHERE d.workspace_id = @workspace_id AND d.contact_id = @source_id::uuid
    RETURNING d.*
)
INSERT INTO contact_event_summaries AS t (workspace_id, contact_id, event_definition_id, count, first_at, last_at,
    daily_day, daily, last_metadata, updated_at)
SELECT moved.workspace_id, @target_id::uuid, moved.event_definition_id, moved.count, moved.first_at,
    moved.last_at, moved.daily_day, moved.daily, moved.last_metadata, @now::timestamptz
FROM moved
ON CONFLICT (workspace_id, contact_id, event_definition_id) DO UPDATE SET
    count = t.count + EXCLUDED.count,
    first_at = least(t.first_at, EXCLUDED.first_at),
    last_at = greatest(t.last_at, EXCLUDED.last_at),
    daily_day = CASE WHEN EXCLUDED.last_at > t.last_at THEN EXCLUDED.daily_day ELSE t.daily_day END,
    daily = CASE WHEN EXCLUDED.last_at > t.last_at THEN EXCLUDED.daily ELSE t.daily END,
    last_metadata = CASE WHEN EXCLUDED.last_at > t.last_at THEN EXCLUDED.last_metadata ELSE t.last_metadata END,
    updated_at = EXCLUDED.updated_at;
