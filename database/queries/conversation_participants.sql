-- name: AddConversationParticipant :one
-- AddConversationParticipant puts a contact on a conversation with a
-- role. Adding a contact already on it changes nothing and returns the
-- row as it was.
INSERT INTO conversation_participants (workspace_id, conversation_id, contact_id, role, created_at)
VALUES (@workspace_id, @conversation_id, @contact_id, @role, @now)
ON CONFLICT (conversation_id, contact_id) DO UPDATE SET role = conversation_participants.role
RETURNING *;

-- name: ListConversationParticipants :many
-- ListConversationParticipants returns the conversation's participants
-- in the order they joined.
SELECT * FROM conversation_participants
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY created_at, contact_id;

-- name: ListConversationCopies :many
-- ListConversationCopies returns who a reply copies: everyone on the
-- conversation besides its contact who still has an email address, in
-- the order they joined.
SELECT c.id, c.name, c.email::text AS email
FROM conversation_participants AS p
JOIN contacts AS c ON c.id = p.contact_id AND c.workspace_id = p.workspace_id
WHERE p.workspace_id = @workspace_id AND p.conversation_id = @conversation_id
  AND p.contact_id <> @contact_id AND c.email IS NOT NULL AND c.deleted_at IS NULL AND c.merged_into_id IS NULL
ORDER BY p.created_at, p.contact_id
LIMIT 50;
