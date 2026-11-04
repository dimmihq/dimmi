-- migrate: phase=expand
-- A message remembers the connection it travelled on, so a reply follows
-- the customer's last channel and a delivery receipt finds its message.
-- The timeline index gains the id, so two messages in one instant keep a
-- stable order.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE messages ADD COLUMN channel_connection_id uuid REFERENCES channel_connections ON DELETE SET NULL;
CREATE INDEX CONCURRENTLY messages_workspace_id_conversation_id_created_at_id_index
    ON messages (workspace_id, conversation_id, created_at, id);
CREATE INDEX CONCURRENTLY messages_channel_message_lookup_idx
    ON messages (workspace_id, channel_connection_id, channel_message_id)
    WHERE channel_message_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY messages_channel_message_lookup_idx;
DROP INDEX CONCURRENTLY messages_workspace_id_conversation_id_created_at_id_index;
ALTER TABLE messages DROP COLUMN channel_connection_id;
