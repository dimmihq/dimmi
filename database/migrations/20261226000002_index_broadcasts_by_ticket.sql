-- migrate: phase=expand
-- A broadcast started from a tracker is found by its ticket. The index is
-- built without locking broadcasts, so sending carries on meanwhile; a
-- database that already has it from an earlier build keeps it.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS broadcasts_ticket_id_index
    ON broadcasts (ticket_id) WHERE ticket_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS broadcasts_ticket_id_index;
