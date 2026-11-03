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
