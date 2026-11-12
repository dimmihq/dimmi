-- name: MoveConversationMessages :execrows
-- MoveConversationMessages hands every entry of one conversation's
-- timeline to another. Entries keep their times, so the target reads
-- them in the order they happened.
UPDATE messages SET conversation_id = @target_id::uuid
WHERE workspace_id = @workspace_id AND conversation_id = @source_id::uuid;

-- name: MoveConversationMentions :exec
-- MoveConversationMentions moves the mentions of the moved entries along
-- with them.
UPDATE message_mentions SET conversation_id = @target_id::uuid
WHERE workspace_id = @workspace_id AND conversation_id = @source_id::uuid;

-- name: MoveConversationMessageRefs :exec
-- MoveConversationMessageRefs moves the mail ids one conversation is
-- known by to another, so a reply to an old mail lands in the target. A
-- mail id the target already knows stays where it was.
UPDATE email_message_refs r SET conversation_id = @target_id::uuid
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @source_id::uuid
  AND NOT EXISTS (
      SELECT 1 FROM email_message_refs t
      WHERE t.workspace_id = r.workspace_id AND t.rfc_message_id = r.rfc_message_id
        AND t.conversation_id = @target_id::uuid
  );

-- name: CopyConversationParticipants :exec
-- CopyConversationParticipants adds the people of one conversation to
-- another, and the source's own customer when they are someone else.
INSERT INTO conversation_participants (workspace_id, conversation_id, contact_id, role, created_at)
SELECT p.workspace_id, @target_id::uuid, p.contact_id, p.role, @now::timestamptz
FROM conversation_participants p
WHERE p.workspace_id = @workspace_id AND p.conversation_id = @source_id::uuid
  AND p.contact_id <> @target_contact_id::uuid
UNION
SELECT @workspace_id, @target_id::uuid, @source_contact_id::uuid, 'participant', @now::timestamptz
WHERE @source_contact_id::uuid <> @target_contact_id::uuid
ON CONFLICT (conversation_id, contact_id) DO NOTHING;

-- name: FoldConversationActivity :one
-- FoldConversationActivity counts a merged conversation's messages into
-- the target: the counts add up, the first times take the earlier one
-- and the last times the later one. The customer still waits when their
-- last message came after the last reply.
WITH s AS (
    SELECT * FROM conversations AS src WHERE src.workspace_id = @workspace_id AND src.id = @source_id::uuid
)
UPDATE conversations AS t SET
    message_count = t.message_count + (SELECT s.message_count FROM s),
    last_message_id = CASE
        WHEN (SELECT s.last_message_at FROM s) > coalesce(t.last_message_at, '-infinity'::timestamptz)
        THEN (SELECT s.last_message_id FROM s) ELSE t.last_message_id END,
    last_message_at = greatest(t.last_message_at, (SELECT s.last_message_at FROM s)),
    last_activity_at = greatest(t.last_activity_at, (SELECT s.last_activity_at FROM s), @now::timestamptz),
    first_customer_message_at = least(t.first_customer_message_at, (SELECT s.first_customer_message_at FROM s)),
    last_customer_message_at = greatest(t.last_customer_message_at, (SELECT s.last_customer_message_at FROM s)),
    first_reply_at = least(t.first_reply_at, (SELECT s.first_reply_at FROM s)),
    last_reply_at = greatest(t.last_reply_at, (SELECT s.last_reply_at FROM s)),
    waiting_since = CASE
        WHEN greatest(t.last_customer_message_at, (SELECT s.last_customer_message_at FROM s))
            > coalesce(greatest(t.last_reply_at, (SELECT s.last_reply_at FROM s)), '-infinity'::timestamptz)
        THEN least(t.waiting_since, (SELECT s.waiting_since FROM s)) ELSE NULL END,
    updated_at = @now::timestamptz
WHERE t.workspace_id = @workspace_id AND t.id = @target_id::uuid
RETURNING *;

-- name: RetireMergedConversation :one
-- RetireMergedConversation closes a conversation merged away, empties
-- its counts, points it at the target and moves it to the trash.
UPDATE conversations SET
    merged_into_id = @target_id::uuid, status = 'closed', closed_at = coalesce(closed_at, @now::timestamptz),
    snoozed_until = NULL, waiting_since = NULL, message_count = 0, last_message_id = NULL,
    deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @source_id::uuid
RETURNING *;
