-- migrate: phase=expand
-- A tracker or incident tells every linked customer at once. The update
-- is written once; each linked conversation gets a delivery row that
-- says whether it was sent, held back and why (the customer wrote since,
-- a teammate already told them), or failed after its retries, so a
-- partial failure shows exactly who did not hear.
--
-- An incident also keeps a public log of short posts the messenger's
-- status card and the help center's status page show.

-- +goose Up
CREATE TABLE ticket_updates (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
    resolve boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sending', 'sent', 'partial', 'failed')),
    recipients integer NOT NULL DEFAULT 0,
    delivered integer NOT NULL DEFAULT 0,
    held integer NOT NULL DEFAULT 0,
    failed integer NOT NULL DEFAULT 0,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE INDEX ticket_updates_ticket_id_created_at_index ON ticket_updates (ticket_id, created_at DESC);
CREATE INDEX ticket_updates_author_member_id_index ON ticket_updates (author_member_id);

ALTER TABLE ticket_updates ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_updates_tenant ON ticket_updates USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_update_deliveries (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    update_id uuid NOT NULL REFERENCES ticket_updates ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'held', 'failed')),
    reason text CHECK (char_length(reason) <= 200),
    attempts integer NOT NULL DEFAULT 0,
    message_id uuid,
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ticket_update_deliveries_update_id_conversation_id_key UNIQUE (update_id, conversation_id)
);

CREATE INDEX ticket_update_deliveries_conversation_id_index ON ticket_update_deliveries (conversation_id);

ALTER TABLE ticket_update_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_update_deliveries_tenant ON ticket_update_deliveries
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_public_updates (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    state_id uuid REFERENCES ticket_states ON DELETE SET NULL,
    update_id uuid REFERENCES ticket_updates ON DELETE SET NULL,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    source text NOT NULL DEFAULT 'dimmi' CHECK (source ~ '^[a-z_]{1,40}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_public_updates_ticket_id_created_at_index ON ticket_public_updates (ticket_id, created_at DESC);
CREATE INDEX ticket_public_updates_state_id_index ON ticket_public_updates (state_id);
CREATE INDEX ticket_public_updates_update_id_index ON ticket_public_updates (update_id);
CREATE INDEX ticket_public_updates_author_member_id_index ON ticket_public_updates (author_member_id);

ALTER TABLE ticket_public_updates ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_public_updates_tenant ON ticket_public_updates
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ticket_public_updates;
DROP TABLE ticket_update_deliveries;
DROP TABLE ticket_updates;
