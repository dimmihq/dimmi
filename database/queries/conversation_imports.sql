-- name: FindConversationByImportExternalID :one
-- FindConversationByImportExternalID returns the conversation brought in
-- from another tool under the id it had there, deleted or not.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND source ? 'external_id' AND source ->> 'external_id' = @external_id::text;
