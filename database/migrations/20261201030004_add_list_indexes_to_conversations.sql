-- migrate: phase=expand
-- One index per way a list is read, each leading with the workspace and
-- ending in the sort key and id so a page is a short range scan however
-- large the workspace grows. Live rows only; spam and the trash have
-- their own.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY conversations_all_activity_idx
    ON conversations (workspace_id, status, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND NOT spam;
CREATE INDEX CONCURRENTLY conversations_member_activity_idx
    ON conversations (workspace_id, assignee_member_id, status, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND NOT spam;
CREATE INDEX CONCURRENTLY conversations_team_activity_idx
    ON conversations (workspace_id, assignee_team_id, status, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND NOT spam;
CREATE INDEX CONCURRENTLY conversations_inbox_activity_idx
    ON conversations (workspace_id, inbox_id, status, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND NOT spam;
CREATE INDEX CONCURRENTLY conversations_waiting_idx
    ON conversations (workspace_id, status, waiting_since, id)
    WHERE deleted_at IS NULL AND NOT spam AND waiting_since IS NOT NULL;
CREATE INDEX CONCURRENTLY conversations_created_idx
    ON conversations (workspace_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY conversations_spam_idx
    ON conversations (workspace_id, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND spam;
CREATE INDEX CONCURRENTLY conversations_deleted_idx
    ON conversations (workspace_id, deleted_at DESC, id DESC)
    WHERE deleted_at IS NOT NULL;
CREATE INDEX CONCURRENTLY conversations_search_idx ON conversations USING gin (workspace_id, search);
CREATE INDEX CONCURRENTLY conversations_subject_trgm_idx
    ON conversations USING gin (workspace_id, dimmi_unaccent(lower(coalesce(subject, ''))) gin_trgm_ops);
CREATE INDEX CONCURRENTLY conversations_thread_lookup_idx
    ON conversations (channel_connection_id, external_thread_key, created_at DESC)
    WHERE external_thread_key IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY conversations_thread_lookup_idx;
DROP INDEX CONCURRENTLY conversations_subject_trgm_idx;
DROP INDEX CONCURRENTLY conversations_search_idx;
DROP INDEX CONCURRENTLY conversations_deleted_idx;
DROP INDEX CONCURRENTLY conversations_spam_idx;
DROP INDEX CONCURRENTLY conversations_created_idx;
DROP INDEX CONCURRENTLY conversations_waiting_idx;
DROP INDEX CONCURRENTLY conversations_inbox_activity_idx;
DROP INDEX CONCURRENTLY conversations_team_activity_idx;
DROP INDEX CONCURRENTLY conversations_member_activity_idx;
DROP INDEX CONCURRENTLY conversations_all_activity_idx;
