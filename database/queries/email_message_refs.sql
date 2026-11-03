-- name: CreateEmailMessageRef :exec
-- CreateEmailMessageRef files a Message-ID under a conversation. Filing
-- it again changes nothing, except that an id first seen only as
-- referenced takes the message once its own mail arrives.
INSERT INTO email_message_refs (workspace_id, rfc_message_id, conversation_id, message_id, kind, created_at)
VALUES (@workspace_id, @rfc_message_id, @conversation_id, sqlc.narg(message_id), @kind, @now::timestamptz)
ON CONFLICT (workspace_id, rfc_message_id, conversation_id) DO UPDATE
SET message_id = coalesce(email_message_refs.message_id, excluded.message_id),
    kind = CASE WHEN email_message_refs.kind = 'referenced' THEN excluded.kind ELSE email_message_refs.kind END;

-- name: FindRefConversations :many
-- FindRefConversations returns the live conversations any of the ids is
-- filed under, of the kinds given, open ones first and then the most
-- recently active.
SELECT DISTINCT ON (c.id) c.id, c.status, c.closed_at, c.last_message_at, c.channel_connection_id,
    c.external_thread_key, c.contact_id, r.kind
FROM email_message_refs AS r
JOIN conversations AS c ON c.id = r.conversation_id AND c.workspace_id = r.workspace_id
WHERE r.workspace_id = @workspace_id AND r.rfc_message_id = ANY (@rfc_message_ids::text[])
  AND r.kind = ANY (@kinds::text[]) AND c.deleted_at IS NULL
ORDER BY c.id;

-- name: CountOutboundRefs :one
-- CountOutboundRefs counts how many of the ids are mail the workspace
-- sent itself, which a copy coming back to it carries.
SELECT count(*) FROM email_message_refs
WHERE workspace_id = @workspace_id AND rfc_message_id = ANY (@rfc_message_ids::text[])
  AND kind IN ('outbound', 'outbound_provider');

-- name: ListConversationRefs :many
-- ListConversationRefs returns the Message-IDs of the conversation's own
-- mail, sent and received, oldest first, for the threading headers of
-- the next reply.
SELECT rfc_message_id, kind, created_at FROM email_message_refs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND kind IN ('inbound', 'outbound')
ORDER BY created_at, rfc_message_id;
