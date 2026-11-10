-- name: PatchChannelConnectionConfig :exec
-- PatchChannelConnectionConfig merges keys into a connection's config,
-- leaving every other key as it is.
UPDATE channel_connections SET config = config || @patch::jsonb, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListContactConnectionConversations :many
-- ListContactConnectionConversations returns a customer's newest
-- conversations on a connection, for their own view of their requests.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND contact_id = @contact_id AND deleted_at IS NULL
ORDER BY last_message_at DESC NULLS LAST, id DESC
LIMIT @row_limit;

-- name: ListChannelMemberLinks :many
-- ListChannelMemberLinks returns every chat app user linked to a
-- teammate on the connection.
SELECT * FROM channel_member_links
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
ORDER BY created_at, id;
