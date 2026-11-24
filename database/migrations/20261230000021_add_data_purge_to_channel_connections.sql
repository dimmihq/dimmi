-- migrate: phase=expand
-- A chat app whose connection ended, because its app was uninstalled,
-- its tokens were revoked or a teammate disconnected it, keeps what it
-- brought in only for a grace period. The connection remembers when it
-- ended, so a nightly purge finds it once the period is over, and when
-- the purge finished, so it is never purged twice. Connecting again
-- before the purge clears the first. Slack connections disconnected
-- before this existed start their grace period now.

-- +goose Up
ALTER TABLE channel_connections
    ADD COLUMN ended_at timestamptz,
    ADD COLUMN data_purged_at timestamptz;

UPDATE channel_connections SET ended_at = now()
WHERE type = 'slack' AND status = 'disconnected' AND deleted_at IS NULL;

-- +goose Down
ALTER TABLE channel_connections
    DROP COLUMN data_purged_at,
    DROP COLUMN ended_at;
