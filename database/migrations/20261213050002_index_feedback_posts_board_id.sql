-- migrate: phase=expand
-- Requests are listed a board at a time, by status and votes.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS feedback_posts_board_id_status_index
    ON feedback_posts (board_id, status, vote_count DESC) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS feedback_posts_board_id_status_index;
