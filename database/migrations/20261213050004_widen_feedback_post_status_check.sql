-- migrate: phase=contract
-- A request's status is any key of its workspace's statuses, so the
-- check that held it to the five built-in ones goes. The application
-- checks the key against the workspace's statuses.

-- +goose Up
ALTER TABLE feedback_posts DROP CONSTRAINT IF EXISTS feedback_posts_status_check;
ALTER TABLE feedback_posts ADD CONSTRAINT feedback_posts_status_key_check
    CHECK (status ~ '^[a-z0-9_]{1,40}$') NOT VALID;
ALTER TABLE feedback_posts VALIDATE CONSTRAINT feedback_posts_status_key_check;

-- +goose Down
SELECT 1;
