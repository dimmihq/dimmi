-- migrate: phase=contract
-- Priority becomes five levels, none to urgent, stored as a small number
-- so sorting is plain index order. Nothing has launched, so the flag goes
-- in one step: what was true becomes high.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE conversations RENAME COLUMN priority TO priority_flag;
ALTER TABLE conversations
    ADD COLUMN priority smallint NOT NULL DEFAULT 0
    CONSTRAINT conversations_priority_check CHECK (priority BETWEEN 0 AND 4);
UPDATE conversations SET priority = 3 WHERE priority_flag;
ALTER TABLE conversations DROP COLUMN priority_flag;
CREATE INDEX CONCURRENTLY conversations_priority_idx
    ON conversations (workspace_id, status, priority DESC, last_message_at DESC, id DESC)
    WHERE deleted_at IS NULL AND NOT spam AND priority > 0;

-- +goose Down
ALTER TABLE conversations RENAME COLUMN priority TO priority_level;
ALTER TABLE conversations ADD COLUMN priority boolean NOT NULL DEFAULT false;
UPDATE conversations SET priority = priority_level > 0;
ALTER TABLE conversations DROP COLUMN priority_level;
