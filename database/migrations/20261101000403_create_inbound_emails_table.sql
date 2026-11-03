-- migrate: phase=expand
-- Every email that reached a workspace is kept here, ignored mail
-- included, so an admin can always see what arrived and why it became a
-- message or not. A provider redelivering the same mail hits the unique
-- provider message id and is stored once. The raw message lives on a
-- disk; its path goes when retention purges it. Recipients are only the
-- workspace's own addresses, never another workspace's.

-- +goose Up
CREATE TABLE inbound_emails (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    email_address_id uuid REFERENCES email_addresses ON DELETE SET NULL,
    channel_connection_id uuid REFERENCES channel_connections ON DELETE SET NULL,
    source text NOT NULL CHECK (source IN ('ses', 'test')),
    provider_message_id text,
    message_id_header text CHECK (char_length(message_id_header) <= 998),
    envelope_from text,
    envelope_to text[] NOT NULL,
    from_address citext,
    subject text,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    raw_disk text NOT NULL,
    raw_path text,
    raw_deleted_at timestamptz,
    status text NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'processed', 'ignored', 'failed', 'quarantined')),
    classification text CHECK (classification IN (
        'normal', 'auto_reply', 'bounce', 'complaint', 'list', 'bulk', 'noreply', 'system_notice', 'loop', 'spam',
        'forwarded', 'test'
    )),
    auth jsonb,
    provider_verdicts jsonb,
    conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    message_id uuid REFERENCES messages ON DELETE SET NULL,
    ignore_reason text,
    error text,
    attempts integer NOT NULL DEFAULT 0,
    received_at timestamptz NOT NULL,
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX inbound_emails_workspace_id_email_address_id_received_at_index
    ON inbound_emails (workspace_id, email_address_id, received_at DESC);
CREATE INDEX inbound_emails_workspace_id_status_received_at_index
    ON inbound_emails (workspace_id, status, received_at);
CREATE UNIQUE INDEX inbound_emails_workspace_id_source_provider_message_id_key
    ON inbound_emails (workspace_id, source, provider_message_id) WHERE provider_message_id IS NOT NULL;
CREATE INDEX inbound_emails_raw_deleted_at_index ON inbound_emails (raw_deleted_at) WHERE raw_path IS NOT NULL;

-- +goose Down
DROP TABLE inbound_emails;
