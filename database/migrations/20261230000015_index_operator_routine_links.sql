-- migrate: phase=expand
-- A routine's threads and suggestions are listed newest first, and a
-- suggestion is found again by its dedup key.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_threads_routine_id_created_at_index
    ON operator_threads (workspace_id, routine_id, created_at DESC) WHERE routine_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_threads_routine_run_id_index ON operator_threads (routine_run_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_changesets_routine_id_created_at_index
    ON operator_changesets (workspace_id, routine_id, created_at DESC) WHERE routine_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_changesets_routine_run_id_index
    ON operator_changesets (routine_run_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS operator_changesets_dedup_key_index
    ON operator_changesets (workspace_id, dedup_key, created_at DESC) WHERE dedup_key IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS operator_changesets_dedup_key_index;
DROP INDEX CONCURRENTLY IF EXISTS operator_changesets_routine_run_id_index;
DROP INDEX CONCURRENTLY IF EXISTS operator_changesets_routine_id_created_at_index;
DROP INDEX CONCURRENTLY IF EXISTS operator_threads_routine_run_id_index;
DROP INDEX CONCURRENTLY IF EXISTS operator_threads_routine_id_created_at_index;
