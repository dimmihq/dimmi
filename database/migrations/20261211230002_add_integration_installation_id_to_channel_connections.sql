-- migrate: phase=expand
-- A chat channel backed by an integration points at its installation
-- instead of keeping its own copy of the tokens, so disconnecting the
-- installation reaches the channel and a reinstall picks up fresh
-- tokens with no change to the channel.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE channel_connections
    ADD COLUMN IF NOT EXISTS integration_installation_id uuid
        REFERENCES integration_installations ON DELETE SET NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS channel_connections_integration_installation_id_index
    ON channel_connections (integration_installation_id);

-- +goose Down
DROP INDEX IF EXISTS channel_connections_integration_installation_id_index;
ALTER TABLE channel_connections DROP COLUMN IF EXISTS integration_installation_id;
