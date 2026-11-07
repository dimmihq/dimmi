-- migrate: phase=expand
-- The bell reads a member's notifications in one workspace, newest change
-- first, and counts the unread ones; both get an index. The table already
-- exists, so the indexes are built without locking out writes.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS notifications_tenant_recipient_index
    ON notifications (tenant_id, notifiable_id, updated_at DESC, id DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS notifications_tenant_unread_index
    ON notifications (tenant_id, notifiable_id) WHERE read_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS notifications_tenant_unread_index;
DROP INDEX CONCURRENTLY IF EXISTS notifications_tenant_recipient_index;
