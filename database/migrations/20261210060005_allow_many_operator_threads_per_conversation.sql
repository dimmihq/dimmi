-- migrate: phase=contract
-- A teammate may keep several threads with Operator about one
-- conversation, starting a fresh one when the subject changes, so a
-- member and a conversation no longer name one thread. The plain index
-- that lists a conversation's threads goes in first, then the unique one
-- goes. Going down keeps each member's latest thread per conversation and
-- restores the uniqueness.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_threads_member_id_conversation_id_index
    ON operator_threads (member_id, conversation_id, updated_at DESC) WHERE conversation_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS operator_threads_member_id_conversation_id_key;

-- +goose Down
DELETE FROM operator_threads t
USING operator_threads newer
WHERE t.conversation_id IS NOT NULL AND newer.member_id = t.member_id
  AND newer.conversation_id = t.conversation_id
  AND (newer.updated_at, newer.id) > (t.updated_at, t.id);
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS operator_threads_member_id_conversation_id_key
    ON operator_threads (member_id, conversation_id) WHERE conversation_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS operator_threads_member_id_conversation_id_index;
