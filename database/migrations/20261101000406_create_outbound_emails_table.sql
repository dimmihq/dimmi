-- migrate: phase=expand
-- One row per email sent for a workspace: replies, test mail, and later
-- the automated kinds. The row exists before the provider is called,
-- carrying the Message-ID it will send, so a retried send reuses the
-- same id and a sent row is never sent again. The provider's own id is
-- unique per transport, since delivery events find their row by it.

-- +goose Up
CREATE TABLE outbound_emails (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    message_id uuid REFERENCES messages ON DELETE SET NULL,
    conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    email_address_id uuid REFERENCES email_addresses ON DELETE SET NULL,
    kind text NOT NULL CHECK (kind IN (
        'reply', 'ai_reply', 'system', 'widget_offline', 'new_conversation', 'test', 'changelog', 'feedback_update',
        'portal_sign_in'
    )),
    from_address text NOT NULL,
    to_addresses text[] NOT NULL DEFAULT '{}',
    cc_addresses text[] NOT NULL DEFAULT '{}',
    bcc_addresses text[] NOT NULL DEFAULT '{}',
    subject text NOT NULL,
    message_id_header text NOT NULL,
    transport text NOT NULL CHECK (transport IN ('ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid', 'log')),
    provider_message_id text,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sent', 'delivered', 'bounced', 'complained', 'failed', 'suppressed')),
    recipient_statuses jsonb NOT NULL DEFAULT '{}',
    size_bytes bigint,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    metadata jsonb NOT NULL DEFAULT '{}',
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX outbound_emails_transport_provider_message_id_key ON outbound_emails (transport, provider_message_id)
    WHERE provider_message_id IS NOT NULL;
CREATE UNIQUE INDEX outbound_emails_workspace_id_idempotency_key_key
    ON outbound_emails (workspace_id, (metadata ->> 'idempotency_key'))
    WHERE metadata ->> 'idempotency_key' IS NOT NULL;
CREATE INDEX outbound_emails_workspace_id_message_id_index ON outbound_emails (workspace_id, message_id);
CREATE INDEX outbound_emails_workspace_id_message_id_header_index
    ON outbound_emails (workspace_id, message_id_header);

-- +goose Down
DROP TABLE outbound_emails;
