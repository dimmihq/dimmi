-- migrate: phase=expand
-- A spike of new conversations about the same thing becomes a
-- suggestion to declare an incident. The detector groups the last
-- minutes of conversations by meaning; a group above the workspace's
-- threshold is stored once under its signature and grows while it
-- lasts. Accepting it declares the incident with the group linked;
-- dismissing it mutes the signature for an hour. The threshold lives
-- on the workspace with the window it counts over.

-- +goose Up
CREATE TABLE incident_suggestions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    signature text NOT NULL CHECK (char_length(signature) BETWEEN 1 AND 200),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    summary text NOT NULL DEFAULT '' CHECK (char_length(summary) <= 2000),
    area text CHECK (char_length(area) <= 80),
    conversation_ids uuid[] NOT NULL DEFAULT '{}',
    conversation_count integer NOT NULL DEFAULT 0,
    window_minutes integer NOT NULL DEFAULT 15,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'dismissed')),
    muted_until timestamptz,
    ticket_id uuid REFERENCES tickets ON DELETE SET NULL,
    decided_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX incident_suggestions_workspace_id_status_index ON incident_suggestions (workspace_id, status, updated_at DESC);
CREATE INDEX incident_suggestions_workspace_id_signature_index ON incident_suggestions (workspace_id, signature);
CREATE INDEX incident_suggestions_ticket_id_index ON incident_suggestions (ticket_id);
CREATE INDEX incident_suggestions_decided_by_member_id_index ON incident_suggestions (decided_by_member_id);

ALTER TABLE incident_suggestions ENABLE ROW LEVEL SECURITY;
CREATE POLICY incident_suggestions_tenant ON incident_suggestions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE workspaces ADD COLUMN incident_spike_threshold integer NOT NULL DEFAULT 8
    CHECK (incident_spike_threshold BETWEEN 2 AND 1000);
ALTER TABLE workspaces ADD COLUMN ticket_settings jsonb NOT NULL DEFAULT '{}'
    CHECK (jsonb_typeof(ticket_settings) = 'object');

-- +goose StatementBegin
CREATE FUNCTION recent_conversation_workspaces(since timestamptz)
    RETURNS TABLE (workspace_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT c.workspace_id FROM conversations c
        WHERE c.created_at >= since AND c.deleted_at IS NULL
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION recent_conversation_workspaces(timestamptz);
ALTER TABLE workspaces DROP COLUMN ticket_settings;
ALTER TABLE workspaces DROP COLUMN incident_spike_threshold;
DROP TABLE incident_suggestions;
