-- migrate: phase=expand
-- Tag filters and unread checks read these pivots by workspace first. The
-- pivots already carry workspace_id; only the indexes were missing.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY conversation_tag_workspace_id_tag_id_conversation_id_index
    ON conversation_tag (workspace_id, tag_id, conversation_id);
CREATE INDEX CONCURRENTLY conversation_reads_workspace_id_member_id_conversation_id_index
    ON conversation_reads (workspace_id, workspace_member_id, conversation_id);

-- +goose Down
DROP INDEX CONCURRENTLY conversation_reads_workspace_id_member_id_conversation_id_index;
DROP INDEX CONCURRENTLY conversation_tag_workspace_id_tag_id_conversation_id_index;
