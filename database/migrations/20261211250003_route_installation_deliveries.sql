-- migrate: phase=contract
-- A Slack installation made only for notifications or Operator has no
-- channel connection, yet its button presses, mentions and direct
-- messages still arrive and must be kept and processed. A delivery is
-- kept for its connection, or for its installation when there is none,
-- and always for one of them. The new check is added unvalidated and
-- validated after, so writers are never blocked while rows are read.

-- +goose Up
ALTER TABLE channel_inbound_events ALTER COLUMN channel_connection_id DROP NOT NULL;
ALTER TABLE channel_inbound_events
    ADD COLUMN integration_installation_id uuid REFERENCES integration_installations ON DELETE CASCADE;
ALTER TABLE channel_inbound_events ADD CONSTRAINT channel_inbound_events_owner_check
    CHECK (channel_connection_id IS NOT NULL OR integration_installation_id IS NOT NULL) NOT VALID;
ALTER TABLE channel_inbound_events VALIDATE CONSTRAINT channel_inbound_events_owner_check;

-- +goose Down
DELETE FROM channel_inbound_events WHERE channel_connection_id IS NULL;
ALTER TABLE channel_inbound_events DROP CONSTRAINT channel_inbound_events_owner_check;
ALTER TABLE channel_inbound_events DROP COLUMN integration_installation_id;
ALTER TABLE channel_inbound_events ALTER COLUMN channel_connection_id SET NOT NULL;
