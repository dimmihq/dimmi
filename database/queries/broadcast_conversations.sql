-- name: BroadcastConversationPage :many
-- BroadcastConversationPage reads the next page of conversations a
-- conversation broadcast answers in, with the customer of each: the
-- conversations linked to the tracker when one is named, otherwise every
-- conversation in the workspace. Each customer is answered once, in the
-- newest of their conversations in scope. Spam, deleted, merged and test
-- conversations are never answered.
WITH scope AS (
    SELECT s.id, s.contact_id FROM conversations s
    WHERE s.workspace_id = @workspace_id
      AND s.deleted_at IS NULL AND NOT s.spam AND s.merged_into_id IS NULL AND NOT s.is_test
      AND (sqlc.narg(ticket_id)::uuid IS NULL OR s.id IN (
          SELECT l.conversation_id FROM ticket_links l
          WHERE l.workspace_id = @workspace_id AND l.kind = 'tracker' AND l.from_ticket_id = sqlc.narg(ticket_id)::uuid
            AND l.conversation_id IS NOT NULL
      ))
)
SELECT sqlc.embed(c), sqlc.embed(p)
FROM scope s
JOIN conversations c ON c.workspace_id = @workspace_id AND c.id = s.id
JOIN contacts p ON p.workspace_id = @workspace_id AND p.id = s.contact_id AND p.deleted_at IS NULL
WHERE s.id > @after::uuid
  AND NOT EXISTS (SELECT 1 FROM scope n WHERE n.contact_id = s.contact_id AND n.id > s.id)
ORDER BY s.id
LIMIT @page_size;
