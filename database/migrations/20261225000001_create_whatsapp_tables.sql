-- migrate: phase=expand
-- WhatsApp lets a business write freely only within a day of the
-- customer's last message; after that only templates Meta approved go
-- out. Each customer's window on each connection is kept here, with the
-- newest message they sent, which read receipts and typing point at.
-- A connection's approved templates are copied from Meta, so the
-- composer lists them without asking Meta every time it opens.

-- +goose Up
CREATE TABLE channel_contact_states (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    last_inbound_at timestamptz,
    window_expires_at timestamptz,
    last_inbound_message_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_contact_states_channel_connection_id_contact_id_key
    ON channel_contact_states (channel_connection_id, contact_id);
CREATE INDEX channel_contact_states_workspace_id_contact_id_index ON channel_contact_states (workspace_id, contact_id);

ALTER TABLE channel_contact_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_contact_states_tenant ON channel_contact_states
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE whatsapp_templates (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    external_id text NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 512),
    language text NOT NULL CHECK (char_length(language) BETWEEN 2 AND 15),
    category text NOT NULL CHECK (category IN ('marketing', 'utility', 'authentication')),
    status text NOT NULL CHECK (char_length(status) BETWEEN 1 AND 40),
    components jsonb NOT NULL DEFAULT '[]'::jsonb,
    parameter_count integer NOT NULL DEFAULT 0 CHECK (parameter_count >= 0),
    sendable boolean NOT NULL DEFAULT false,
    rejected_reason text,
    bindings jsonb NOT NULL DEFAULT '{}'::jsonb,
    synced_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX whatsapp_templates_channel_connection_id_name_language_key
    ON whatsapp_templates (channel_connection_id, name, language);
CREATE INDEX whatsapp_templates_workspace_id_channel_connection_id_index
    ON whatsapp_templates (workspace_id, channel_connection_id);

ALTER TABLE whatsapp_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY whatsapp_templates_tenant ON whatsapp_templates
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE whatsapp_templates;
DROP TABLE channel_contact_states;
