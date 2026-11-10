-- migrate: phase=expand
-- Every verified delivery from a chat app is kept as it came before
-- anything reads it, then processed by a job, so the provider gets its
-- answer at once and nothing is lost when processing fails. A provider
-- delivering the same event twice hits the unique key and is kept once.
-- Payloads carry what customers wrote, so they are pruned after days.

-- +goose Up
CREATE TABLE channel_inbound_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('slack', 'discord', 'telegram', 'whatsapp')),
    transport text NOT NULL CHECK (transport IN ('webhook', 'socket', 'polling', 'gateway', 'interaction')),
    dedupe_key text NOT NULL,
    serial_key text,
    kind text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'ignored', 'failed', 'held')),
    attempts integer NOT NULL DEFAULT 0,
    error text,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_inbound_events_workspace_id_provider_dedupe_key_key
    ON channel_inbound_events (workspace_id, provider, dedupe_key);
CREATE INDEX channel_inbound_events_workspace_id_status_received_at_index
    ON channel_inbound_events (workspace_id, status, received_at);
CREATE INDEX channel_inbound_events_received_at_index ON channel_inbound_events (received_at);

ALTER TABLE channel_inbound_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_inbound_events_tenant ON channel_inbound_events
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE channel_inbound_events;
