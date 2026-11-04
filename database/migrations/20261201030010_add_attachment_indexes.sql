-- migrate: phase=expand
-- Files nobody linked to a message are purged by a job that sweeps every
-- workspace by age, the documented exception to leading with the workspace.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY attachments_orphan_idx ON attachments (created_at) WHERE message_id IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY attachments_orphan_idx;
