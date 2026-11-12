-- migrate: phase=expand
-- When a new conversation seems to report a problem an open tracker
-- already follows, the AI's match is kept as a suggestion the panel
-- shows: "Looks like CSV export crashes (54 others). Add to tracker?".
-- One row per conversation and tracker; adding it links the two, and a
-- workspace that turns on automatic linking links at once and keeps
-- the row as accepted.

-- +goose Up
CREATE TABLE tracker_suggestions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    rank integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'dismissed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tracker_suggestions_conversation_id_ticket_id_key ON tracker_suggestions (conversation_id, ticket_id);
CREATE INDEX tracker_suggestions_workspace_id_index ON tracker_suggestions (workspace_id);
CREATE INDEX tracker_suggestions_ticket_id_index ON tracker_suggestions (ticket_id);

ALTER TABLE tracker_suggestions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tracker_suggestions_tenant ON tracker_suggestions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE tracker_suggestions;
